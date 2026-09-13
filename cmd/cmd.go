package cmd

import (
	"fmt"
	"time"

	"platform.prodigy9.co/internal/termlog"
)

// exitError carries a child process's status out of a command, so a command that must
// reproduce that status still returns — leaving its deferred session Close to run — rather
// than calling os.Exit from inside a session. Execute unwraps it into the process's code.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// observer renders a build's steps for the operator. It is where the CLI decides what a
// build looks like — termlog is only the sink — and it is the reason a default `platform
// build` shows named steps instead of Dagger's TUI.
//
// Every command that builds passes one; nothing else in the CLI observes a run.
type observer struct{}

func newObserver() observer { return observer{} }

func (observer) RunStart(unit string, _ time.Time) {
	termlog.Event(unit, "started")
}

func (observer) CloneStart(unit string, _ time.Time) {
	termlog.Event(unit+"/clone", "started")
}

func (observer) CloneDone(unit string, _ time.Time, err error) {
	reportPhaseDone(unit+"/clone", err)
}

func (observer) ConfigStart(unit string, _ time.Time) {
	termlog.Event(unit+"/config", "started")
}

func (observer) ConfigDone(unit, engine string, _ time.Time, err error) {
	reportPhaseDone(unit+"/config", err)
}

func (observer) StepStart(unit, step string, _ time.Time) {
	termlog.Event(unit+"/"+step, "started")
}

// StepOutput is ignored: the CLI's report is the named-step progress, and dumping every
// step's stdout would bury it. What a failed step printed still reaches the operator —
// dagger carries it on the error StepDone hands to termlog.
func (observer) StepOutput(unit, step string, _ time.Time, stdout, stderr string) {}

func (observer) StepDone(unit, step string, _ time.Time, err error) {
	reportPhaseDone(unit+"/"+step, err)
}

func (observer) PublishStart(unit string, _ time.Time) {
	termlog.Event(unit+"/publish", "started")
}

func (observer) PublishDone(unit string, _ time.Time, err error) {
	reportPhaseDone(unit+"/publish", err)
}

func (observer) RunDone(unit, image, hash string, _ time.Time, err error) {
	if err != nil {
		return
	}
	if hash != "" {
		termlog.Image("publish", image, hash)
		return
	}
	termlog.Event(unit, "built")
}

func reportPhaseDone(object string, err error) {
	if err != nil {
		termlog.Error(err)
		return
	}
	termlog.Event(object, "done")
}
