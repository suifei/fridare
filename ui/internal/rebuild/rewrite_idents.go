package rebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// LayerIdentifiers is auto under profile=deep|explore|abi: rename functions/classes/namespaces.
const LayerIdentifiers StripLayerID = "L13_identifiers_namespaces"

// MapFridaIdentifier maps a single identifier token from Frida naming to magic.
// Returns (mapped, true) if the identifier should change.
// Does not touch protocol dotted names (those are not single identifiers).
func MapFridaIdentifier(id, magic string) (string, bool) {
	if id == "" || magic == "" || magic == "frida" {
		return id, false
	}
	// Keep build-system / releng module names stable (file is still frida_version.py).
	switch id {
	case "frida_version", "FridaVersion", "FRIDA_VERSION",
		"frida_gum", "frida_core", "frida_tools", "frida_python",
		"frida_node", "frida_clr", "frida_qml", "frida_swift", "frida_go":
		return id, false
	}
	pas := pascalMagic(magic)
	up := strings.ToUpper(magic)

	// exact
	switch id {
	case "frida":
		return magic, true
	case "Frida":
		return pas, true
	case "FRIDA":
		return up, true
	}

	// frida_* snake (functions, types in C)
	if strings.HasPrefix(id, "frida_") {
		return magic + id[len("frida"):], true // frida_agent → magic_agent (keep underscore: magic + "_agent")
	}
	// careful: frida_agent = magic + id[5:] where id[5:] is _agent → magic_agent ✓
	// id = "frida_agent_main", id[len("frida"):] = "_agent_main" → magic + "_agent_main" ✓

	// FRIDA_* macros
	if strings.HasPrefix(id, "FRIDA_") {
		return up + id[len("FRIDA"):], true
	}

	// FridaPascal / FridaAgent / FridaScriptEngine
	if strings.HasPrefix(id, "Frida") && len(id) > 5 {
		rest := id[5:]
		// next rune should be uppercase or digit for Pascal continuation
		r, _ := utf8.DecodeRuneInString(rest)
		if unicode.IsUpper(r) || unicode.IsDigit(r) {
			return pas + rest, true
		}
	}

	// _frida_ private symbols
	if strings.HasPrefix(id, "_frida_") {
		return "_" + magic + id[len("_frida"):], true
	}
	if strings.HasPrefix(id, "_Frida") && len(id) > 6 {
		return "_" + pas + id[6:], true
	}

	return id, false
}

