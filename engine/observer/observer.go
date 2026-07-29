// Package observer is the reporting vocabulary a build run speaks: the Observer contract,
// the tee that composes two of them, and the accumulator that folds a stream down to an
// Outcome. It knows nothing of engines, containers or frameworks — every callback carries
// scalars only, which is what lets any package satisfy it without importing this one.
package observer

import "time"

// Observer is how a run reports what it is doing. Three callbacks are lifecycle and two are
// output: ImageBuilt is the common path — every successful build fires it, and most commands
// that build never publish — while Published fires only on the publish path and is the only
// place a registry hash exists. One callback per event kind; nothing is inferred from a
// shared method with a mode flag.
//
// Every signature carries scalars only — never an engine or framework type. Go interfaces
// are structural, so that is what lets a package implement this without importing engine
// at all, which is how the leaf internal/termlog and srv can satisfy the same contract.
//
// Callbacks fire on the goroutine driving the run, one unit per goroutine under fan-out,
// so an implementation serializes itself; the engine adds no lock on a caller's behalf.
type Observer interface {
	StepStarted(unit, step string, at time.Time)

	// StepDone ends the step StepStarted opened; err is what that step failed with, and
	// nil when it succeeded. There is no separate failure callback.
	StepDone(unit, step string, at time.Time, err error)

	// ImageBuilt announces the image a successful run produced, before that run ends.
	ImageBuilt(unit, image string, at time.Time)

	// Published announces a push and carries the registry hash, which exists nowhere else.
	Published(unit, image, hash string, at time.Time)

	// RunDone fires exactly once per run, however the run ends, so the report is
	// self-terminating and a consumer needs no out-of-band done signal.
	RunDone(unit string, at time.Time, err error)
}
