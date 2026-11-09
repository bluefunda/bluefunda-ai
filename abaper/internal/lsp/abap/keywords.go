package abap

import (
	"strings"
	"unicode"
)

// IsWordChar checks if a character is valid for ABAP identifiers
func IsWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

// GetWordAtPosition extracts the word at the given character position in a line
func GetWordAtPosition(line string, character int) string {
	if character < 0 || character >= len(line) {
		return ""
	}

	start := character
	for start > 0 && IsWordChar(rune(line[start-1])) {
		start--
	}

	end := character
	for end < len(line) && IsWordChar(rune(line[end])) {
		end++
	}

	if start >= end {
		return ""
	}
	return line[start:end]
}

// IsSystemField checks if a name is a system field
func IsSystemField(name string) bool {
	upperName := strings.ToUpper(name)
	for _, field := range systemFields {
		if field == upperName {
			return true
		}
	}
	return strings.HasPrefix(upperName, "SY-")
}

// LooksLikeKeyword checks if a word looks like it should be an ABAP keyword
func LooksLikeKeyword(word string) bool {
	if len(word) < 2 {
		return false
	}
	if strings.Contains(word, "_") || strings.Contains(word, "-") {
		return false
	}
	if word != strings.ToUpper(word) && word != strings.ToLower(word) {
		return false
	}
	return strings.ToUpper(word) == word
}

// SuggestKeyword suggests the closest valid keyword for a misspelled word
func SuggestKeyword(word string, keywords []string) string {
	if len(word) == 0 {
		return ""
	}
	bestMatch := ""
	bestScore := 999
	for _, keyword := range keywords {
		score := levenshteinDistance(word, keyword)
		if score < bestScore && score <= len(word)/2 {
			bestScore = score
			bestMatch = keyword
		}
	}
	return bestMatch
}

func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := 1; j <= len(s2); j++ {
		matrix[0][j] = j
	}
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			del := matrix[i-1][j] + 1
			ins := matrix[i][j-1] + 1
			sub := matrix[i-1][j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			matrix[i][j] = m
		}
	}
	return matrix[len(s1)][len(s2)]
}

// GetABAPKeywords returns the list of standard ABAP keywords
func GetABAPKeywords() []string {
	return abapKeywords
}

// GetABAPFunctions returns the list of common ABAP functions
func GetABAPFunctions() []string {
	return abapFunctions
}

// GetSystemFields returns the list of ABAP system fields
func GetSystemFields() []string {
	return systemFields
}

var systemFields = []string{
	"SY-SUBRC", "SY-TABIX", "SY-INDEX", "SY-DBCNT", "SY-UNAME",
	"SY-DATUM", "SY-UZEIT", "SY-TCODE", "SY-REPID", "SY-DYNNR",
	"SY-LANGU", "SY-MANDT", "SY-SYSID", "SY-HOST", "SY-OPSYS",
	"SY-PFKEY", "SY-TITLE", "SY-LINCT", "SY-LINNO", "SY-PAGNO",
	"SY-COLNO", "SY-LINSZ", "SY-MSGID", "SY-MSGNO", "SY-MSGTY",
	"SY-MSGV1", "SY-MSGV2", "SY-MSGV3", "SY-MSGV4",
}

