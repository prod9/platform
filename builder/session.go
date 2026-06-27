package builder

import (
	"context"
	"fmt"

	"dagger.io/dagger"
	"platform.prodigy9.co/internal/buildlog"
)

// Session holds the Dagger engine pool for one build run. In-cluster it is one client per
// engine pod (round-robined per job, see forEngine); locally it is a single auto-provisioned
// engine. A per-job view (bound != nil) shares the pool but owns nothing.
type Session struct {
	ctx     context.Context
	clients []*dagger.Client
	bound   *dagger.Client
}

func NewSession(ctx context.Context) (*Session, error) {
	hosts := discoverEngines()
	if len(hosts) == 0 {
		client, err := dagger.Connect(ctx, dagger.WithLogOutput(buildlog.OutputForDagger()))
		if err != nil {
			return nil, err
		}
		return &Session{ctx: ctx, clients: []*dagger.Client{client}}, nil
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
	return &Session{ctx: ctx, clients: clients}, nil
}

// forEngine returns a per-job view bound to engine idx%n — every operation a single job
// issues must reach one engine (a Dagger session spans multiple connections; splitting it
// across engines breaks it), so selection is per job and fixed for the job's lifetime.
func (s *Session) forEngine(idx int) *Session {
	return &Session{ctx: s.ctx, clients: s.clients, bound: s.clients[idx%len(s.clients)]}
}

func (s *Session) Client() *dagger.Client {
	if s.bound != nil {
		return s.bound
	}
	return s.clients[0]
}

func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) JobContext(unit *BuildUnit) (context.Context, context.CancelFunc) {
	return context.WithTimeout(s.ctx, unit.Timeout)
}

// Close shuts down every engine connection in the pool. A per-job view owns nothing, so
// closing one is a no-op.
func (s *Session) Close() error {
	if s.bound != nil {
		return nil
	}

	var err error
	for _, client := range s.clients {
		if cerr := client.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}
