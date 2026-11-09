package abap

import (
	"fmt"
	"strings"
)

// SyntaxMessage represents a diagnostic finding from offline validation
type SyntaxMessage struct {
	Severity string // "error", "warning", "info", "hint"
	Text     string
	Line     int
	Column   int
	Code     string
}

// ValidateSource performs offline syntax validation on ABAP source code.
// isSaved controls whether strict checks (like period validation) are applied.
func ValidateSource(source string, isSaved bool) []SyntaxMessage {
	lines := strings.Split(source, "\n")
	var msgs []SyntaxMessage

	// Per-line checks
	for lineNum, line := range lines {
		msgs = append(msgs, validateLine(line, lineNum, isSaved)...)
	}

	// Structural checks
	msgs = append(msgs, validateControlStructures(lines)...)

	return msgs
}

func validateLine(line string, lineNum int, isSaved bool) []SyntaxMessage {
	var msgs []SyntaxMessage
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "\"") {
		return msgs
	}

	upperLine := strings.ToUpper(trimmed)

	// Line length check
	const maxLineLength = 120
	if len(line) > maxLineLength {
		msgs = append(msgs, SyntaxMessage{
			Severity: "warning",
			Text:     fmt.Sprintf("Line too long (%d > %d characters)", len(line), maxLineLength),
			Line:     lineNum,
			Column:   maxLineLength,
			Code:     "line-too-long",
		})
	}

	// Period check - only for saved documents
	if isSaved && shouldEndWithPeriod(upperLine) &&
		!strings.HasSuffix(trimmed, ".") && !strings.HasSuffix(trimmed, ":") {
		msgs = append(msgs, SyntaxMessage{
			Severity: "error",
			Text:     "Missing period at end of statement",
			Line:     lineNum,
			Column:   len(line),
			Code:     "missing-period",
		})
	}

	// Keyword validation
	words := strings.Fields(upperLine)
	if len(words) > 0 {
		firstWord := words[0]
		if LooksLikeKeyword(firstWord) && !isValidKeyword(firstWord) {
			suggestion := SuggestKeyword(firstWord, abapKeywords)
			message := fmt.Sprintf("Unknown keyword '%s'", firstWord)
			if suggestion != "" {
				message += fmt.Sprintf(". Did you mean '%s'?", suggestion)
			}
			severity := "error"
			if !isSaved {
				severity = "warning"
			}
			msgs = append(msgs, SyntaxMessage{
				Severity: severity,
				Text:     message,
				Line:     lineNum,
				Column:   0,
				Code:     "unknown-keyword",
			})
		}
	}

	// Deprecated statements
	msgs = append(msgs, validateDeprecated(trimmed, lineNum, isSaved)...)

	return msgs
}

func validateDeprecated(line string, lineNum int, isSaved bool) []SyntaxMessage {
	var msgs []SyntaxMessage
	upperLine := strings.ToUpper(strings.TrimSpace(line))
	deprecated := map[string]string{
		"MOVE":     "Use assignment operator (=) instead",
		"ADD":      "Use += operator instead",
		"SUBTRACT": "Use -= operator instead",
		"MULTIPLY": "Use *= operator instead",
		"DIVIDE":   "Use /= operator instead",
		"COMPUTE":  "Use direct assignment instead",
	}
	words := strings.Fields(upperLine)
	if len(words) > 0 {
		if replacement, ok := deprecated[words[0]]; ok {
			severity := "warning"
			if !isSaved {
				severity = "info"
			}
			msgs = append(msgs, SyntaxMessage{
				Severity: severity,
				Text:     fmt.Sprintf("'%s' is deprecated. %s", words[0], replacement),
				Line:     lineNum,
				Column:   0,
				Code:     "deprecated-statement",
			})
		}
	}
	return msgs
}

func validateControlStructures(lines []string) []SyntaxMessage {
	var msgs []SyntaxMessage
	type stackEntry struct {
		keyword string
		line    int
	}
	var stack []stackEntry

	pairs := map[string]string{
		"ENDIF":    "IF",
		"ENDDO":    "DO",
		"ENDWHILE": "WHILE",
		"ENDLOOP":  "LOOP",
		"ENDCASE":  "CASE",
		"ENDTRY":   "TRY",
	}

	openers := map[string]bool{
		"IF": true, "DO": true, "WHILE": true,
		"LOOP": true, "CASE": true, "TRY": true,
	}

	for lineNum, line := range lines {
		upper := strings.ToUpper(strings.TrimSpace(line))
		words := strings.Fields(upper)
		if len(words) == 0 {
			continue
		}
		first := strings.TrimSuffix(words[0], ".")

		if openers[first] {
			stack = append(stack, stackEntry{first, lineNum})
		} else if expected, isCloser := pairs[first]; isCloser {
			if len(stack) > 0 && stack[len(stack)-1].keyword == expected {
				stack = stack[:len(stack)-1]
			} else if len(stack) > 0 {
				msgs = append(msgs, SyntaxMessage{
					Severity: "error",
					Text:     fmt.Sprintf("%s without matching %s (found %s at line %d)", first, expected, stack[len(stack)-1].keyword, stack[len(stack)-1].line+1),
					Line:     lineNum,
					Column:   0,
					Code:     "unmatched-" + strings.ToLower(first),
				})
			} else {
				msgs = append(msgs, SyntaxMessage{
					Severity: "error",
					Text:     fmt.Sprintf("%s without matching %s", first, expected),
					Line:     lineNum,
					Column:   0,
					Code:     "unmatched-" + strings.ToLower(first),
				})
			}
		}
	}

	// Report unclosed openers
	for _, entry := range stack {
		msgs = append(msgs, SyntaxMessage{
			Severity: "error",
			Text:     fmt.Sprintf("%s statement without matching END%s", entry.keyword, entry.keyword),
			Line:     entry.line,
			Column:   0,
			Code:     "unmatched-" + strings.ToLower(entry.keyword),
		})
	}

	return msgs
}