var abapKeywords = []string{
	// Data declarations
	"DATA", "TYPES", "CONSTANTS", "STATICS", "FIELD-SYMBOLS",
	"PARAMETERS", "SELECT-OPTIONS", "RANGES",

	// Control structures
	"IF", "ELSEIF", "ELSE", "ENDIF",
	"CASE", "WHEN", "ENDCASE",
	"DO", "ENDDO", "WHILE", "ENDWHILE",
	"LOOP", "ENDLOOP", "AT", "ENDAT",

	// Database operations
	"SELECT", "FROM", "WHERE", "INTO", "ENDSELECT",
	"INSERT", "UPDATE", "DELETE", "MODIFY",
	"OPEN", "CLOSE", "FETCH", "CURSOR",
	"COMMIT", "ROLLBACK", "WORK",

	// Output and messaging
	"WRITE", "MESSAGE", "SKIP", "ULINE", "NEW-LINE", "NEW-PAGE",
	"FORMAT", "BACK", "POSITION", "HOTSPOT",

	// Procedure calls
	"CALL", "PERFORM", "SUBMIT", "LEAVE",
	"FUNCTION", "ENDFUNCTION", "METHOD", "ENDMETHOD",
	"FORM", "ENDFORM", "USING", "CHANGING", "TABLES",

	// Object-oriented
	"CLASS", "ENDCLASS", "INTERFACE", "ENDINTERFACE",
	"PUBLIC", "PROTECTED", "PRIVATE", "SECTION",
	"INHERITING", "FINAL", "ABSTRACT", "INTERFACES",
	"METHODS", "EVENTS", "CLASS-METHODS", "CLASS-DATA",
	"ALIASES", "REDEFINITION", "IMPLEMENTATION",

	// Exception handling
	"TRY", "ENDTRY", "CATCH", "CLEANUP", "RAISE", "RESUME",
	"EXCEPTIONS", "ERROR", "OTHERS",

	// Data manipulation
	"MOVE", "CLEAR", "REFRESH", "FREE", "ASSIGN",
	"APPEND", "SORT", "DESCRIBE", "COLLECT", "SUM",
	"SPLIT", "CONCATENATE", "CONDENSE", "TRANSLATE",
	"REPLACE", "FIND", "SEARCH", "SHIFT", "OVERLAY",

	// Arithmetic
	"ADD", "SUBTRACT", "MULTIPLY", "DIVIDE", "COMPUTE",
	"MOD", "DIV",

	// Comparison and logical
	"EQ", "NE", "LT", "LE", "GT", "GE", "CP", "NP", "CA", "NA",
	"CS", "NS", "CO", "CN", "AND", "OR", "NOT", "BETWEEN",
	"IN", "IS", "INITIAL", "BOUND", "ASSIGNED", "REQUESTED",

	// Type operations
	"TYPE", "LIKE", "REF", "TO", "VALUE", "DEFAULT",
	"OPTIONAL", "PREFERRED", "RADIOBUTTON", "CHECKBOX",
	"AS", "LINE", "OF", "WITH", "UNIQUE", "NON-UNIQUE",
	"KEY", "HEADER", "OCCURS", "MEMORY", "ID",

	// Program structure
	"REPORT", "PROGRAM", "INCLUDE", "LOAD-OF-PROGRAM",
	"START-OF-SELECTION", "END-OF-SELECTION", "TOP-OF-PAGE",
	"END-OF-PAGE", "SELECTION-SCREEN",

	// Macros and includes
	"DEFINE", "END-OF-DEFINITION",

	// Miscellaneous
	"CHECK", "CONTINUE", "EXIT", "STOP", "RETURN",
	"HIDE", "GET", "PUT", "SET", "UNPACK", "PACK",
	"EDITOR-CALL", "AUTHORITY-CHECK", "BREAK-POINT",
	"ASSERT", "LOG-POINT", "ENHANCEMENT-POINT",
	"ENHANCEMENT-SECTION", "ENDHANCEMENT-SECTION",
	"BREAK", "COMMUNICATION", "CONTEXT", "CONVERSION",
	"CUSTOMER", "ENHANCEMENT", "FILTER", "CURRENCY",
	"DECIMALS", "ROUND", "LEFT-JUSTIFIED", "RIGHT-JUSTIFIED",
	"CENTERED", "UNDER", "NO-GAP", "INTENSIFIED", "INVERSE",
	"INPUT", "OUTPUT", "COLOR", "ICON", "SYMBOL",

	// Modern ABAP
	"CORRESPONDING", "EXACT", "CONV", "CAST", "NEW",
	"FOR", "LET", "THEN", "UNTIL", "STEP",
	"GROUP", "BY", "ASCENDING", "DESCENDING",
	"REDUCE", "COND", "SWITCH",

	// ABAP Objects
	"CREATE", "OBJECT", "INSTANCE", "STATIC",
	"DEFINITION",
	"IMPORTING", "EXPORTING", "RETURNING", "RAISING",
	"CONSTRUCTOR", "DESTRUCTOR", "CLASS-CONSTRUCTOR",

	// ALV and GUI
	"SCREEN", "TRANSACTION", "DYNPRO", "PBO", "PAI",
	"MODULE", "ENDMODULE", "ON", "CHAIN", "ENDCHAIN",
	"FIELD", "CONTROL", "TABSTRIP",

	// Web services
	"SERVICE", "ENTITY", "ASSOCIATION", "COMPOSITION",
	"ANNOTATION", "EXTEND", "METADATA", "ODATA",

	// Additional
	"READ", "TABLE",
	"SINGLE", "UP", "FIRST", "LAST",
	"BINARY", "TRANSPORTING", "NO", "FIELDS",
	"ALL", "ENTRIES", "WITHOUT",
}

