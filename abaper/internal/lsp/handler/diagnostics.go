package handler

import (
	"github.com/bluefunda/abaper/internal/lsp/backend"
	"github.com/bluefunda/abaper/internal/lsp/document"
	"github.com/bluefunda/abaper/types"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// publishDiagnostics runs syntax check and publishes LSP diagnostics.
func publishDiagnostics(ctx *glsp.Context, doc *document.Document, b backend.LSPBackend, _ bool) {
	// Take a snapshot of document content to avoid races
	content := doc.Content
	objectType := doc.ObjectType
	objectName := doc.ObjectName
	uri := doc.URI

	// Run backend syntax check (abaplint for offline, ADT for connected)
	var msgs []types.SyntaxCheckMessage
	result, err := b.SyntaxCheck(objectType, objectName, content)
	if err == nil && result != nil {
		msgs = result.Messages
	}

	var diagnostics []protocol.Diagnostic
	for _, m := range msgs {
		sev := mapSeverity(m.Severity)
		source := diagnosticSource(b)
		diag := protocol.Diagnostic{
			Range:    makeRange(m),
			Severity: &sev,
			Source:   &source,
			Message:  m.Text,
		}
		if m.Code != "" {
			code := protocol.IntegerOrString{Value: m.Code}
			diag.Code = &code
		}
		diagnostics = append(diagnostics, diag)
	}

	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
		URI:         protocol.DocumentUri(uri),
		Diagnostics: diagnostics,
	})
}

// makeRange creates an LSP Range from a SyntaxCheckMessage.
// Uses end position when available for proper underline width.
func makeRange(m types.SyntaxCheckMessage) protocol.Range {
	startLine := clampUint(m.Line)
	startCol := clampUint(m.Column)
	endLine := clampUint(m.EndLine)
	endCol := clampUint(m.EndCol)

	// If end position is zero/same as start, extend to end of token
	if endLine == startLine && endCol <= startCol {
		endCol = startCol + 1
	}

	return protocol.Range{
		Start: protocol.Position{Line: startLine, Character: startCol},
		End:   protocol.Position{Line: endLine, Character: endCol},
	}
}

func clampUint(v int) protocol.UInteger {
	if v < 0 {
		return 0
	}
	return protocol.UInteger(v)
}

func diagnosticSource(b backend.LSPBackend) string {
	if b.IsConnected() {
		return "abaper-adt"
	}
	return "abaplint"
}

// runActivation activates an object and publishes activation messages as diagnostics.
func runActivation(ctx *glsp.Context, doc *document.Document, b backend.LSPBackend) {
	result, err := b.Activate(doc.ObjectType, doc.ObjectName)
	if err != nil {
		ctx.Notify(protocol.ServerWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeError,
			Message: "Activation failed: " + err.Error(),
		})
		return
	}
	if result != nil && !result.Success {
		var diagnostics []protocol.Diagnostic
		for _, m := range result.Messages {
			sev := mapSeverity(m.Severity)
			source := "abaper-activation"
			diagnostics = append(diagnostics, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: protocol.UInteger(m.Line), Character: 0},
					End:   protocol.Position{Line: protocol.UInteger(m.Line), Character: 0},
				},
				Severity: &sev,
				Source:   &source,
				Message:  m.Text,
			})
		}
		ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
			URI:         protocol.DocumentUri(doc.URI),
			Diagnostics: diagnostics,
		})
	}
}

func mapSeverity(s string) protocol.DiagnosticSeverity {
	switch s {
	case "error":
		return protocol.DiagnosticSeverityError
	case "warning":
		return protocol.DiagnosticSeverityWarning
	case "info":
		return protocol.DiagnosticSeverityInformation
	case "hint":
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityWarning
	}
}
