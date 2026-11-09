package abap

import (
	"regexp"
	"strings"
	"unicode"
)

// Format formats ABAP source code according to conventions:
// keywords -> UPPERCASE, variables -> lowercase, preserve strings and comments.
func Format(source string) (string, error) {
	lines := strings.Split(source, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, formatLine(line))
	}
	return strings.Join(result, "\n"), nil
}

func formatLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "*") {
		return line
	}
	leading := getLeadingWhitespace(line)
	content := strings.TrimLeft(line, " \t")
	formatted := formatLineContent(content)
	return leading + formatted
}

func getLeadingWhitespace(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return line
}

func formatLineContent(content string) string {
	if content == "" {
		return content
	}

	keywordMap := buildKeywordMap()
	tokens := tokenizeLine(content)

	var processed []string
	for _, tok := range tokens {
		processed = append(processed, processToken(tok, keywordMap))
	}
	return rejoinTokens(processed)
}

func buildKeywordMap() map[string]bool {
	m := make(map[string]bool)
	for _, kw := range abapKeywords {
		m[strings.ToUpper(kw)] = true
	}
	extra := []string{
		"TO", "WITH", "WITHOUT", "FOR", "ALL", "ENTRIES",
		"SINGLE", "UP", "FIRST", "LAST", "DESCENDING", "ASCENDING",
		"BINARY", "SEARCH", "TRANSPORTING", "NO", "FIELDS",
	}
	for _, kw := range extra {
		m[kw] = true
	}
	return m
}

func tokenizeLine(content string) []string {
	var tokens []string
	var cur strings.Builder
	inString := false
	var stringChar byte

	for i := 0; i < len(content); i++ {
		ch := content[i]

		if (ch == '\'' || ch == '"') && !inString {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			inString = true
			stringChar = ch
			cur.WriteByte(ch)
		} else if inString && ch == stringChar {
			cur.WriteByte(ch)
			tokens = append(tokens, cur.String())
			cur.Reset()
			inString = false
			stringChar = 0
		} else if inString {
			cur.WriteByte(ch)
		} else if unicode.IsSpace(rune(ch)) {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			for i+1 < len(content) && unicode.IsSpace(rune(content[i+1])) {
				i++
			}
		} else if isSpecialChar(ch) {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			if i+1 < len(content) {
				two := string([]byte{ch, content[i+1]})
				if isMultiCharOp(two) {
					tokens = append(tokens, two)
					i++
					continue
				}
			}
			tokens = append(tokens, string(ch))
		} else {
			cur.WriteByte(ch)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func isSpecialChar(ch byte) bool {
	return strings.ContainsRune(".,()[]{}:;=+-*/<>!&|~^%@#", rune(ch))
}

func isMultiCharOp(s string) bool {
	ops := []string{"<=", ">=", "<>", "=>", "->", "::", "==", "!=", "&&", "||", "**"}
	for _, op := range ops {
		if s == op {
			return true
		}
	}
	return false
}

func processToken(token string, keywordMap map[string]bool) string {
	if isStringLiteral(token) || isNumber(token) || isOperatorOrSpecial(token) {
		return token
	}

	upper := strings.ToUpper(token)

	if keywordMap[upper] {
		return upper
	}
	if isCompoundKeyword(upper) {
		return upper
	}
	if strings.HasPrefix(upper, "SY-") || upper == "SPACE" || upper == "ABAP_TRUE" || upper == "ABAP_FALSE" {
		return upper
	}
	return strings.ToLower(token)
}

func isStringLiteral(w string) bool {
	return (strings.HasPrefix(w, "'") && strings.HasSuffix(w, "'")) ||
		(strings.HasPrefix(w, "\"") && strings.HasSuffix(w, "\""))
}

var numberRe = regexp.MustCompile(`^-?\d+(\.\d+)?([eE][+-]?\d+)?$`)

func isNumber(token string) bool {
	return len(token) > 0 && numberRe.MatchString(token)
}

func isOperatorOrSpecial(token string) bool {
	if len(token) == 0 {
		return false
	}
	if len(token) == 1 {
		return isSpecialChar(token[0])
	}
	return isMultiCharOp(token)
}

func isCompoundKeyword(token string) bool {
	compound := []string{
		"TYPE-POOL", "TYPE-POOLS", "FUNCTION-POOL", "CLASS-POOL", "INTERFACE-POOL",
		"START-OF-SELECTION", "END-OF-SELECTION", "TOP-OF-PAGE", "END-OF-PAGE",
		"SELECTION-SCREEN", "AT-SELECTION-SCREEN", "LOAD-OF-PROGRAM",
		"CLASS-DATA", "CLASS-METHODS", "CLASS-EVENTS", "INSTANCE-METHODS",
		"BREAK-POINT", "AUTHORITY-CHECK", "EDITOR-CALL", "LOG-POINT",
		"NEW-LINE", "NEW-PAGE", "ENHANCEMENT-POINT", "ENHANCEMENT-SECTION",
		"END-OF-DEFINITION", "ENDHANCEMENT-SECTION",
		"SELECT-OPTIONS", "FIELD-SYMBOLS", "CORRESPONDING-FIELDS",
	}
	for _, kw := range compound {
		if token == kw {
			return true
		}
	}
	return false
}

func rejoinTokens(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	var b strings.Builder
	for i, tok := range tokens {
		b.WriteString(tok)
		if i < len(tokens)-1 {
			if shouldAddSpace(tok, tokens[i+1]) {
				b.WriteByte(' ')
			}
		}
	}
	return b.String()
}

func shouldAddSpace(current, next string) bool {
	if next == ")" || next == "]" || next == "}" {
		return false
	}
	if current == "(" || current == "[" || current == "{" {
		return false
	}
	if next == "," || next == ";" || next == "." {
		return false
	}
	if current == "." {
		return false
	}
	if current == "::" || next == "::" {
		return false
	}
	if current == "->" || next == "->" {
		return false
	}
	return true
}