var abapFunctions = []string{
	// String functions
	"strlen", "substring", "condense", "translate", "replace",
	"find", "split", "concatenate", "shift", "overlay",
	"search", "contains", "matches", "count", "reverse",
	"to_upper", "to_lower", "to_mixed", "escape",

	// Math functions
	"abs", "ceil", "floor", "frac", "sign", "sqrt", "exp",
	"log", "log10", "cos", "sin", "tan", "acos", "asin",
	"atan", "cosh", "sinh", "tanh", "trunc", "round", "ipow",

	// Table functions
	"lines", "read", "loop", "append", "insert", "delete",
	"clear", "refresh", "free", "sort", "collect", "modify",
	"describe", "move-corresponding",

	// Type functions
	"rtts", "rtti", "cast", "instanceof",

	// Date/Time functions
	"sy-datum", "sy-uzeit", "sy-tzone", "convert",
	"timestamp", "timezone",

	// System functions
	"sy-subrc", "sy-tabix", "sy-index", "sy-dbcnt",
	"authority-check", "get-parameter", "set-parameter",

	// Database aggregate functions
	"sum", "avg", "min", "max", "distinct",

	// Conversion functions
	"conversion_exit", "unit_conversion", "currency_conversion",
}

// KeywordDocumentation maps ABAP keywords to their descriptions
var KeywordDocumentation = map[string]string{
	"DATA":          "Declares data objects (variables)",
	"TYPES":         "Declares data types",
	"CONSTANTS":     "Declares constants",
	"STATICS":       "Declares static variables",
	"FIELD-SYMBOLS": "Declares field symbols (generic pointers)",
	"IF":            "Conditional statement",
	"ELSEIF":        "Additional condition in IF statement",
	"ELSE":          "Alternative branch in IF statement",
	"ENDIF":         "Ends IF statement",
	"LOOP":          "Loop statement for iterating over internal tables",
	"ENDLOOP":       "Ends LOOP statement",
	"DO":            "Unconditional loop",
	"ENDDO":         "Ends DO statement",
	"WHILE":         "Conditional loop",
	"ENDWHILE":      "Ends WHILE statement",
	"SELECT":        "SQL statement for database access",
	"FROM":          "Specifies source table in SELECT",
	"WHERE":         "Specifies conditions in SELECT",
	"INTO":          "Specifies target in SELECT",
	"ENDSELECT":     "Ends SELECT statement",
	"WRITE":         "Output statement",
	"MESSAGE":       "Displays messages to user",
	"CALL":          "Calls procedures, functions, or methods",
	"PERFORM":       "Calls a form routine",
	"MOVE":          "Assigns values between variables",
	"CLEAR":         "Resets variables to initial values",
	"REFRESH":       "Clears internal table content",
	"APPEND":        "Adds a line to an internal table",
	"INSERT":        "Inserts lines into internal table",
	"DELETE":        "Removes lines from internal table",
	"MODIFY":        "Changes lines in internal table",
	"READ":          "Reads lines from internal table",
	"SORT":          "Sorts internal table",
	"DESCRIBE":      "Gets information about data objects",
	"CLASS":         "Object-oriented class definition",
	"ENDCLASS":      "Ends class definition",
	"METHOD":        "Method definition within a class",
	"ENDMETHOD":     "Ends method definition",
	"INTERFACE":     "Interface definition for object-oriented programming",
	"ENDINTERFACE":  "Ends interface definition",
	"PUBLIC":        "Public visibility section",
	"PROTECTED":     "Protected visibility section",
	"PRIVATE":       "Private visibility section",
	"SECTION":       "Visibility section keyword",
	"TRY":           "Exception handling block",
	"ENDTRY":        "Ends exception handling block",
	"CATCH":         "Exception handling catch block",
	"CLEANUP":       "Cleanup block in exception handling",
	"RAISE":         "Raises an exception",
	"FORM":          "Form routine definition",
	"ENDFORM":       "Ends form routine",
	"FUNCTION":      "Function module definition",
	"ENDFUNCTION":   "Ends function module",
	"REPORT":        "Defines an executable program",
	"PROGRAM":       "Alternative to REPORT",
	"INCLUDE":       "Includes another program",
	"CASE":          "Multi-way conditional statement",
	"WHEN":          "Branch in CASE statement",
	"ENDCASE":       "Ends CASE statement",
	"EXIT":          "Exits current loop or block",
	"CONTINUE":      "Continues with next iteration",
	"CHECK":         "Conditional statement processing",
	"RETURN":        "Returns from current processing block",
	"STOP":          "Stops program execution",
	"LEAVE":         "Leaves current processing block",
}

