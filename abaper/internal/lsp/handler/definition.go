package handler

import (
	"github.com/bluefunda/abaper/internal/lsp/backend"
	"github.com/bluefunda/abaper/internal/lsp/document"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Definition handles textDocument/definition requests.
func Definition(ctx *glsp.Context, params *protocol.DefinitionParams, docs *document.Manager, b backend.LSPBackend) (any, error) {
	uri := string(params.TextDocument.URI)
	doc, ok := docs.Get(uri)
	if !ok {
		return nil, nil
	}

	target, err := b.Navigate(
		doc.ObjectType, doc.ObjectName, doc.Content,
		int(params.Position.Line), int(params.Position.Character),
	)
	if err != nil || target == nil {
		return nil, nil
	}

	return &protocol.Location{
		URI: protocol.DocumentUri(target.URI),
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      protocol.UInteger(target.Line),
				Character: protocol.UInteger(target.Column),
			},
			End: protocol.Position{
				Line:      protocol.UInteger(target.Line),
				Character: protocol.UInteger(target.Column),
			},
		},
	}, nil
}
