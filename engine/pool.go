package engine

import (
	"context"
	"fmt"
	"time"

	"dagger.io/dagger"
	"platform.prodigy9.co/internal/buildlog"
)

// Pool holds the Dagger engine connections for one build run. In-cluster it is one client
// per engine pod (round-robined per unit, see bind); locally it is a single auto-provisioned
// engine. A per-unit view (bound != nil) shares the pool but owns nothing.
type Pool struct {
	ctx     context.Context
	clients []*dagger.Client
	bound   *dagger.Client
}

func New(ctx context.Context) (*Pool, error) {
	hosts := discoverEngines()
	if len(hosts) == 0 {
		client, err := dagger.Connect(ctx, dagger.WithLogOutput(buildlog.OutputForDagger()))
		if err != nil {
			return nil, err
		}
		return &Pool{ctx: ctx, clients: []*dagger.Client{client}}, nil
	}

	clients := make([]*dagger.Client, 0, len(hosts))
	for _, host := range hosts {
		client, err := dagger.Connect(ctx,
			dagger.WithRunnerHost(host),
			dagger.WithLogOutput(buildlog.OutputForDagger()))
		if err != nil {
			return nil, fmt.Errorf("connecting to engine %s: %w", host, err)
		}
		clients = append(clients, client)
	}
	return &Pool{ctx: ctx, clients: clients}, nil
}

// bind returns a per-unit view bound to engine idx%n — every operation a single unit
// issues must reach one engine (a Dagger session spans multiple connections; splitting it
// across engines breaks it), so selection is per unit and fixed for the unit's lifetime.
func (p *Pool) bind(idx int) *Pool {
	return &Pool{ctx: p.ctx, clients: p.clients, bound: p.clients[idx%len(p.clients)]}
}

func (p *Pool) Client() *dagger.Client {
	if p.bound != nil {
		return p.bound
	}
	return p.clients[0]
}

func (p *Pool) Context() context.Context {
	return p.ctx
}

// unitContext derives a per-unit context carrying the unit's build timeout. It takes a bare
// duration rather than a BuildUnit so the pool stays free of the build model.
func (p *Pool) unitContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(p.ctx, timeout)
}

// Close shuts down every engine connection in the pool. A per-unit view owns nothing, so
// closing one is a no-op.
func (p *Pool) Close() error {
	if p.bound != nil {
		return nil
	}

	var err error
	for _, client := range p.clients {
		if cerr := client.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}
