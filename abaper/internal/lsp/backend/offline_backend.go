package backend

import (
	"fmt"
	"log"
	"strings"

	"github.com/bluefunda/abaper/internal/lsp/abap"
	"github.com/bluefunda/abaper/internal/lsp/abaplint"
	"github.com/bluefunda/abaper/types"
)

// OfflineBackend provides LSP features without a live SAP connection.
// It uses abaplint for diagnostics and local ABAP keyword knowledge
// for completion and formatting.
type OfflineBackend struct {
	linter *abaplint.Runner
}

// NewOfflineBackend creates a new offline backend. workDir is the workspace
// root used to find an optional abaplint.json config.
func NewOfflineBackend(workDir string) *OfflineBackend {
	b := &OfflineBackend{}
	runner, err := abaplint.NewRunner(workDir)
	if err != nil {
		log.Printf("abaplint not available, falling back to basic validation: %v", err)
	} else {
		b.linter = runner
	}
	return b
}

// SyntaxCheck runs abaplint for diagnostics, falling back to basic validation
// if abaplint is not available.
func (b *OfflineBackend) SyntaxCheck(objectType, objectName, source string) (*types.SyntaxCheckResult, error) {
	if b.linter != nil {
		return b.linter.Check(objectType, objectName, source)
	}
	// Fallback: basic offline validation
	msgs := abap.ValidateSource(source, true)
	result := &types.SyntaxCheckResult{
		ObjectName: objectName,
		ObjectType: objectType,
	}
	for _, m := range msgs {
		result.Messages = append(result.Messages, types.SyntaxCheckMessage{
			Severity: m.Severity,
			Text:     m.Text,
			Line:     m.Line,
			Column:   m.Column,
			Code:     m.Code,
		})
	}
	return result, nil
}

// Complete returns keyword/function completions matching the prefix at the cursor
func (b *OfflineBackend) Complete(objectType, objectName, source string, line, column int) ([]types.CompletionProposal, error) {
	lines := strings.Split(source, "\n")
	if line >= len(lines) {
		return nil, nil
	}
	currentLine := lines[line]
	prefix := getCompletionPrefix(currentLine, column)

	var proposals []types.CompletionProposal

	// Keyword completions
	for _, kw := range abap.GetABAPKeywords() {
		if strings.HasPrefix(kw, prefix) {
			proposals = append(proposals, types.CompletionProposal{
				Identifier:  kw,
				Description: "ABAP Keyword",
				Kind:        "keyword",
			})
		}
	}

	// Function completions
	for _, fn := range abap.GetABAPFunctions() {
		if strings.HasPrefix(strings.ToUpper(fn), prefix) {
			proposals = append(proposals, types.CompletionProposal{
				Identifier:  fn,
				Description: "ABAP Function",
				Kind:        "function",
			})
		}
	}

	// Contextual completions
	proposals = append(proposals, getContextualCompletions(currentLine, prefix)...)

	return proposals, nil
}

// Navigate is not supported in offline mode
func (b *OfflineBackend) Navigate(objectType, objectName, source string, line, column int) (*types.NavigationTarget, error) {
	return nil, nil
}

// Format formats ABAP source code
func (b *OfflineBackend) Format(source string) (string, error) {
	return abap.Format(source)
}

// Activate is not supported in offline mode
func (b *OfflineBackend) Activate(objectType, objectName string) (*types.ActivationResult, error) {
	return nil, fmt.Errorf("activation requires a live SAP connection")
}

// IsConnected always returns false for the offline backend
func (b *OfflineBackend) IsConnected() bool {
	return false
}

func getCompletionPrefix(line string, column int) string {
	if column <= 0 || column > len(line) {
		return ""
	}
	start := column - 1
	for start >= 0 && abap.IsWordChar(rune(line[start])) {
		start--
	}
	start++
	return strings.ToUpper(line[start:column])
}

func getContextualCompletions(line, prefix string) []types.CompletionProposal {
	var items []types.CompletionProposal
	upper := strings.ToUpper(strings.TrimSpace(line))

	// After TYPE keyword: suggest data types
	if strings.Contains(upper, "TYPE") && !strings.Contains(upper, "TYPE-POOL") {
		dataTypes := []string{"STRING", "I", "P", "F", "D", "T", "C", "N", "X", "XSTRING"}
		for _, dt := range dataTypes {
			if strings.HasPrefix(dt, prefix) {
				items = append(items, types.CompletionProposal{
					Identifier:  dt,
					Description: "ABAP Data Type",
					Kind:        "type",
				})
			}
		}
	}

	// After INHERITING FROM: suggest class prefix
	if strings.Contains(upper, "INHERITING") && strings.Contains(upper, "FROM") {
		items = append(items, types.CompletionProposal{
			Identifier:  "CL_",
			Description: "Class prefix",
			Kind:        "class",
		})
	}

	// METHOD context: suggest parameter keywords
	if strings.Contains(upper, "METHODS") || strings.Contains(upper, "METHOD") {
		paramKW := []string{"IMPORTING", "EXPORTING", "CHANGING", "RETURNING"}
		for _, kw := range paramKW {
			if strings.HasPrefix(kw, prefix) {
				items = append(items, types.CompletionProposal{
					Identifier:  kw,
					Description: "Method Parameter",
					Kind:        "keyword",
				})
			}
		}
	}

	return items
}
