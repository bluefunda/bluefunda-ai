package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bluefunda/abaper/internal/adt"
	"github.com/bluefunda/abaper/types"
	"go.uber.org/zap"
)

const defaultPoolTTL = 30 * time.Minute

// poolKey uniquely identifies an ADT connection (password excluded intentionally).
func poolKey(host, client, username string) string {
	return host + "|" + client + "|" + username
}

type poolEntry struct {
	client   types.ADTClient
	lastUsed time.Time
}

// Pool maintains a set of authenticated ADT clients, one per unique
// host+client+username tuple, with idle-based eviction.
type Pool struct {
	mu      sync.Mutex
	entries map[string]*poolEntry
	ttl     time.Duration
	logger  *zap.Logger
}

// NewPool creates a Pool with the given idle TTL.
func NewPool(ttl time.Duration, logger *zap.Logger) *Pool {
	if ttl <= 0 {
		ttl = defaultPoolTTL
	}
	return &Pool{
		entries: make(map[string]*poolEntry),
		ttl:     ttl,
		logger:  logger.With(zap.String("component", "adt_pool")),
	}
}

// Get returns an authenticated ADT client for cfg, creating one if not pooled.
// On authentication failure the entry is removed so the next call retries.
func (p *Pool) Get(ctx context.Context, cfg types.ADTConfig) (types.ADTClient, error) {
	key := poolKey(cfg.Host, cfg.Client, cfg.Username)

	p.mu.Lock()
	e, ok := p.entries[key]
	if ok {
		e.lastUsed = time.Now()
		p.mu.Unlock()
		if e.client.IsAuthenticated() {
			return e.client, nil
		}
		// Session expired — re-authenticate in place.
		if err := e.client.Authenticate(); err != nil {
			p.mu.Lock()
			delete(p.entries, key)
			p.mu.Unlock()
			return nil, fmt.Errorf("re-authenticate %s: %w", cfg.Host, err)
		}
		return e.client, nil
	}
	p.mu.Unlock()

	// Create and authenticate a new client outside the lock.
	c := adt.NewADTClient(&cfg)
	if err := c.Authenticate(); err != nil {
		return nil, fmt.Errorf("authenticate %s: %w", cfg.Host, err)
	}

	p.mu.Lock()
	p.entries[key] = &poolEntry{client: c, lastUsed: time.Now()}
	p.mu.Unlock()

	p.logger.Info("ADT client added to pool",
		zap.String("host", cfg.Host),
		zap.String("sap_client", cfg.Client),
		zap.String("username", cfg.Username))
	return c, nil
}

// EvictIdle removes pool entries that have not been used within the TTL.
func (p *Pool) EvictIdle() {
	cutoff := time.Now().Add(-p.ttl)

	p.mu.Lock()
	defer p.mu.Unlock()

	for key, e := range p.entries {
		if e.lastUsed.Before(cutoff) {
			delete(p.entries, key)
			p.logger.Info("Evicted idle ADT connection", zap.String("key", key))
		}
	}
}

// StartEviction runs EvictIdle on the given interval until ctx is cancelled.
func (p *Pool) StartEviction(ctx context.Context, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				p.EvictIdle()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Size returns the number of connections currently in the pool.
func (p *Pool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}
