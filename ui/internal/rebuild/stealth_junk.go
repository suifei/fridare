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

func normalizeJunkSeed(seed string) string {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		seed = "0"
	}
	if len(seed) > 8 {
		seed = seed[:8]
	}
	return seed
}

func junkSeedFlag(seed string) string {
	return fmt.Sprintf(" -DFRIDARE_JUNK_SEED=0x%s", normalizeJunkSeed(seed))
}

// isInjectionCompileBuildLine reports whether a ninja "build" edge compiles one
// of the injection TUs (the $in source), not a later link that merely lists *.c.o.
func isInjectionCompileBuildLine(line string, needles []string) bool {
	s := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(s, "build ") {
		return false
	}
	if !strings.Contains(s, "c_COMPILER") {
		return false
	}
	for _, n := range needles {
		token := "/" + n
		idx := 0
		for {
			i := strings.Index(s[idx:], token)
			if i < 0 {
				break
			}
			i += idx
			end := i + len(token)
			var next byte
			if end < len(s) {
				next = s[end]
			}
			// Source input: /gumprocess.c followed by space, tab, |, CR, LF, or EOF.
			// Reject /gumprocess.c.o (linker inputs and DEPFILE paths).
			if next == 0 || next == ' ' || next == '\t' || next == '|' || next == '\n' || next == '\r' {
				return true
			}
			idx = end
		}
	}
	return false
}

// PatchNinjaStealthFlags appends -DFRIDARE_JUNK_SEED only to the ARGS= of
// injection-TU c_COMPILER edges. Never mutates "build " / DEPFILE lines —
// appending there makes ninja treat the flag as an implicit input
// (error: '-DFRIDARE_JUNK_SEED=…', needed by '…gumprocess.c.o').
func PatchNinjaStealthFlags(content, seed string) (string, int) {
	flag := junkSeedFlag(seed)
	needles := InjectionCompileBasenames()
	changed := 0
	pending := false
	var b strings.Builder
	for _, line := range strings.SplitAfter(content, "\n") {
		body := strings.TrimRight(line, "\r\n")
		nl := line[len(body):]
		trimmed := strings.TrimLeft(body, " \t")
		if strings.HasPrefix(trimmed, "build ") {
			pending = isInjectionCompileBuildLine(trimmed, needles)
			b.WriteString(line)
			continue
		}
		if pending && strings.HasPrefix(trimmed, "ARGS") && strings.Contains(trimmed, "=") &&
			!strings.Contains(body, "FRIDARE_JUNK_SEED") {
			b.WriteString(body)
			b.WriteString(flag)
			b.WriteString(nl)
			changed++
			pending = false
			continue
		}
		b.WriteString(line)
	}
	return b.String(), changed
}

// PatchCompileCommandsStealthFlags inserts the seed flag before -c on
// compile_commands.json "command" lines for injection TUs only.
func PatchCompileCommandsStealthFlags(content, seed string) (string, int) {
	flag := junkSeedFlag(seed) + " "
	needles := InjectionCompileBasenames()
	changed := 0
	var b strings.Builder
	for _, line := range strings.SplitAfter(content, "\n") {
		if !strings.Contains(line, `"command"`) || !strings.Contains(line, " -c ") ||
			strings.Contains(line, "FRIDARE_JUNK_SEED") {
			b.WriteString(line)
			continue
		}
		hit := false
		for _, n := range needles {
			if strings.Contains(line, "/"+n) || strings.Contains(line, " "+n) {
				hit = true
				break
			}
		}
		if !hit {
			b.WriteString(line)
			continue
		}
		b.WriteString(strings.Replace(line, " -c ", flag+"-c ", 1))
		changed++
	}
	return b.String(), changed
}

// PerTUStealthFlagsShell patches this build dir's ninja ARGS (injection TUs only).
// Never exports process-wide CFLAGS/CPPFLAGS. Must not append flags to ninja
// "build" edges — those tokens become implicit dependencies.
func PerTUStealthFlagsShell(seed string) string {
	seed = normalizeJunkSeed(seed)
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
flag = " -DFRIDARE_JUNK_SEED=0x%s"
changed = 0

def is_injection_compile(line):
    s = line.lstrip()
    if not s.startswith("build ") or "c_COMPILER" not in s:
        return False
    for n in needles:
        token = "/" + n
        idx = 0
        while True:
            i = s.find(token, idx)
            if i < 0:
                break
            end = i + len(token)
            nxt = s[end:end+1]
            if nxt in ("", " ", "\t", "|", "\n", "\r"):
                return True
            idx = end
    return False

def patch_ninja(text):
    out = []
    pending = False
    nchg = 0
    for line in text.splitlines(True):
        stripped = line.lstrip()
        if stripped.startswith("build "):
            pending = is_injection_compile(line)
            out.append(line)
            continue
        if pending and stripped.startswith("ARGS") and "=" in stripped and "FRIDARE_JUNK_SEED" not in line:
            body = line.rstrip("\r\n")
            nl = line[len(body):]
            out.append(body + flag + nl)
            nchg += 1
            pending = False
            continue
        out.append(line)
    return "".join(out), nchg

def patch_compile_commands(text):
    out = []
    nchg = 0
    for line in text.splitlines(True):
        if '"command"' in line and " -c " in line and "FRIDARE_JUNK_SEED" not in line:
            if any(("/" + n in line or " " + n in line) for n in needles):
                line = line.replace(" -c ", flag + " -c ", 1)
                nchg += 1
        out.append(line)
    return "".join(out), nchg

# Only this build dir's ninja (rglob over all build-* trees is multi-minute).
for p in list(pathlib.Path(".").glob("build.ninja")) + list(pathlib.Path(".").glob("compile_commands.json")):
    s = p.read_text(encoding="utf-8", errors="replace")
    if p.name == "compile_commands.json":
        new, n = patch_compile_commands(s)
    else:
        new, n = patch_ninja(s)
    if n:
        p.write_text(new, encoding="utf-8")
        changed += 1
        print("[fridare] per-TU stealth flags:", p, "args+", n)
print("[fridare] per-TU stealth files patched:", changed)
PY`, lit.String(), seed)
}
