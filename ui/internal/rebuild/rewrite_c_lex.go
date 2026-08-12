package rebuild

import "strings"

// rewriteCLikeLex is a structure-aware lexical rewriter for C-like languages.
// rawTriple: reserved for future (unused for C).
func rewriteCLikeLex(content, magic string, rawTriple bool) (string, int) {
	return rewriteCLikeLexOpts(content, magic, rawTriple, StringRewriteOpts{})
}

func rewriteCLikeLexOpts(content, magic string, rawTriple bool, opts StringRewriteOpts) (string, int) {
	_ = rawTriple
	if content == "" || magic == "" || magic == "frida" {
		return content, 0
	}
	pairs := quotedFridaReplacementsOpts(magic, opts)
	n := 0
	var out strings.Builder
	out.Grow(len(content))
	i := 0
	for i < len(content) {
		c := content[i]
		// line comment //
		if c == '/' && i+1 < len(content) && content[i+1] == '/' {
			j := i + 2
			for j < len(content) && content[j] != '\n' {
				j++
			}
			seg2, c2 := replacePairsInSegment(content[i:j], pairs)
			n += c2
			out.WriteString(seg2)
			i = j
			continue
		}
		// block comment /* */
		if c == '/' && i+1 < len(content) && content[i+1] == '*' {
			j := i + 2
			for j+1 < len(content) && !(content[j] == '*' && content[j+1] == '/') {
				j++
			}
			if j+1 < len(content) {
				j += 2
			}
			seg2, c2 := replacePairsInSegment(content[i:j], pairs)
			n += c2
			out.WriteString(seg2)
			i = j
			continue
		}
		// preprocessor keep as code (identifiers safe); still allow strings after #
		// double-quoted string
		if c == '"' {
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
			seg2, c2 := replacePairsInSegment(content[i:j], pairs)
			n += c2
			out.WriteString(seg2)
			i = j
			continue
		}
		// character literal
		if c == '\'' {
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
			seg2, c2 := replacePairsInSegment(content[i:j], pairs)
			n += c2
			out.WriteString(seg2)
			i = j
			continue
		}
		// unquoted code: copy as-is (identifiers untouched)
		out.WriteByte(c)
		i++
	}
	return out.String(), n
}
