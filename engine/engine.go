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
// It orchestrates two single-purpose units: runners (which endpoints exist) and clients
// (one reused, ping-checked client per endpoint). Engine itself holds no lock — only a
// round-robin cursor — so the good path is obvious: discover, pick, get.
type Engine struct {
	cfg     *fxconfig.Source
	runners *runners
	clients *clients
	cursor  atomic.Uint64
}

func New(cfg *fxconfig.Source) *Engine {
	return &Engine{
		cfg:     cfg,
		runners: newRunners(cfg),
		clients: newClients(),
	}
}

// Close tears down every dialed engine connection. Call once at process/server shutdown.
func (e *Engine) Close() error { return e.clients.Close() }

// Client picks the next endpoint round-robin over the currently-discovered set and returns
// a live client for it. Build/Publish use it per unit; commands that need ad-hoc Dagger
// access (ls, preview) call it directly.
func (e *Engine) Client(ctx context.Context) (*dagger.Client, error) {
	hosts, err := e.resolveHosts(ctx)
	if err != nil {
		return nil, err
	}

	next := e.cursor.Add(1) - 1
	host := hosts[int(next%uint64(len(hosts)))]
	return e.clients.Get(ctx, host)
}

// Clean prunes the build cache of every engine in the fleet, forcing subsequent builds to
// run cold. It sheds stale or poisoned cache entries a fresh checkout would not carry.
func (e *Engine) Clean(ctx context.Context) error {
	hosts, err := e.resolveHosts(ctx)
	if err != nil {
		return err
	}

	for _, host := range hosts {
		client, err := e.clients.Get(ctx, host)
		if err != nil {
			return err
		}
		if err := client.Engine().LocalCache().Prune(ctx); err != nil {
			return err
		}
	}
	return nil
}

// resolveHosts returns the discovered engine endpoints, or a single empty host — meaning
// the local engine — when none are discovered.
func (e *Engine) resolveHosts(ctx context.Context) ([]string, error) {
	hosts, err := e.runners.Hosts(ctx)
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return []string{""}, nil
	}
	return hosts, nil
}

type engineContextKey struct{}

// NewContext returns a context carrying eng, so downstream Build/Publish resolve it via
// FromContext — the same shape as fx/data's request-scoped *sqlx.DB.
func NewContext(ctx context.Context, eng *Engine) context.Context {
	return context.WithValue(ctx, engineContextKey{}, eng)
}

// FromContext returns the engine carried by ctx, panicking if none is present. It is the
// Must-style counterpart to LookupFromContext; use it where the engine is a precondition.
func FromContext(ctx context.Context) *Engine {
	return ctx.Value(engineContextKey{}).(*Engine)
}

func LookupFromContext(ctx context.Context) (*Engine, bool) {
	eng, ok := ctx.Value(engineContextKey{}).(*Engine)
	return eng, ok
}