// FunctionDocumentation maps ABAP functions to their descriptions
var FunctionDocumentation = map[string]string{
	"strlen":      "Returns the length of a string",
	"substring":   "Extracts a substring from a string",
	"condense":    "Removes extra spaces from a string",
	"translate":   "Translates characters in a string",
	"replace":     "Replaces occurrences of a substring",
	"find":        "Searches for a substring within a string",
	"split":       "Splits a string into components",
	"concatenate": "Concatenates multiple strings",
	"shift":       "Shifts string content left or right",
	"overlay":     "Overlays one string onto another",
	"search":      "Searches for patterns in strings",
	"contains":    "Checks if string contains substring",
	"matches":     "Pattern matching with regular expressions",
	"count":       "Counts occurrences of substring",
	"reverse":     "Reverses string content",
	"to_upper":    "Converts string to uppercase",
	"to_lower":    "Converts string to lowercase",
	"to_mixed":    "Converts string to mixed case",
	"escape":      "Escapes special characters",
	"abs":         "Returns the absolute value of a number",
	"ceil":        "Returns the ceiling of a number",
	"floor":       "Returns the floor of a number",
	"frac":        "Returns the fractional part of a number",
	"sign":        "Returns the sign of a number",
	"sqrt":        "Returns the square root",
	"exp":         "Returns e raised to the power",
	"log":         "Returns natural logarithm",
	"log10":       "Returns base-10 logarithm",
	"cos":         "Returns cosine",
	"sin":         "Returns sine",
	"tan":         "Returns tangent",
	"acos":        "Returns arc cosine",
	"asin":        "Returns arc sine",
	"atan":        "Returns arc tangent",
	"cosh":        "Returns hyperbolic cosine",
	"sinh":        "Returns hyperbolic sine",
	"tanh":        "Returns hyperbolic tangent",
	"trunc":       "Truncates decimal places",
	"round":       "Rounds to specified decimal places",
	"ipow":        "Integer power function",
	"lines":       "Returns number of lines in internal table",
}

// SystemFieldDocumentation maps system fields to their descriptions
var SystemFieldDocumentation = map[string]string{
	"SY-SUBRC": "Return code of last operation (0 = success)",
	"SY-TABIX": "Current line index in internal table operations",
	"SY-INDEX": "Current loop counter in DO/WHILE loops",
	"SY-DBCNT": "Number of database records processed",
	"SY-UNAME": "Current user name",
	"SY-DATUM": "Current system date (YYYYMMDD)",
	"SY-UZEIT": "Current system time (HHMMSS)",
	"SY-TCODE": "Current transaction code",
	"SY-REPID": "Current program name",
	"SY-DYNNR": "Current screen number",
	"SY-LANGU": "Current language key",
	"SY-MANDT": "Current client",
	"SY-SYSID": "System ID",
	"SY-HOST":  "Application server name",
	"SY-OPSYS": "Operating system",
	"SY-PFKEY": "Current GUI status",
	"SY-TITLE": "Program title",
	"SY-LINCT": "Number of lines per page",
	"SY-LINNO": "Current line number in list",
	"SY-PAGNO": "Current page number",
	"SY-COLNO": "Current column number",
	"SY-LINSZ": "Line size",
	"SY-MSGID": "Message class of last message",
	"SY-MSGNO": "Message number of last message",
	"SY-MSGTY": "Message type of last message",
	"SY-MSGV1": "Message variable 1",
	"SY-MSGV2": "Message variable 2",
	"SY-MSGV3": "Message variable 3",
	"SY-MSGV4": "Message variable 4",
}
