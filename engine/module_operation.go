package engine

import (
	"context"
	"errors"
	"sort"
	"time"

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
	input   Local
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

	cursor, err := o.prepare(obs)
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

func (o *moduleOperation) prepare(obs observer.Observer) (*Run, error) {
	obs.ConfigStart(o.name, time.Now())
	cursor, host, err := o.configure(obs)
	obs.ConfigDone(o.name, host, time.Now(), err)
	return cursor, err
}

func (o *moduleOperation) configure(obs observer.Observer) (*Run, string, error) {
	cfg, err := conf.LoadFile(o.input.ConfigPath)
	if err != nil {
		return nil, "", err
	}

	units, err := framework.Units(cfg, []string{o.name}, o.intent.arch(o.session, cfg), o.input.Version)
	if err != nil {
		return nil, "", err
	}
	unit := units[0]
	creds := registryCredsFrom(cfgFrom(o.session.ctx))
	unit.RegistryUsername, unit.RegistryPassword = creds.username, creds.password
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

func selectedNames(input Local, names []string) ([]string, error) {
	if len(names) == 0 {
		cfg, err := conf.LoadFile(input.ConfigPath)
		if err != nil {
			return nil, err
		}
		for name := range cfg.Modules {
			names = append(names, name)
		}
		sort.Strings(names)
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
