package cmd

import (
	"sync"
	"time"

	"platform.prodigy9.co/internal/buildlog"
)

// observer renders a build's steps for the operator. It is where the CLI decides what a
// build looks like — buildlog is only the sink — and it is the reason a default `platform
// build` shows named steps instead of Dagger's TUI.
//
// Every command that builds passes one; nothing else in the CLI observes a run.
type observer struct {
	mtx     sync.Mutex
	started map[string]time.Time
}

func newObserver() *observer {
	return &observer{started: map[string]time.Time{}}
}

func (o *observer) StepStarted(unit, step string, at time.Time) {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	if _, ok := o.started[unit]; !ok {
		o.started[unit] = at
	}

	o.started[unit+"/"+step] = at
	buildlog.StepStart(unit, step)
}

func (o *observer) StepDone(unit, step string, at time.Time, err error) {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	buildlog.StepDone(unit, step, o.since(unit+"/"+step, at), err)
}

func (o *observer) RunDone(unit string, at time.Time, err error) {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	buildlog.BuildDone(unit, o.since(unit, at), err)
}

// since is the elapsed time of a key opened earlier, consuming it. A run's own key is
// opened by its first step, so a unit is timed from when it started doing work rather
// than from when its goroutine happened to be scheduled.
func (o *observer) since(key string, at time.Time) time.Duration {
	start, ok := o.started[key]
	if !ok {
		return 0
	}

	delete(o.started, key)
	return at.Sub(start)
}
