package handler

import (
	"github.com/bluefunda/abaper/internal/lsp/backend"
	"github.com/bluefunda/abaper/internal/lsp/document"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Completion handles textDocument/completion requests.
func Completion(ctx *glsp.Context, params *protocol.CompletionParams, docs *document.Manager, b backend.LSPBackend) (any, error) {
	uri := string(params.TextDocument.URI)
	doc, ok := docs.Get(uri)
	if !ok {
		return nil, nil
	}

	proposals, err := b.Complete(
		doc.ObjectType, doc.ObjectName, doc.Content,
		int(params.Position.Line), int(params.Position.Character),
	)
	if err != nil {
		return nil, nil
	}

	var items []protocol.CompletionItem
	for _, p := range proposals {
		kind := mapCompletionKind(p.Kind)
		detail := p.Description
		item := protocol.CompletionItem{
			Label:  p.Identifier,
			Kind:   &kind,
			Detail: &detail,
		}
		if p.InsertText != "" {
			item.InsertText = &p.InsertText
		}
		items = append(items, item)
	}

	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil
}

func mapCompletionKind(kind string) protocol.CompletionItemKind {
	switch kind {
	case "keyword":
		return protocol.CompletionItemKindKeyword
	case "function":
		return protocol.CompletionItemKindFunction
	case "variable":
		return protocol.CompletionItemKindVariable
	case "class":
		return protocol.CompletionItemKindClass
	case "type":
		return protocol.CompletionItemKindTypeParameter
	default:
		return protocol.CompletionItemKindKeyword
	}
}
