package engine

import "time"

// Observer is how a run reports what it is doing. The caller supplies one; a nil observer
// is a guarded skip, so a caller that wants no report passes nothing and pays nothing.
//
// Every signature carries scalars only — never an engine or framework type. Go interfaces
// are structural, so that is what lets a package implement this without importing engine
// at all, which is how the leaf internal/buildlog and srv can satisfy the same contract.
//
// Callbacks fire on the goroutine driving the run, one unit per goroutine under fan-out,
// so an implementation serializes itself; the engine adds no lock on a caller's behalf.
type Observer interface {
	StepStarted(unit, step string, at time.Time)

	// StepDone ends the step StepStarted opened; err is what that step failed with, and
	// nil when it succeeded. There is no separate failure callback.
	StepDone(unit, step string, at time.Time, err error)

	// RunDone fires exactly once per run, however the run ends, so the report is
	// self-terminating and a consumer needs no out-of-band done signal.
	RunDone(unit string, at time.Time, err error)
}
