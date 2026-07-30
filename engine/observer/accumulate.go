package observer

import "time"

// Outcome is what a run's stream folds down to: the scalars a result is minted from. It is
// data and nothing else, which is what keeps the observer that writes it out of every field
// that only wants the scalars. Its fields are exported because the fold crosses the package
// line into the engine; the observer that writes them never does.
type Outcome struct {
	Image string
	Hash  string
	Err   error
}

// Accumulate composes the observer a run reports to and hands back the fold that observer
// writes. Both come from one call because they are one decision — a run always folds, and
// composing the caller in is the same act — and a caller gets an Observer plus an *Outcome,
// never the accumulator's own type.
//
// This is where nil is eliminated: a nil caller yields the bare accumulator, so Tee never
// sees a nil child and no downstream report path carries a guard.
func Accumulate(caller Observer) (Observer, *Outcome) {
	out := &Outcome{}

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
type accObserver struct{ out *Outcome }

func (*accObserver) StepStarted(unit, step string, at time.Time) {}

// StepOutput folds nothing: what a step printed belongs to whoever stores it, and an
// outcome that grew with every line a build printed would no longer be a fold.
func (*accObserver) StepOutput(unit, step string, at time.Time, stdout, stderr string) {}

func (a *accObserver) StepDone(unit, step string, at time.Time, err error) {
	a.out.fail(err)
}

func (a *accObserver) ImageBuilt(unit, image string, at time.Time) {
	a.out.Image = image
}

// Published overwrites the image because pushing renames it: the tag a run built under is
// not the ref that ended up in the registry.
func (a *accObserver) Published(unit, image, hash string, at time.Time) {
	a.out.Image, a.out.Hash = image, hash
}

func (a *accObserver) RunDone(unit string, at time.Time, err error) {
	a.out.fail(err)
}

// fail keeps the first failure reported: the step that broke is the cause, and the run's own
// terminal error is that same error arriving a second time.
func (o *Outcome) fail(err error) {
	if o.Err == nil {
		o.Err = err
	}
}