func isValidKeyword(word string) bool {
	for _, kw := range abapKeywords {
		if kw == word {
			return true
		}
	}
	return false
}

// shouldEndWithPeriod checks if an ABAP statement should end with a period.
// Ported from abap-lsp/internal/abap/abap.go ShouldEndWithPeriod.
func shouldEndWithPeriod(upperLine string) bool {
	line := strings.TrimSpace(upperLine)
	if line == "" || strings.HasPrefix(line, "*") {
		return false
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return false
	}
	firstWord := words[0]

	if strings.HasSuffix(line, ":") {
		return false
	}

	noperiod := []string{
		"IF", "ELSEIF", "ELSE", "ENDIF",
		"CASE", "WHEN", "ENDCASE",
		"DO", "ENDDO", "WHILE", "ENDWHILE",
		"LOOP", "ENDLOOP", "AT", "ENDAT",
		"TRY", "ENDTRY", "CATCH", "CLEANUP",
		"SELECT", "ENDSELECT", "PROVIDE", "ENDPROVIDE",
		"EXEC", "ENDEXEC",
		"CLASS", "ENDCLASS", "INTERFACE", "ENDINTERFACE",
		"METHOD", "ENDMETHOD", "FORM", "ENDFORM",
		"FUNCTION", "ENDFUNCTION", "MODULE", "ENDMODULE",
		"DEFINITION", "IMPLEMENTATION",
		"PUBLIC", "PROTECTED", "PRIVATE", "SECTION",
		"REPORT", "PROGRAM", "INCLUDE", "TYPE-POOL", "FUNCTION-POOL",
		"CLASS-POOL", "INTERFACE-POOL", "START-OF-SELECTION",
		"END-OF-SELECTION", "TOP-OF-PAGE", "END-OF-PAGE",
		"INITIALIZATION", "LOAD-OF-PROGRAM", "SELECTION-SCREEN",
		"IMPORTING", "EXPORTING", "CHANGING", "RETURNING",
		"RAISING", "EXCEPTIONS", "USING", "TABLES",
		"EVENTS", "CLASS-EVENTS",
	}
	for _, kw := range noperiod {
		if firstWord == kw {
			return false
		}
	}

	// Chain declarations with comma
	if (firstWord == "METHODS" || firstWord == "CLASS-METHODS" ||
		firstWord == "DATA" || firstWord == "CLASS-DATA" ||
		firstWord == "TYPES" || firstWord == "CONSTANTS" ||
		firstWord == "ALIASES" || firstWord == "INTERFACES") &&
		strings.Contains(line, ",") && !strings.Contains(line, ".") {
		return false
	}

	// Statement keywords that need periods
	stmtKeywords := []string{
		"DATA", "TYPES", "CONSTANTS", "STATICS", "FIELD-SYMBOLS",
		"PARAMETERS", "SELECT-OPTIONS", "RANGES",
		"WRITE", "MESSAGE", "SKIP", "ULINE", "NEW-LINE", "NEW-PAGE",
		"MOVE", "CLEAR", "REFRESH", "FREE", "ASSIGN",
		"APPEND", "INSERT", "DELETE", "MODIFY", "READ",
		"SORT", "DESCRIBE", "COLLECT", "SUM",
		"SPLIT", "CONCATENATE", "CONDENSE", "TRANSLATE",
		"REPLACE", "FIND", "SEARCH", "SHIFT", "OVERLAY",
		"ADD", "SUBTRACT", "MULTIPLY", "DIVIDE", "COMPUTE",
		"CALL", "PERFORM", "SUBMIT", "LEAVE", "EXIT", "CONTINUE",
		"CHECK", "RETURN", "STOP", "RAISE",
		"COMMIT", "ROLLBACK", "OPEN", "CLOSE", "FETCH",
		"HIDE", "GET", "PUT", "SET", "UNPACK", "PACK",
		"BREAK-POINT", "ASSERT", "LOG-POINT",
		"AUTHORITY-CHECK", "EDITOR-CALL",
		"CREATE", "CONVERT",
	}
	for _, kw := range stmtKeywords {
		if firstWord == kw {
			return true
		}
	}

	if strings.Contains(line, " = ") || strings.Contains(line, "=") {
		return true
	}

	if len(words) > 1 {
		return true
	}

	return false
}
