package engine

import (
	"context"
	"sync/atomic"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
)

// Engine is the process-wide handle to the Dagger engine fleet — like sql.DB, a
// concurrency-safe set of connections dialed lazily and reused. Build one from config,
// share it across the process, and carry it on a context with NewContext.
//
// It orchestrates two single-purpose units: discovery (which endpoints exist) and clients
// (one reused, ping-checked client per endpoint). Engine itself holds no lock — only a
// round-robin cursor — so the good path is obvious: discover, pick, get.
type Engine struct {
	cfg       *fxconfig.Source
	discovery *discovery
	clients   *clients
	cursor    atomic.Uint64
}

func New(cfg *fxconfig.Source) *Engine {
	return &Engine{
		cfg:       cfg,
		discovery: newDiscovery(cfg),
		clients:   newClients(),
	}
}

// Close tears down every dialed engine connection. Call once at process/server shutdown.
func (e *Engine) Close() error { return e.clients.Close() }

// Client picks the next endpoint round-robin over the currently-discovered set and returns
// a live client for it. Build/Publish use it per unit; commands that need ad-hoc Dagger
// access (ls, preview) call it directly.
func (e *Engine) Client(ctx context.Context) (*dagger.Client, error) {
	hosts, err := e.discovery.Hosts(ctx)
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		// no remote engines discovered — spawn/reuse the local one.
		return e.clients.Get(ctx, localHost)
	}

	next := e.cursor.Add(1) - 1
	host := hosts[int(next%uint64(len(hosts)))]
	return e.clients.Get(ctx, host)
}

type engineContextKey struct{}

// NewContext returns a context carrying eng, so downstream Build/Publish resolve it via
// FromContext — the same shape as fx/data's request-scoped *sqlx.DB.
func NewContext(ctx context.Context, eng *Engine) context.Context {
	return context.WithValue(ctx, engineContextKey{}, eng)
}

func FromContext(ctx context.Context) *Engine {
	return ctx.Value(engineContextKey{}).(*Engine)
}

func LookupFromContext(ctx context.Context) (*Engine, bool) {
	eng, ok := ctx.Value(engineContextKey{}).(*Engine)
	return eng, ok
}
