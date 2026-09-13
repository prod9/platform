package engine

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"time"

	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine/observer"
	"platform.prodigy9.co/framework"
)

type buildIntent interface {
	arch(*Session, *conf.Model) string
	prepare(*framework.BuildUnit) error
	finish(context.Context, *Run) (string, error)
}
type buildOnly struct{}

type moduleOperation struct {
	session *Session
	input   Input
	name    string
	intent  buildIntent
	caller  observer.Observer
}
type operationResult struct {
	results []BuildResult
	err     error
}

func (o *moduleOperation) run(ctx context.Context) operationResult {
	obs, out := observer.Accumulate(o.caller)
	obs.RunStart(o.name, time.Now())

	cursor, err := o.prepare(ctx, obs)
	image, hash := "", ""
	if err == nil {
		stepCtx, cancel := context.WithTimeout(ctx, cursor.unit.Timeout)
		for cursor.Next(stepCtx) {
		}
		cancel()
		err = cursor.err
		if err == nil {
			image = cursor.unit.ImageName
			hash, err = o.intent.finish(ctx, cursor)
		}
	}

	obs.RunDone(o.name, image, hash, time.Now(), err)
	if cursor == nil {
		return operationResult{err: out.Err}
	}

	container := cursor.container
	if out.Err != nil {
		container = nil
	}
	result := BuildResult{Unit: cursor.unit, Err: out.Err, container: container, out: out}
	return operationResult{results: []BuildResult{result}, err: out.Err}
}

func (o *moduleOperation) prepare(ctx context.Context, obs observer.Observer) (*Run, error) {
	path, err := o.configPath(ctx, obs)
	if err != nil {
		return nil, err
	}

	obs.ConfigStart(o.name, time.Now())
	cursor, host, err := o.configure(path, obs)
	obs.ConfigDone(o.name, host, time.Now(), err)
	return cursor, err
}

func (o *moduleOperation) configPath(ctx context.Context, obs observer.Observer) (string, error) {
	switch input := o.input.(type) {
	case Local:
		return input.ConfigPath, nil
	case Source:
		obs.CloneStart(o.name, time.Now())
		checkoutCtx := fxconfig.NewContext(ctx, cfgFrom(o.session.ctx))
		worktree, err := Checkout(checkoutCtx, input)
		obs.CloneDone(o.name, time.Now(), err)
		if err != nil {
			return "", err
		}
		o.session.mu.Lock()
		o.session.worktrees = append(o.session.worktrees, worktree)
		o.session.mu.Unlock()
		return filepath.Join(worktree.Dir, "platform.toml"), nil
	default:
		return "", errors.New("engine: invalid build input")
	}
}

func (o *moduleOperation) configure(path string, obs observer.Observer) (*Run, string, error) {
	cfg, err := conf.LoadFile(path)
	if err != nil {
		return nil, "", err
	}

	units, err := framework.Units(cfg, []string{o.name}, o.intent.arch(o.session, cfg))
	if err != nil {
		return nil, "", err
	}
	unit := units[0]
	if err := o.intent.prepare(unit); err != nil {
		return nil, "", err
	}
	steps := unit.Framework.Plan(unit)
	if len(steps) == 0 {
		return nil, "", ErrEmptyPlan
	}

	client, host, err := o.session.connect()
	if err != nil {
		return nil, "", err
	}
	return &Run{unit: unit, obs: obs, steps: steps, client: client}, host, nil
}

func (buildOnly) arch(s *Session, cfg *conf.Model) string { return s.buildArch(cfg) }

func (buildOnly) prepare(*framework.BuildUnit) error { return nil }

func (buildOnly) finish(context.Context, *Run) (string, error) { return "", nil }

func selectedNames(input Input, names []string) ([]string, error) {
	switch source := input.(type) {
	case Local:
		if len(names) == 0 {
			cfg, err := conf.LoadFile(source.ConfigPath)
			if err != nil {
				return nil, err
			}
			for name := range cfg.Modules {
				names = append(names, name)
			}
			sort.Strings(names)
		}
	case Source:
	default:
		return nil, errors.New("engine: invalid build input")
	}
	if len(names) == 0 {
		return nil, ErrNoJobs
	}
	for _, name := range names {
		if name == "" {
			return nil, errors.New("engine: module name is required")
		}
	}
	return names, nil
}
