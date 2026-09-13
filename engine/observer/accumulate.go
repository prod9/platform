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

func (a *accObserver) RunStart(unit string, at time.Time) {}

func (a *accObserver) CloneStart(unit string, at time.Time) {}

func (a *accObserver) CloneDone(unit string, at time.Time, err error) { a.out.fail(err) }

func (a *accObserver) ConfigStart(unit string, at time.Time) {}

func (a *accObserver) ConfigDone(unit, engine string, at time.Time, err error) { a.out.fail(err) }

func (a *accObserver) StepStart(unit, step string, at time.Time) {}

func (a *accObserver) StepOutput(unit, step string, at time.Time, stdout, stderr string) {}

func (a *accObserver) StepDone(unit, step string, at time.Time, err error) { a.out.fail(err) }

func (a *accObserver) PublishStart(unit string, at time.Time) {}

func (a *accObserver) PublishDone(unit string, at time.Time, err error) { a.out.fail(err) }

func (a *accObserver) RunDone(unit, image, hash string, at time.Time, err error) {
	a.out.Image, a.out.Hash = image, hash
	a.out.fail(err)
}

func (o *Outcome) fail(err error) {
	if o.Err == nil {
		o.Err = err
	}
}
