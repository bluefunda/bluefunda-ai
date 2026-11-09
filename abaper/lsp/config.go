package lsp

import "github.com/bluefunda/bluefunda-ai/abaper/types"

// Config holds LSP server configuration
type Config struct {
	// SAP connection settings (reuse existing ADTConfig)
	ADT *types.ADTConfig

	// ActivateOnSave triggers object activation after save
	ActivateOnSave bool

	// OfflineOnly disables all live SAP features
	OfflineOnly bool

	// TCPAddress enables TCP transport instead of stdio (e.g. ":8089")
	TCPAddress string
}
