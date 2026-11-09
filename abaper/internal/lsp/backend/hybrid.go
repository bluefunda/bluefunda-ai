package backend

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/bluefunda/abaper/types"
)

// HybridBackend tries the ADT backend first, falling back to offline.
type HybridBackend struct {
	adt     *ADTBackend
	offline *OfflineBackend

	mu        sync.RWMutex
	connected bool
}

// NewHybridBackend creates a hybrid backend that prefers ADT but falls back to offline.
// The provided ctx controls the lifetime of the background connection monitor goroutine;
// cancel it (or use a context derived from a server shutdown context) to stop monitoring.
func NewHybridBackend(ctx context.Context, client types.ADTClient, workDir string) *HybridBackend {
	h := &HybridBackend{
		adt:       NewADTBackend(client),
		offline:   NewOfflineBackend(workDir),
		connected: client.IsAuthenticated(),
	}
	go h.connectionMonitor(ctx)
	return h
}

// SyntaxCheck tries ADT first, then falls back to offline
func (h *HybridBackend) SyntaxCheck(objectType, objectName, source string) (*types.SyntaxCheckResult, error) {
	if h.isConnected() {
		result, err := h.adt.SyntaxCheck(objectType, objectName, source)
		if err == nil {
			return result, nil
		}
		log.Printf("ADT syntax check failed, falling back to offline: %v", err)
		h.markDisconnected()
	}
	return h.offline.SyntaxCheck(objectType, objectName, source)
}

// Complete tries ADT first, then falls back to offline
func (h *HybridBackend) Complete(objectType, objectName, source string, line, column int) ([]types.CompletionProposal, error) {
	if h.isConnected() {
		proposals, err := h.adt.Complete(objectType, objectName, source, line, column)
		if err == nil {
			return proposals, nil
		}
		log.Printf("ADT completion failed, falling back to offline: %v", err)
		h.markDisconnected()
	}
	return h.offline.Complete(objectType, objectName, source, line, column)
}

// Navigate delegates to ADT only (offline cannot navigate)
func (h *HybridBackend) Navigate(objectType, objectName, source string, line, column int) (*types.NavigationTarget, error) {
	if h.isConnected() {
		return h.adt.Navigate(objectType, objectName, source, line, column)
	}
	return nil, nil
}

// Format always uses offline formatter
func (h *HybridBackend) Format(source string) (string, error) {
	return h.offline.Format(source)
}

// Activate delegates to ADT only
func (h *HybridBackend) Activate(objectType, objectName string) (*types.ActivationResult, error) {
	if h.isConnected() {
		return h.adt.Activate(objectType, objectName)
	}
	return h.offline.Activate(objectType, objectName)
}

// IsConnected returns the current connection status
func (h *HybridBackend) IsConnected() bool {
	return h.isConnected()
}

func (h *HybridBackend) isConnected() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connected
}

func (h *HybridBackend) markDisconnected() {
	h.mu.Lock()
	h.connected = false
	h.mu.Unlock()
}

// connectionMonitor periodically checks if the ADT connection is available.
// It exits when ctx is cancelled.
func (h *HybridBackend) connectionMonitor(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			wasConnected := h.isConnected()
			nowConnected := h.adt.IsConnected()
			h.mu.Lock()
			h.connected = nowConnected
			h.mu.Unlock()
			if !wasConnected && nowConnected {
				log.Println("ADT connection restored")
			} else if wasConnected && !nowConnected {
				log.Println("ADT connection lost, using offline mode")
			}
		}
	}
}