// RewriteCIdentifiers renames Frida-related identifiers and leaves strings/comments
// to the string rewriter. Order: typically apply string rewrite first, then identifiers.
func RewriteCIdentifiers(content, magic string) (string, int) {
	if content == "" || magic == "" || magic == "frida" {
		return content, 0
	}
	n := 0
	var out strings.Builder
	out.Grow(len(content))
	i := 0
	for i < len(content) {
		// skip strings/comments as opaque (already handled by string layer; don't rename inside)
		if content[i] == '/' && i+1 < len(content) && content[i+1] == '/' {
			j := i + 2
			for j < len(content) && content[j] != '\n' {
				j++
			}
			out.WriteString(content[i:j])
			i = j
			continue
		}
		if content[i] == '/' && i+1 < len(content) && content[i+1] == '*' {
			j := i + 2
			for j+1 < len(content) && !(content[j] == '*' && content[j+1] == '/') {
				j++
			}
			if j+1 < len(content) {
				j += 2
			}
			out.WriteString(content[i:j])
			i = j
			continue
		}
		if content[i] == '"' {
			j := i + 1
			for j < len(content) {
				if content[j] == '\\' && j+1 < len(content) {
					j += 2
					continue
				}
				if content[j] == '"' {
					j++
					break
				}
				j++
			}
			out.WriteString(content[i:j])
			i = j
			continue
		}
		if content[i] == '\'' {
			j := i + 1
			for j < len(content) {
				if content[j] == '\\' && j+1 < len(content) {
					j += 2
					continue
				}
				if content[j] == '\'' {
					j++
					break
				}
				j++
			}
			out.WriteString(content[i:j])
			i = j
			continue
		}
		// identifier
		if isIdentStart(content[i]) {
			j := i + 1
			for j < len(content) && isIdentCont(content[j]) {
				j++
			}
			id := content[i:j]
			if mapped, ok := MapFridaIdentifier(id, magic); ok {
				out.WriteString(mapped)
				n++
			} else {
				out.WriteString(id)
			}
			i = j
			continue
		}
		out.WriteByte(content[i])
		i++
	}
	return out.String(), n
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isIdentCont(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}

// RewritePythonIdentifiers renames Frida-related Name tokens outside strings/comments.
func RewritePythonIdentifiers(content, magic string) (string, int) {
	// Same identifier rules as C for pure-Go path (Python idents are similar).
	return RewriteCIdentifiers(content, magic)
}

// RewriteValaNamespacesAndIdents renames identifiers and dotted Frida type prefixes.
// Vala: namespace Frida / Frida.Agent / types Frida.Foo become magic Pascal forms.
func RewriteValaNamespacesAndIdents(content, magic string) (string, int) {
	// First pass: identifier tokens (covers namespace Frida, class names, methods).
	out, n := RewriteCIdentifiers(content, magic)
	return out, n
}

// StructureAwareRewriteAll applies string-literal rewrite then identifier/namespace rename
// when renameIdents is true. ProtocolAPI strings follow opts (typically true for deep).
func StructureAwareRewriteAll(path, content, magic string, renameIdents bool) LangRewriteResult {
	return StructureAwareRewriteAllOpts(path, content, magic, renameIdents, StringRewriteOpts{ProtocolAPI: renameIdents})
}

// StructureAwareRewriteAllOpts is StructureAwareRewriteAll with explicit string opts.
func StructureAwareRewriteAllOpts(path, content, magic string, renameIdents bool, opts StringRewriteOpts) LangRewriteResult {
	res := StructureAwareRewriteOpts(path, content, magic, opts)
	if !renameIdents || magic == "" || magic == "frida" {
		return res
	}
	ext := strings.ToLower(filepath.Ext(path))
	var n2 int
	switch {
	case ext == ".py" || ext == ".pyw":
		res.Content, n2 = RewritePythonIdentifiers(res.Content, magic)
		res.Engine = res.Engine + "+idents"
	case ext == ".vala" || ext == ".vapi":
		res.Content, n2 = RewriteValaNamespacesAndIdents(res.Content, magic)
		res.Engine = "vala-idents"
	case ext == ".c" || ext == ".h" || ext == ".cc" || ext == ".cpp" || ext == ".cxx" ||
		ext == ".hh" || ext == ".hpp" || ext == ".m" || ext == ".mm":
		res.Content, n2 = RewriteCIdentifiers(res.Content, magic)
		res.Engine = res.Engine + "+idents"
	default:
		// meson.build etc.: only rename identifiers carefully (may hit dependency names)
		res.Content, n2 = RewriteCIdentifiers(res.Content, magic)
		res.Engine = "generic-idents"
	}
	res.Replacements += n2
	if ext == ".py" || ext == ".pyw" {
		res.ParseOKAfter = pythonAstParseOK(res.Content)
	}
	return res
}

// skipIdentifierRenamePath reports paths where token rename would break the
// Frida build system (releng modules, meson vendored tree, git metadata).
func skipIdentifierRenamePath(path string) bool {
	norm := filepath.ToSlash(path)
	low := strings.ToLower(norm)
	if strings.Contains(low, "/.git/") {
		return true
	}
	// releng: frida_version.py module name must stay importable as releng.frida_version
	if strings.Contains(low, "/releng/") {
		return true
	}
	// vendored meson inside releng already covered; also skip pure build-system blobs
	if strings.Contains(low, "/meson/mesonbuild/") {
		return true
	}
	base := strings.ToLower(filepath.Base(path))
	if base == "detect-version.py" || base == "frida_version.py" {
		return true
	}
	return false
}

// ApplyIdentifierRenameStrip walks the tree renaming functions/classes/namespaces
// (Frida*/frida_*/FRIDA_*) after string-layer rewrite.
func ApplyIdentifierRenameStrip(sourceDir, magic string) (filesTouched, replacements int, err error) {
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
		if skipIdentifierRenamePath(path) {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, "fridare-") {
			return nil
		}
		// Never rename inside subproject wrap names by path segment? We only touch file content.
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
		res := StructureAwareRewriteAll(path, string(data), magic, true)
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

// ProfileRenamesIdentifiers reports whether profile requests full symbol/namespace rename.
// deep does NOT rename identifiers (protocol/API strings only — keeps meson/C ABI buildable).
// abi | full | explore enable token-level Frida* / frida_* renames (higher break risk).
func ProfileRenamesIdentifiers(profile string) bool {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "explore", "abi", "full":
		return true
	default:
		return false
	}
}
