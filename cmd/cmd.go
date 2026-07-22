package cmd

import (
	"sync"
	"time"

	"platform.prodigy9.co/internal/buildlog"
)

// progress renders a build's steps for the operator. It is where the CLI decides what a
// build looks like — buildlog is only the sink — and it is the reason a default `platform
// build` shows named steps instead of Dagger's TUI.
//
// Every command that builds passes one; nothing else in the CLI observes a run.
type progress struct {
	mtx     sync.Mutex
	started map[string]time.Time
}

func newProgress() *progress {
	return &progress{started: map[string]time.Time{}}
}

func (p *progress) StepStarted(unit, step string, at time.Time) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if _, ok := p.started[unit]; !ok {
		p.started[unit] = at
	}

	p.started[unit+"/"+step] = at
	buildlog.StepStart(unit, step)
}

func (p *progress) StepDone(unit, step string, at time.Time, err error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	buildlog.StepDone(unit, step, p.since(unit+"/"+step, at), err)
}

func (p *progress) RunDone(unit string, at time.Time, err error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	buildlog.BuildDone(unit, p.since(unit, at), err)
}

// since is the elapsed time of a key opened earlier, consuming it. A run's own key is
// opened by its first step, so a unit is timed from when it started doing work rather
// than from when its goroutine happened to be scheduled.
func (p *progress) since(key string, at time.Time) time.Duration {
	start, ok := p.started[key]
	if !ok {
		return 0
	}

	delete(p.started, key)
	return at.Sub(start)
}
