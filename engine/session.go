package engine

import (
	"context"
	"errors"
	"sync"

	"dagger.io/dagger"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/framework"
)

// Session is the span during which containers are usable. Every container is a handle into
// the connection that built it, and every connection is dialed into the session's own
// context — so a container stays valid for exactly as long as its session and dies with
// Close. Nothing else ends a session: no caller's cancellation reaches one, because a
// per-unit timeout that killed a connection would invalidate containers the caller is still
// holding.
//
// A session opens as many connections as its work needs and closes them together. It is safe
// for concurrent use, and opening one dials nothing, so a command opens exactly one and
// defers Close.
type Session struct {
	ctx context.Context // carries both the lifetime and the config

	mu    sync.Mutex
	conns []*dagger.Client
}

// NewSession opens a session on the fleet. It dials nothing: connections are made as runs
// need them, so a session that never builds never touches an engine. ctx is the lifetime
// every connection will be dialed into — a process scope, never a request or per-unit one —
// and it carries the config the roster reads.
func NewSession(ctx context.Context) *Session {
	return &Session{ctx: ctx}
}

// Close ends every connection this session opened, and with them every container built on
// one. Call it once, deferred, by whoever opened the session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for _, conn := range s.conns {
		errs = append(errs, conn.Close())
	}

	s.conns = nil
	return errors.Join(errs...)
}

// Unsafe hands over a raw connection, and the name is the warning: past here a caller
// expresses Dagger operations the session otherwise owns. ls's ad-hoc container reaches
// through it because it never builds; no new caller joins it.
func (s *Session) Unsafe() (*dagger.Client, error) { return s.connect() }

// Build builds every module matched by modnames (all of them when it is empty), one run per
// unit, each on its own connection. It constructs the units itself — resolving the arch from
// cfg — so callers never name a platform; what they need afterwards they read off
// BuildResult.Unit.
func (s *Session) Build(ctx context.Context, cfg *conf.Model, modnames []string, obs Observer) ([]BuildResult, error) {
	units, err := framework.Units(cfg, modnames, s.buildArch(cfg))
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		return nil, ErrNoJobs
	}

	m := &multiplexer[*framework.BuildUnit, BuildResult]{}
	m.Reset(units)
	return m.Start(func(idx int, unit *framework.BuildUnit) BuildResult {
		return s.runUnit(ctx, unit, obs).Result()
	}), nil
}

// BuildAndPublish builds every matched module at the publish arch and pushes each image as
// its own run finishes. Publishing is a bracket around the run rather than a second pass
// over results: the connection that built the container is still in hand, so the registry
// secret is minted by the session that owns the container it authenticates.
func (s *Session) BuildAndPublish(ctx context.Context, cfg *conf.Model, modnames []string, tag string, obs Observer) ([]PublishResult, error) {
	units, err := framework.Units(cfg, modnames, cfg.PublishArch)
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		return nil, ErrNoJobs
	}

	for _, unit := range units {
		unit.ImageName = unit.ImageName + ":" + tag
	}
	creds := registryCredsFrom(cfgFrom(s.ctx))

	m := &multiplexer[*framework.BuildUnit, PublishResult]{}
	m.Reset(units)
	results := m.Start(func(idx int, unit *framework.BuildUnit) PublishResult {
		return s.runUnit(ctx, unit, obs).publish(ctx, creds)
	})

	var errs []error
	for _, result := range results {
		if result.Err != nil {
			errs = append(errs, result.Err)
		}
	}
	return results, errors.Join(errs...)
}

// Clean prunes the build cache of every engine in the fleet, forcing later builds to run
// cold. It sheds stale or poisoned cache entries a fresh checkout would not carry, so it
// dials each engine by name rather than picking one.
func (s *Session) Clean(ctx context.Context) error {
	hosts, err := Hosts(s.ctx)
	if err != nil {
		return err
	}
	if len(hosts) == 0 {
		hosts = []string{""} // the local engine is the whole fleet
	}

	for _, host := range hosts {
		client, err := dial(s.ctx, host)
		if err != nil {
			return err
		}
		s.keep(client)

		if err := client.Engine().LocalCache().Prune(ctx); err != nil {
			return err
		}
	}
	return nil
}

// runUnit drives one unit's whole plan and hands back the finished run, which still holds
// the connection its container is bound to. The timeout bounds the unit's steps and nothing
// else — it is never the context a connection is dialed into, so a unit ending cannot close
// a session whose containers the caller still holds.
func (s *Session) runUnit(ctx context.Context, unit *framework.BuildUnit, obs Observer) *Run {
	unitCtx, cancel := context.WithTimeout(ctx, unit.Timeout)
	defer cancel()

	run := NewRun(s, unit, obs)
	for run.Next(unitCtx) {
	}
	return run
}

// connect adds one connection to the session and hands it over. A run calls this once and
// reuses what it gets, because a container is bound to the connection that built it; a
// session driving many runs calls it many times, and that is what spreads runs across the
// fleet.
func (s *Session) connect() (*dagger.Client, error) {
	client, err := Dial(s.ctx)
	if err != nil {
		return nil, err
	}

	s.keep(client)
	return client, nil
}

// keep puts a connection under the session's ownership, so Close ends it.
func (s *Session) keep(client *dagger.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conns = append(s.conns, client)
}
