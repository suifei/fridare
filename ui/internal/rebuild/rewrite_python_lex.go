package rebuild

import "strings"

// rewritePythonLex rewrites markers only in Python comments and string literals
// (single, double, and triple-quoted). Does not rewrite bare identifiers.
func rewritePythonLex(content, magic string) (string, int) {
	return rewritePythonLexOpts(content, magic, StringRewriteOpts{})
}

func rewritePythonLexOpts(content, magic string, opts StringRewriteOpts) (string, int) {
	if content == "" || magic == "" || magic == "frida" {
		return content, 0
	}
	pairs := quotedFridaReplacementsOpts(magic, opts)
	n := 0
	var out strings.Builder
	out.Grow(len(content))
	i := 0
	for i < len(content) {
		// # line comment
		if content[i] == '#' {
			j := i + 1
			for j < len(content) && content[j] != '\n' {
				j++
			}
			seg2, c2 := replacePairsInSegment(content[i:j], pairs)
			n += c2
			out.WriteString(seg2)
			i = j
			continue
		}
		// triple double """
		if i+2 < len(content) && content[i] == '"' && content[i+1] == '"' && content[i+2] == '"' {
			j := i + 3
			for j+2 < len(content) && !(content[j] == '"' && content[j+1] == '"' && content[j+2] == '"') {
				if content[j] == '\\' && j+1 < len(content) {
					j += 2
					continue
				}
				j++
			}
			if j+2 < len(content) {
				j += 3
			}
			seg2, c2 := replacePairsInSegment(content[i:j], pairs)
			n += c2
			out.WriteString(seg2)
			i = j
			continue
		}
		// triple single '''
		if i+2 < len(content) && content[i] == '\'' && content[i+1] == '\'' && content[i+2] == '\'' {
			j := i + 3
			for j+2 < len(content) && !(content[j] == '\'' && content[j+1] == '\'' && content[j+2] == '\'') {
				if content[j] == '\\' && j+1 < len(content) {
					j += 2
					continue
				}
				j++
			}
			if j+2 < len(content) {
				j += 3
			}
			seg2, c2 := replacePairsInSegment(content[i:j], pairs)
			n += c2
			out.WriteString(seg2)
			i = j
			continue
		}
		// prefixes: r b f u fr rf ... before quote
		if isPyStringPrefixStart(content, i) {
			k := i
			for k < len(content) && isPyPrefixByte(content[k]) {
				k++
			}
			if k < len(content) && (content[k] == '"' || content[k] == '\'') {
				// include prefix in segment start for rewrite of body only — rewrite whole string token
				quote := content[k]
				// check triple
				if k+2 < len(content) && content[k+1] == quote && content[k+2] == quote {
					j := k + 3
					for j+2 < len(content) && !(content[j] == quote && content[j+1] == quote && content[j+2] == quote) {
						if content[j] == '\\' {
							j += 2
							continue
						}
						j++
					}
					if j+2 < len(content) {
						j += 3
					}
					seg2, c2 := replacePairsInSegment(content[i:j], pairs)
					n += c2
					out.WriteString(seg2)
					i = j
					continue
				}
				j := k + 1
				for j < len(content) {
					if content[j] == '\\' && j+1 < len(content) {
						j += 2
						continue
					}
					if content[j] == quote {
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
		}
		// plain " or '
		if content[i] == '"' || content[i] == '\'' {
			quote := content[i]
			j := i + 1
			for j < len(content) {
				if content[j] == '\\' && j+1 < len(content) {
					j += 2
					continue
				}
				if content[j] == quote {
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
		out.WriteByte(content[i])
		i++
	}
	return out.String(), n
}

func isPyPrefixByte(b byte) bool {
	switch b {
	case 'r', 'R', 'b', 'B', 'f', 'F', 'u', 'U':
		return true
	default:
		return false
	}
}

// isPyStringPrefixStart is true if at i we might have rf"..." / b'''...'''
func isPyStringPrefixStart(s string, i int) bool {
	if i >= len(s) || !isPyPrefixByte(s[i]) {
		return false
	}
	// must not be middle of identifier: previous char not id
	if i > 0 {
		p := s[i-1]
		if p == '_' || (p >= 'a' && p <= 'z') || (p >= 'A' && p <= 'Z') || (p >= '0' && p <= '9') {
			return false
		}
	}
	return true
}
