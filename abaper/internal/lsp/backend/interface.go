package backend

import "github.com/bluefunda/abaper/types"

// LSPBackend defines the interface for LSP feature providers.
// Implementations include the offline backend (keyword completion, formatting,
// basic syntax validation) and the ADT backend (live SAP syntax check,
// completion proposals, navigation).
type LSPBackend interface {
	// SyntaxCheck validates ABAP source and returns diagnostics
	SyntaxCheck(objectType, objectName, source string) (*types.SyntaxCheckResult, error)

	// Complete returns completion proposals at the given position
	Complete(objectType, objectName, source string, line, column int) ([]types.CompletionProposal, error)

	// Navigate returns a go-to-definition target for the given position
	Navigate(objectType, objectName, source string, line, column int) (*types.NavigationTarget, error)

	// Format reformats ABAP source code
	Format(source string) (string, error)

	// Activate activates an ABAP object on the SAP system
	Activate(objectType, objectName string) (*types.ActivationResult, error)

	// IsConnected reports whether the backend has a live SAP connection
	IsConnected() bool
}
