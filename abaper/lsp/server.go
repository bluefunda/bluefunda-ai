// Package lsp provides an ABAP Language Server Protocol (LSP) server.
// It supports stdio and TCP transports, and three backend modes:
// offline (keyword completion + abaplint), ADT (live SAP connection), and hybrid.
package lsp

import (
	"github.com/bluefunda/abaper/internal/lsp/backend"
	"github.com/bluefunda/abaper/internal/lsp/document"
	"github.com/bluefunda/abaper/internal/lsp/handler"
	"github.com/tliron/commonlog"
	_ "github.com/tliron/commonlog/simple"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	glspserver "github.com/tliron/glsp/server"
)

const serverName = "abaper-lsp"

// Server wraps the GLSP server with abaper-specific components
type Server struct {
	config  *Config
	backend backend.LSPBackend
	docs    *document.Manager
	server  *glspserver.Server
}

// NewServer creates a new LSP server instance
func NewServer(cfg *Config, b backend.LSPBackend) *Server {
	s := &Server{
		config:  cfg,
		backend: b,
		docs:    document.NewManager(),
	}

	h := s.buildHandler()
	commonlog.Configure(1, nil) // minimal logging
	s.server = glspserver.NewServer(h, serverName, false)

	return s
}

// RunStdio starts the LSP server over stdin/stdout
func (s *Server) RunStdio() error {
	return s.server.RunStdio()
}

// RunTCP starts the LSP server listening on a TCP address
func (s *Server) RunTCP(address string) error {
	return s.server.RunTCP(address)
}

func (s *Server) buildHandler() *protocol.Handler {
	return &protocol.Handler{
		Initialize:  s.handleInitialize,
		Initialized: s.handleInitialized,
		Shutdown:    s.handleShutdown,
		Exit:        s.handleExit,

		TextDocumentDidOpen:   s.handleDidOpen,
		TextDocumentDidChange: s.handleDidChange,
		TextDocumentDidSave:   s.handleDidSave,
		TextDocumentDidClose:  s.handleDidClose,

		TextDocumentCompletion: s.handleCompletion,
		TextDocumentHover:      s.handleHover,
		TextDocumentDefinition: s.handleDefinition,
		TextDocumentFormatting: s.handleFormatting,
	}
}

// --- Initialize / Lifecycle ---

func (s *Server) handleInitialize(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
	return handler.Initialize(ctx, params, s.backend.IsConnected())
}

func (s *Server) handleInitialized(ctx *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func (s *Server) handleShutdown(ctx *glsp.Context) error {
	return nil
}

func (s *Server) handleExit(ctx *glsp.Context) error {
	return nil
}

// --- Document Sync ---

func (s *Server) handleDidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	handler.DidOpen(ctx, params, s.docs, s.backend)
	return nil
}

func (s *Server) handleDidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	handler.DidChange(ctx, params, s.docs, s.backend)
	return nil
}

func (s *Server) handleDidSave(ctx *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	handler.DidSave(ctx, params, s.docs, s.backend, s.config.ActivateOnSave)
	return nil
}

func (s *Server) handleDidClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	handler.DidClose(ctx, params, s.docs)
	return nil
}

// --- Language Features ---

func (s *Server) handleCompletion(ctx *glsp.Context, params *protocol.CompletionParams) (any, error) {
	return handler.Completion(ctx, params, s.docs, s.backend)
}

func (s *Server) handleHover(ctx *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	return handler.Hover(ctx, params, s.docs)
}

func (s *Server) handleDefinition(ctx *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	return handler.Definition(ctx, params, s.docs, s.backend)
}

func (s *Server) handleFormatting(ctx *glsp.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	return handler.Formatting(ctx, params, s.docs, s.backend)
}
