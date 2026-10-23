package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func TestCatalogCoversUserStrings(t *testing.T) {
	root := moduleRoot(t)
	keys := map[string]struct{}{}
	fset := token.NewFileSet()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		af, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "T" {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name != "i18n" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			val, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatalf("unquote %s: %v", path, err)
			}
			keys[val] = struct{}{}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) < 1000 {
		t.Fatalf("expected the GUI/CLI string sweep, got %d literals", len(keys))
	}

	// These are passed as identifiers, so the AST sweep does not see the text.
	for _, key := range identifierMessageKeys(t, root) {
		keys[key] = struct{}{}
	}

	var missing, hanLeft, verbMismatch int
	for key := range keys {
		if !containsHan(key) {
			continue
		}
		en, ok := catalogEN[key]
		if !ok || strings.TrimSpace(en) == "" {
			missing++
			if missing <= 8 {
				t.Errorf("missing English for %q", truncate(key, 80))
			}
			continue
		}
		if containsHan(en) {
			hanLeft++
			if hanLeft <= 8 {
				t.Errorf("English still has Chinese: %q -> %q", truncate(key, 60), truncate(en, 60))
			}
		}
		if strings.Join(fmtVerbs(key), ",") != strings.Join(fmtVerbs(en), ",") {
			verbMismatch++
			if verbMismatch <= 8 {
				t.Errorf("fmt verbs differ\n zh %q %v\n en %q %v", truncate(key, 60), fmtVerbs(key), truncate(en, 60), fmtVerbs(en))
			}
		}
	}
	if missing+hanLeft+verbMismatch > 0 {
		t.Fatalf("catalog problems: missing=%d han=%d verbs=%d", missing, hanLeft, verbMismatch)
	}

	SetLanguage(LangEn)
	t.Cleanup(func() { SetLanguage(LangZh) })
	if got := T("取消"); got != "Cancel" {
		t.Fatalf("取消 -> %q", got)
	}
	if got := T("设置"); got != "Settings" {
		t.Fatalf("设置 -> %q", got)
	}
	if got := Tf("端口: %d", 27042); got != "Port: 27042" {
		t.Fatalf("Tf port: %q", got)
	}
}

func identifierMessageKeys(t *testing.T, root string) []string {
	t.Helper()
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	gui := regexp.MustCompile(`GUIDescription\s*=\s*"([^"]+)"`).FindStringSubmatch(read("internal/appmeta/meta.go"))
	mirror := regexp.MustCompile("(?s)const DockerMirrorHelp = `([^`]*)`").FindStringSubmatch(read("internal/rebuild/docker_mirror.go"))
	endpoint := regexp.MustCompile("(?s)const RecommendedEndpointHelp = `([^`]*)`").FindStringSubmatch(read("internal/rebuild/constants.go"))
	if gui == nil || mirror == nil || endpoint == nil {
		t.Fatal("could not read identifier message constants")
	}
	return []string{gui[1], mirror[1], endpoint[1]}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// fmtVerbs returns Go fmt verbs in order, including %%. A bare % that is not a
// verb is kept as "%" so translations cannot drop or invent conversions.
func fmtVerbs(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '%' {
			out = append(out, "%%")
			i++
			continue
		}
		j := i + 1
		for j < len(s) && strings.ContainsRune("#+- 0", rune(s[j])) {
			j++
		}
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == '*' {
			j++
		}
		if j < len(s) && s[j] == '.' {
			j++
			if j < len(s) && s[j] == '*' {
				j++
			} else {
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
			}
		}
		if j < len(s) && strings.ContainsRune("vTbcdoOqxXUeEfFgGspw", rune(s[j])) {
			out = append(out, s[i:j+1])
			i = j
			continue
		}
		out = append(out, "%")
	}
	return out
}
