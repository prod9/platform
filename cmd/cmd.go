package cmd

import (
	"errors"
	"fmt"
	"time"

	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/internal/termlog"
)

// exitError carries a child process's status out of a command, so a command that must
// reproduce that status still returns — leaving its deferred session Close to run — rather
// than calling os.Exit from inside a session. Execute unwraps it into the process's code.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// failedUnits joins whatever the fanned-out units failed with, so a multi-module verb can
// finish every module and fail once. The observer has already shown each failure as it
// happened; what the joined error decides is the exit code, not the report. Returning nil
// when every unit succeeded is what lets a command end with this line.
func failedUnits(results []engine.BuildResult) error {
	var errs []error
	for _, result := range results {
		if result.Err != nil {
			errs = append(errs, result.Err)
		}
	}
	return errors.Join(errs...)
}

// observer renders a build's steps for the operator. It is where the CLI decides what a
// build looks like — termlog is only the sink — and it is the reason a default `platform
// build` shows named steps instead of Dagger's TUI.
//
// Every command that builds passes one; nothing else in the CLI observes a run.
type observer struct{}

func newObserver() observer { return observer{} }

func (observer) StepStarted(unit, step string, _ time.Time) {
	termlog.Event(unit+"/"+step, "started")
}

// StepOutput is ignored: the CLI's report is the named-step progress, and dumping every
// step's stdout would bury it. What a failed step printed still reaches the operator —
// dagger carries it on the error StepDone hands to termlog.
func (observer) StepOutput(unit, step string, _ time.Time, stdout, stderr string) {}

func (observer) StepDone(unit, step string, _ time.Time, err error) {
	if err != nil {
		termlog.Error(err)
		return
	}
	termlog.Event(unit+"/"+step, "done")
}

func (observer) ImageBuilt(unit, _ string, _ time.Time) {
	termlog.Event(unit, "built")
}

func (observer) Published(_, image, hash string, _ time.Time) {
	termlog.Image("publish", image, hash)
}

// RunDone renders nothing: a failure was already shown by the step that failed, and a
// success by the image it produced. The CLI's report is what happened, not that it ended.
func (observer) RunDone(string, time.Time, error) {}
