package engine

import (
	"context"
	"sync"

	"dagger.io/dagger"
	"platform.prodigy9.co/internal/buildlog"
)

// clients is the connection cache: one reused *dagger.Client per engine endpoint. It knows
// nothing about discovery. Get validates a cached client with a cheap Version() ping and
// redials when the engine has gone (graceful DNS removal or an outright crash), so callers
// always receive a live client — there is no separate prune step and nothing is ever closed
// mid-build. The lock is held only around map reads/writes, never across a dial or ping.
type clients struct {
	dial func(ctx context.Context, host string) (*dagger.Client, error)

	mu   sync.Mutex
	pool map[string]*dagger.Client
}

func newClients() *clients {
	return &clients{dial: dialEngine, pool: map[string]*dagger.Client{}}
}

// dialEngine connects to the engine at host. An empty host carries no runner host, so dagger
// auto-provisions and reuses the local engine — that is how the core asks for "local".
func dialEngine(ctx context.Context, host string) (*dagger.Client, error) {
	opts := []dagger.ClientOpt{dagger.WithLogOutput(buildlog.OutputForDagger())}
	if host != "" {
		opts = append(opts, dagger.WithRunnerHost(host))
	}
	return dagger.Connect(ctx, opts...)
}

// Get returns a live client for host, reusing a cached one when it still answers a ping and
// dialing a fresh one otherwise.
func (c *clients) Get(ctx context.Context, host string) (*dagger.Client, error) {
	c.mu.Lock()
	cached := c.pool[host]
	c.mu.Unlock()

	if cached != nil {
		if _, err := cached.Version(ctx); err == nil {
			return cached, nil
		}
		// dead — drop it and fall through to redial.
		c.mu.Lock()
		if c.pool[host] == cached {
			delete(c.pool, host)
			_ = cached.Close()
		}
		c.mu.Unlock()
	}

	client, err := c.dial(ctx, host)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.pool[host]; existing != nil {
		// lost a concurrent dial race — keep the winner, close ours.
		_ = client.Close()
		return existing, nil
	}
	c.pool[host] = client
	return client, nil
}

// Close shuts down every cached client. Used at process/server shutdown; liveness during a
// run is handled by Get's ping, not by closing.
func (c *clients) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	for host, client := range c.pool {
		if cerr := client.Close(); cerr != nil && err == nil {
			err = cerr
		}
		delete(c.pool, host)
	}
	return err
}
