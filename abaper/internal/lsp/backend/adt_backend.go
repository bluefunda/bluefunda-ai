package backend

import (
	"context"

	"github.com/bluefunda/abaper/internal/lsp/abap"
	"github.com/bluefunda/abaper/types"
)

// adtLSPClient is the minimal interface the ADTBackend depends on.
type adtLSPClient interface {
	types.LangFeatures
	types.ObjectActivator
	IsAuthenticated() bool
}

// ADTBackend provides LSP features via a live SAP ADT connection.
type ADTBackend struct {
	client adtLSPClient
}

// NewADTBackend creates a new ADT backend wrapping an existing ADTClient
func NewADTBackend(client types.ADTClient) *ADTBackend {
	return &ADTBackend{client: client}
}

// SyntaxCheck delegates to the ADT client's SyntaxCheck endpoint
func (b *ADTBackend) SyntaxCheck(objectType, objectName, source string) (*types.SyntaxCheckResult, error) {
	return b.client.SyntaxCheck(context.Background(), objectType, objectName, source)
}

// Complete delegates to the ADT client's GetCompletionProposals endpoint
func (b *ADTBackend) Complete(objectType, objectName, source string, line, column int) ([]types.CompletionProposal, error) {
	return b.client.GetCompletionProposals(context.Background(), objectType, objectName, source, line, column)
}

// Navigate delegates to the ADT client's GetNavigationTarget endpoint
func (b *ADTBackend) Navigate(objectType, objectName, source string, line, column int) (*types.NavigationTarget, error) {
	return b.client.GetNavigationTarget(context.Background(), objectType, objectName, source, line, column)
}

// Format uses the offline formatter (ADT doesn't have a formatting endpoint)
func (b *ADTBackend) Format(source string) (string, error) {
	return abap.Format(source)
}

// Activate delegates to the ADT client's ActivateObject method
func (b *ADTBackend) Activate(objectType, objectName string) (*types.ActivationResult, error) {
	return b.client.ActivateObject(context.Background(), objectType, objectName)
}

// IsConnected checks if the ADT client is authenticated
func (b *ADTBackend) IsConnected() bool {
	return b.client.IsAuthenticated()
}
