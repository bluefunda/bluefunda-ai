package types

// SyntaxCheckResult holds the result of a syntax check operation
type SyntaxCheckResult struct {
	ObjectName string               `json:"object_name"`
	ObjectType string               `json:"object_type"`
	Messages   []SyntaxCheckMessage `json:"messages"`
}

// SyntaxCheckMessage represents a single syntax check finding
type SyntaxCheckMessage struct {
	Severity string `json:"severity"` // "error", "warning", "info", "hint"
	Text     string `json:"text"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	EndLine  int    `json:"end_line"`
	EndCol   int    `json:"end_col"`
	Code     string `json:"code,omitempty"`
}

// CompletionProposal represents a code completion suggestion
type CompletionProposal struct {
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	Kind        string `json:"kind"` // "keyword", "function", "variable", "class", "type"
	InsertText  string `json:"insert_text,omitempty"`
}

// NavigationTarget represents a go-to-definition target
type NavigationTarget struct {
	URI        string `json:"uri"`
	ObjectName string `json:"object_name"`
	ObjectType string `json:"object_type"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
}
