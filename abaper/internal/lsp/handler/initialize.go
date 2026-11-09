package handler

import (
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Initialize handles the "initialize" request and returns server capabilities.
func Initialize(ctx *glsp.Context, params *protocol.InitializeParams, connected bool) (any, error) {
	syncKind := protocol.TextDocumentSyncKindFull
	includeText := true

	capabilities := protocol.ServerCapabilities{
		TextDocumentSync: &protocol.TextDocumentSyncOptions{
			OpenClose: boolPtr(true),
			Change:    &syncKind,
			Save: &protocol.SaveOptions{
				IncludeText: &includeText,
			},
		},
		CompletionProvider: &protocol.CompletionOptions{
			TriggerCharacters: []string{" ", ".", "-", ">"},
		},
		HoverProvider:              &protocol.HoverOptions{},
		DefinitionProvider:         &protocol.DefinitionOptions{},
		DocumentFormattingProvider: &protocol.DocumentFormattingOptions{},
	}

	serverInfo := &protocol.InitializeResultServerInfo{
		Name:    "abaper-lsp",
		Version: strPtr("0.1.0"),
	}

	result := protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo:   serverInfo,
	}

	// Notify the client about connection status
	mode := "offline"
	if connected {
		mode = "connected to SAP"
	}
	go func() {
		ctx.Notify(protocol.ServerWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeInfo,
			Message: "ABAP Language Server started (" + mode + ")",
		})
	}()

	return result, nil
}

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
