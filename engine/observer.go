package engine

import "time"

// Observer is how a run reports what it is doing. Three callbacks are lifecycle and two are
// output: ImageBuilt is the common path — every successful build fires it, and most commands
// that build never publish — while Published fires only on the publish path and is the only
// place a registry hash exists. One callback per event kind; nothing is inferred from a
// shared method with a mode flag.
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

	// ImageBuilt announces the image a successful run produced, before that run ends.
	ImageBuilt(unit, image string, at time.Time)

	// Published announces a push and carries the registry hash, which exists nowhere else.
	Published(unit, image, hash string, at time.Time)

	// RunDone fires exactly once per run, however the run ends, so the report is
	// self-terminating and a consumer needs no out-of-band done signal.
	RunDone(unit string, at time.Time, err error)
}

// outcome is what a run's stream folds down to: the scalars a result is minted from. It is
// data and nothing else, which is what keeps the observer that writes it out of every field
// that only wants the scalars.
type outcome struct {
	image string
	hash  string
	err   error
}

// fail keeps the first failure reported: the step that broke is the cause, and the run's own
// terminal error is that same error arriving a second time.
func (o *outcome) fail(err error) {
	if o.err == nil {
		o.err = err
	}
}

// accumulate composes the observer a run reports to and hands back the fold that observer
// writes. Both come from one call because they are one decision — a run always folds, and
// composing the caller in is the same act — and a caller gets an Observer plus an *outcome,
// never the accumulator's own type.
//
// This is where nil is eliminated: a nil caller yields the bare accumulator, so Tee never
// sees a nil child and no downstream report path carries a guard.
func accumulate(caller Observer) (Observer, *outcome) {
	out := &outcome{}

	obs := Observer(&accObserver{out})
	if caller != nil {
		obs = Tee(obs, caller)
	}

	return obs, out
}

// accObserver folds the callbacks into a run's outcome. The engine injects one into every
// run and it is the sole minter of that outcome, so what a result says is derived from what
// the run actually reported rather than authored at a call site — which is what makes an
// inconsistent result unconstructable instead of merely discouraged.
type accObserver struct{ out *outcome }

func (*accObserver) StepStarted(unit, step string, at time.Time) {}

func (a *accObserver) StepDone(unit, step string, at time.Time, err error) {
	a.out.fail(err)
}

func (a *accObserver) ImageBuilt(unit, image string, at time.Time) {
	a.out.image = image
}

// Published overwrites the image because pushing renames it: the tag a run built under is
// not the ref that ended up in the registry.
func (a *accObserver) Published(unit, image, hash string, at time.Time) {
	a.out.image, a.out.hash = image, hash
}

func (a *accObserver) RunDone(unit string, at time.Time, err error) {
	a.out.fail(err)
}

// TeeObserver forwards every callback to each of its children. Its contract is non-nil
// children only — nil is eliminated once, where the tee is composed, so nothing downstream
// carries a guard.
type TeeObserver []Observer

func Tee(obs ...Observer) Observer { return TeeObserver(obs) }

func (t TeeObserver) StepStarted(unit, step string, at time.Time) {
	for _, obs := range t {
		obs.StepStarted(unit, step, at)
	}
}

func (t TeeObserver) StepDone(unit, step string, at time.Time, err error) {
	for _, obs := range t {
		obs.StepDone(unit, step, at, err)
	}
}

func (t TeeObserver) ImageBuilt(unit, image string, at time.Time) {
	for _, obs := range t {
		obs.ImageBuilt(unit, image, at)
	}
}

func (t TeeObserver) Published(unit, image, hash string, at time.Time) {
	for _, obs := range t {
		obs.Published(unit, image, hash, at)
	}
}

func (t TeeObserver) RunDone(unit string, at time.Time, err error) {
	for _, obs := range t {
		obs.RunDone(unit, at, err)
	}
}
