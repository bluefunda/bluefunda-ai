package handler

import (
	"fmt"
	"strings"

	"github.com/bluefunda/abaper/internal/lsp/abap"
	"github.com/bluefunda/abaper/internal/lsp/document"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Hover handles textDocument/hover requests.
func Hover(ctx *glsp.Context, params *protocol.HoverParams, docs *document.Manager) (*protocol.Hover, error) {
	uri := string(params.TextDocument.URI)
	doc, ok := docs.Get(uri)
	if !ok {
		return nil, nil
	}

	lines := strings.Split(doc.Content, "\n")
	line := int(params.Position.Line)
	character := int(params.Position.Character)
	if line >= len(lines) {
		return nil, nil
	}

	word := abap.GetWordAtPosition(lines[line], character)
	if word == "" {
		return nil, nil
	}

	upper := strings.ToUpper(word)

	// Check keyword documentation
	if doc, ok := abap.KeywordDocumentation[upper]; ok {
		return makeHover(fmt.Sprintf("**%s**\n\n%s", upper, doc)), nil
	}

	// Check function documentation
	lower := strings.ToLower(word)
	if doc, ok := abap.FunctionDocumentation[lower]; ok {
		return makeHover(fmt.Sprintf("**%s**\n\n%s", lower, doc)), nil
	}

	// Check system field documentation
	if strings.HasPrefix(upper, "SY-") {
		if doc, ok := abap.SystemFieldDocumentation[upper]; ok {
			return makeHover(fmt.Sprintf("**%s**\n\n%s", upper, doc)), nil
		}
		return makeHover(fmt.Sprintf("**%s**\n\nABAP system field", upper)), nil
	}

	return nil, nil
}

func makeHover(content string) *protocol.Hover {
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: content,
		},
	}
}
