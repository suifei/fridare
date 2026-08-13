package rebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fridareJunkMarker = "/* FRIDARE_JUNK "

// SeededJunkC returns a deterministic C snippet (dead code + fake branches)
// whose literals depend on seed. Same seed ⇒ same bytes; different seed ⇒ different.
func SeededJunkC(seed string) string {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		seed = "0"
	}
	// fold seed into two 32-bit constants
	var a, b uint32
	for i, c := range seed {
		if i%2 == 0 {
			a = a*33 + uint32(c)
		} else {
			b = b*33 + uint32(c)
		}
	}
	if a == 0 {
		a = 0x9e3779b9
	}
	if b == 0 {
		b = 0x85ebca6b
	}
	tag := seed
	if len(tag) > 16 {
		tag = tag[:16]
	}
	id := sanitizeJunkIdent(tag)
	return fmt.Sprintf(`%sseed=%s */
#if 1
/* noinline + volatile: a constant call is otherwise folded; strip then drops .debug copies */
static unsigned fridare_junk_%s(unsigned x) __attribute__((used, noinline));
static unsigned fridare_junk_%s(unsigned x) {
  volatile unsigned k = 0x%08xu;
  volatile unsigned m = 0x%08xu;
  if ((x ^ k) == m) {
    k ^= 0x11111111u;
  } else if (x == k) {
    k += m;
  } else {
    k = (k << 1) | (x & 1u);
  }
  return k ^ x ^ m;
}
static const unsigned fridare_junk_%s_words[2] = { 0x%08xu, 0x%08xu };
static void fridare_junk_%s_keep(void) __attribute__((constructor, used, noinline));
static void fridare_junk_%s_keep(void) {
  volatile unsigned arg = 1u;
  volatile unsigned v = fridare_junk_%s(arg);
  v ^= fridare_junk_%s_words[0] ^ fridare_junk_%s_words[1];
  (void)v;
}
#endif
`, fridareJunkMarker, tag, id, id, a, b, id, a, b, id, id, id, id, id)
}

func sanitizeJunkIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "0"
	}
	return b.String()
}

// InjectSeededJunk appends or replaces the FRIDARE_JUNK block in a C translation unit.
func InjectSeededJunk(content, seed string) string {
	block := SeededJunkC(seed)
	if i := strings.Index(content, fridareJunkMarker); i >= 0 {
		// Replace only this block: first #endif after the marker (never LastIndex —
		// that would swallow later preprocessor closes if anything follows the junk).
		rest := content[i:]
		end := strings.Index(rest, "#endif")
		if end >= 0 {
			end += len("#endif")
			if end < len(rest) && rest[end] == '\n' {
				end++
			}
			return content[:i] + block + rest[end:]
		}
		return content[:i] + block
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + block
}

// ApplySeededJunkToInjectionTUs writes seed-stable junk into whitelist *.c files.
func ApplySeededJunkToInjectionTUs(sourceDir, seed string) (int, error) {
	if sourceDir == "" {
		return 0, fmt.Errorf("sourceDir empty")
	}
	n := 0
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".c" {
			return nil
		}
		rel, _ := filepath.Rel(sourceDir, path)
		if !IsInjectionABIPath(filepath.ToSlash(rel)) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		out := InjectSeededJunk(string(data), seed)
		if out == string(data) {
			return nil
		}
		if err := os.WriteFile(path, []byte(out), info.Mode()); err != nil {
			return err
		}
		n++
		return nil
	})
	return n, err
}

// InjectionCompileBasenames are .c files that may receive per-TU -DFRIDARE_JUNK_SEED.
func InjectionCompileBasenames() []string {
	return []string{
		"gumprocess-linux.c",
		"gumprocess-windows.c",
		"gumprocess-posix.c",
		"gumprocess.c",
		"agent-glue.c",
		"linjector-glue.c",
		"winjector-glue.c",
	}
}

// PerTUStealthFlagsShell patches build.ninja compile lines for injection TUs only.
// Never exports process-wide CFLAGS/CPPFLAGS.
func PerTUStealthFlagsShell(seed string) string {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		seed = "0"
	}
	if len(seed) > 8 {
		seed = seed[:8]
	}
	names := InjectionCompileBasenames()
	var lit strings.Builder
	for i, n := range names {
		if i > 0 {
			lit.WriteString(", ")
		}
		lit.WriteString(fmt.Sprintf("%q", n))
	}
	return fmt.Sprintf(`python3 - <<'PY'
import pathlib
needles = [%s]
flag = " -DFRIDARE_JUNK_SEED=0x%s "
changed = 0
# Only this build dir's ninja (rglob over all build-* trees is multi-minute).
for p in list(pathlib.Path(".").glob("build.ninja")) + list(pathlib.Path(".").glob("compile_commands.json")):
    s = p.read_text(encoding="utf-8", errors="replace")
    out_lines = []
    file_changed = False
    for line in s.splitlines(True):
        hit = any(n in line for n in needles)
        if hit and "FRIDARE_JUNK_SEED" not in line:
            if " -c " in line:
                line = line.replace(" -c ", flag + "-c ", 1)
            else:
                line = line.rstrip("\n") + flag + "\n"
            file_changed = True
        out_lines.append(line)
    if file_changed:
        p.write_text("".join(out_lines), encoding="utf-8")
        changed += 1
        print("[fridare] per-TU stealth flags:", p)
print("[fridare] per-TU stealth files patched:", changed)
PY`, lit.String(), seed)
}
