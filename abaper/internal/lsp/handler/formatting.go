package handler

import (
	"strings"

	"github.com/bluefunda/abaper/internal/lsp/backend"
	"github.com/bluefunda/abaper/internal/lsp/document"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Formatting handles textDocument/formatting requests.
func Formatting(ctx *glsp.Context, params *protocol.DocumentFormattingParams, docs *document.Manager, b backend.LSPBackend) ([]protocol.TextEdit, error) {
	uri := string(params.TextDocument.URI)
	doc, ok := docs.Get(uri)
	if !ok {
		return nil, nil
	}

	formatted, err := b.Format(doc.Content)
	if err != nil {
		return nil, nil
	}

	if formatted == doc.Content {
		return nil, nil
	}

	// Replace the entire document content
	lines := strings.Split(doc.Content, "\n")
	lastLine := len(lines) - 1
	lastChar := len(lines[lastLine])

	return []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: protocol.UInteger(lastLine), Character: protocol.UInteger(lastChar)},
			},
			NewText: formatted,
		},
	}, nil
}
