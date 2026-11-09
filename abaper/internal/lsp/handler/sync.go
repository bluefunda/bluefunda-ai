package handler

import (
	"github.com/bluefunda/abaper/internal/lsp/backend"
	"github.com/bluefunda/abaper/internal/lsp/document"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// DidOpen handles textDocument/didOpen — stores the document and triggers diagnostics.
func DidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams, docs *document.Manager, b backend.LSPBackend) {
	uri := params.TextDocument.URI
	doc := docs.Open(
		uri,
		params.TextDocument.LanguageID,
		params.TextDocument.Text,
		int(params.TextDocument.Version),
	)
	if doc != nil {
		go publishDiagnostics(ctx, doc, b, false)
	}
}

// DidChange handles textDocument/didChange — updates content and triggers diagnostics.
func DidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams, docs *document.Manager, b backend.LSPBackend) {
	uri := string(params.TextDocument.URI)
	// With TextDocumentSyncKindFull, the last content change has the full text
	var newContent string
	for _, change := range params.ContentChanges {
		switch c := change.(type) {
		case protocol.TextDocumentContentChangeEventWhole:
			newContent = c.Text
		case protocol.TextDocumentContentChangeEvent:
			// For incremental changes we'd need to apply the diff,
			// but with Full sync we always get the whole text
			newContent = c.Text
		}
	}
	if newContent != "" {
		doc := docs.Update(uri, newContent, int(params.TextDocument.Version))
		if doc != nil {
			go publishDiagnostics(ctx, doc, b, false)
		}
	}
}

// DidSave handles textDocument/didSave — marks saved, runs full diagnostics.
func DidSave(ctx *glsp.Context, params *protocol.DidSaveTextDocumentParams, docs *document.Manager, b backend.LSPBackend, activateOnSave bool) {
	uri := string(params.TextDocument.URI)
	text := ""
	if params.Text != nil {
		text = *params.Text
	}
	doc := docs.Save(uri, text)
	if doc != nil {
		go publishDiagnostics(ctx, doc, b, true)
		if activateOnSave {
			go runActivation(ctx, doc, b)
		}
	}
}

// DidClose handles textDocument/didClose — removes document and clears diagnostics.
func DidClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams, docs *document.Manager) {
	uri := string(params.TextDocument.URI)
	docs.Close(uri)
	// Clear diagnostics for closed document
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
		URI:         protocol.DocumentUri(uri),
		Diagnostics: []protocol.Diagnostic{},
	})
}
