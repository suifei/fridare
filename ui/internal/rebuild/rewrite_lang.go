package rebuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Structure-aware (per-language lexical/AST) rewriters form the mechanical safety
// layer for source mods: only rewrite frida markers in string/comment tokens,
// never unquoted identifiers (e.g. frida_agent_main as a C function name).

// LangRewriteResult is stats from one file rewrite.
type LangRewriteResult struct {
	Lang          string
	Replacements  int
	Engine        string // "c-lex" | "python-lex" | "python-ast" | "generic-lex"
	Content       string
	ParseOKAfter  bool // set when post-check ran (Python)
}

// StructureAwareRewrite dispatches by file extension to a language engine.
// Unknown text types fall back to generic C-like lexical string/comment rewrite.
// Protocol/API string rewrite is off (stock-client safe).
func StructureAwareRewrite(path, content, magic string) LangRewriteResult {
	return StructureAwareRewriteOpts(path, content, magic, StringRewriteOpts{})
}

// StructureAwareRewriteOpts is StructureAwareRewrite with optional protocol/API string pairs.
func StructureAwareRewriteOpts(path, content, magic string, opts StringRewriteOpts) LangRewriteResult {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))
	switch {
	case ext == ".c" || ext == ".h" || ext == ".cc" || ext == ".cpp" || ext == ".cxx" ||
		ext == ".hh" || ext == ".hpp" || ext == ".m" || ext == ".mm":
		out, n := RewriteCSourceOpts(content, magic, opts)
		return LangRewriteResult{Lang: "c", Replacements: n, Engine: "c-lex", Content: out, ParseOKAfter: true}
	case ext == ".py" || ext == ".pyw":
		return rewritePythonPreferAST(content, magic, opts)
	case ext == ".vala" || ext == ".vapi":
		out, n := RewriteCSourceOpts(content, magic, opts)
		return LangRewriteResult{Lang: "vala", Replacements: n, Engine: "c-lex", Content: out, ParseOKAfter: true}
	case ext == ".js" || ext == ".ts" || ext == ".json" || ext == ".meson" || base == "meson.build":
		out, n := RewriteCSourceOpts(content, magic, opts)
		return LangRewriteResult{Lang: "text", Replacements: n, Engine: "generic-lex", Content: out, ParseOKAfter: true}
	default:
		out, n := RewriteCSourceOpts(content, magic, opts)
		return LangRewriteResult{Lang: "generic", Replacements: n, Engine: "generic-lex", Content: out, ParseOKAfter: true}
	}
}

// RewriteCSource token-classifies C/C++/ObjC-like source: rewrites markers only
// inside // and /* */ comments and "..." / '...' literals. Unquoted identifiers
// such as frida_agent_main are preserved.
func RewriteCSource(content, magic string) (string, int) {
	return RewriteCSourceOpts(content, magic, StringRewriteOpts{})
}

// RewriteCSourceOpts is RewriteCSource with optional protocol/API string rewrites.
func RewriteCSourceOpts(content, magic string, opts StringRewriteOpts) (string, int) {
	return rewriteCLikeLexOpts(content, magic, false, opts)
}

// RewritePythonSource rewrites frida markers only inside Python string tokens
// (# comments and '...', "...", triple quotes). Bare identifiers are preserved.
// This is a pure-Go lexical engine (tokenize-aligned), not a full CPython AST.
func RewritePythonSource(content, magic string) (string, int) {
	return rewritePythonLexOpts(content, magic, StringRewriteOpts{})
}

func rewritePythonPreferAST(content, magic string, opts StringRewriteOpts) LangRewriteResult {
	// Prefer CPython tokenize (stdlib) for format-preserving STRING-token rewrite.
	// Do NOT use ast.unparse — it drops shebangs/comments and reformats the file.
	if out, n, ok := tryPythonTokenizeRewrite(content, magic, opts); ok {
		parseOK := pythonAstParseOK(out)
		return LangRewriteResult{Lang: "python", Replacements: n, Engine: "python-tokenize", Content: out, ParseOKAfter: parseOK}
	}
	out, n := rewritePythonLexOpts(content, magic, opts)
	// Pure-Go lex always format-preserving; ParseOKAfter checked when python available.
	parseOK := true
	if _, err := exec.LookPath("python"); err == nil {
		parseOK = pythonAstParseOK(out)
	} else if _, err := exec.LookPath("python3"); err == nil {
		parseOK = pythonAstParseOK(out)
	}
	return LangRewriteResult{Lang: "python", Replacements: n, Engine: "python-lex", Content: out, ParseOKAfter: parseOK}
}

