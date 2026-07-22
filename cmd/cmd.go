package cmd

import (
	"time"

	"platform.prodigy9.co/internal/buildlog"
)

// observer renders a build's steps for the operator. It is where the CLI decides what a
// build looks like — buildlog is only the sink — and it is the reason a default `platform
// build` shows named steps instead of Dagger's TUI.
//
// Every command that builds passes one; nothing else in the CLI observes a run.
type observer struct{}

func newObserver() observer { return observer{} }

func (observer) StepStarted(unit, step string, _ time.Time) {
	buildlog.Event(unit+"/"+step, "started")
}

func (observer) StepDone(unit, step string, _ time.Time, err error) {
	if err != nil {
		buildlog.Error(err)
		return
	}
	buildlog.Event(unit+"/"+step, "done")
}

func (observer) RunDone(unit string, _ time.Time, err error) {
	if err != nil {
		return
	}
	buildlog.Event(unit, "built")
}