// tryPythonTokenizeRewrite uses stdlib tokenize to rewrite only STRING tokens,
// preserving shebang, comments, and layout. Returns ok=false if python missing/fails.
func tryPythonTokenizeRewrite(content, magic string, opts StringRewriteOpts) (string, int, bool) {
	py, err := exec.LookPath("python")
	if err != nil {
		py, err = exec.LookPath("python3")
		if err != nil {
			return "", 0, false
		}
	}
	// Build pairs in Go (includes optional protocol/API) and pass as JSON lines to Python.
	pairs := quotedFridaReplacementsOpts(magic, opts)
	var pairLines strings.Builder
	for _, p := range pairs {
		// tab-separated, escape newlines not present in our pairs
		pairLines.WriteString(p[0])
		pairLines.WriteByte('\t')
		pairLines.WriteString(p[1])
		pairLines.WriteByte('\n')
	}
	script := `
import io, sys, tokenize
src = sys.stdin.read()
pairs = []
for line in sys.argv[1].split("\n"):
    line = line.strip("\n")
    if not line or "\t" not in line:
        continue
    a, b = line.split("\t", 1)
    pairs.append((a, b))
count = [0]

def repl_str_token(tokval: str) -> str:
    out = tokval
    for a, b in pairs:
        if a == "frida" or a == "Frida" or a == "FRIDA":
            continue
        if a in out:
            n = out.count(a)
            out = out.replace(a, b)
            count[0] += n
    return out

try:
    tokens = list(tokenize.generate_tokens(io.StringIO(src).readline))
except tokenize.TokenError:
    sys.exit(2)
new_tokens = []
for tok in tokens:
    ttype, tstr, start, end, line = tok
    if ttype == tokenize.STRING:
        tstr = repl_str_token(tstr)
    new_tokens.append(tokenize.TokenInfo(ttype, tstr, start, end, line))
try:
    out = tokenize.untokenize(new_tokens)
except Exception:
    sys.exit(3)
if isinstance(out, bytes):
    out = out.decode("utf-8", "replace")
sys.stdout.write(out)
sys.stderr.write("COUNT=%d\n" % count[0])
`
	cmd := exec.Command(py, "-c", script, pairLines.String())
	cmd.Stdin = strings.NewReader(content)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", 0, false
	}
	out := stdout.String()
	n := 0
	for _, line := range strings.Split(stderr.String(), "\n") {
		if strings.HasPrefix(line, "COUNT=") {
			fmt.Sscanf(line, "COUNT=%d", &n)
		}
	}
	if out == "" && content != "" {
		return "", 0, false
	}
	return out, n, true
}

func pythonAstParseOK(src string) bool {
	py, err := exec.LookPath("python")
	if err != nil {
		py, err = exec.LookPath("python3")
		if err != nil {
			return true // cannot check
		}
	}
	cmd := exec.Command(py, "-c", "import ast,sys; ast.parse(sys.stdin.read())")
	cmd.Stdin = strings.NewReader(src)
	return cmd.Run() == nil
}

// ApplyStructureAwareStrip walks a source tree and applies per-language structure-aware rewrites.
func ApplyStructureAwareStrip(sourceDir, magic string) (filesTouched, replacements int, err error) {
	return ApplyStructureAwareStripOpts(sourceDir, magic, StringRewriteOpts{})
}

// ApplyStructureAwareStripOpts is ApplyStructureAwareStrip with ProtocolAPI / string opts.
func ApplyStructureAwareStripOpts(sourceDir, magic string, opts StringRewriteOpts) (filesTouched, replacements int, err error) {
	if sourceDir == "" {
		return 0, 0, fmt.Errorf("sourceDir empty")
	}
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Size() == 0 || info.Size() > 4*1024*1024 {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, "fridare-") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !textFileExts[ext] && base != "meson.build" && base != "makefile" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if len(data) > 0 {
			headN := 512
			if len(data) < headN {
				headN = len(data)
			}
			if strings.IndexByte(string(data[:headN]), 0) >= 0 {
				return nil
			}
		}
		res := StructureAwareRewriteOpts(path, string(data), magic, opts)
		if res.Replacements == 0 || res.Content == string(data) {
			return nil
		}
		if err := os.WriteFile(path, []byte(res.Content), info.Mode()); err != nil {
			return err
		}
		filesTouched++
		replacements += res.Replacements
		return nil
	})
	return filesTouched, replacements, err
}
