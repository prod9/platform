package observer

import "time"

// teeObserver forwards every callback to each of its children. Its contract is non-nil
// children only — nil is eliminated once, where the tee is composed, so nothing downstream
// carries a guard.
type teeObserver []Observer

func Tee(obs ...Observer) Observer { return teeObserver(obs) }

func (t teeObserver) StepStarted(unit, step string, at time.Time) {
	for _, obs := range t {
		obs.StepStarted(unit, step, at)
	}
}

func (t teeObserver) StepOutput(unit, step string, at time.Time, stdout, stderr string) {
	for _, obs := range t {
		obs.StepOutput(unit, step, at, stdout, stderr)
	}
}

func (t teeObserver) StepDone(unit, step string, at time.Time, err error) {
	for _, obs := range t {
		obs.StepDone(unit, step, at, err)
	}
}

func (t teeObserver) ImageBuilt(unit, image string, at time.Time) {
	for _, obs := range t {
		obs.ImageBuilt(unit, image, at)
	}
}

func (t teeObserver) Published(unit, image, hash string, at time.Time) {
	for _, obs := range t {
		obs.Published(unit, image, hash, at)
	}
}

func (t teeObserver) RunDone(unit string, at time.Time, err error) {
	for _, obs := range t {
		obs.RunDone(unit, at, err)
	}
}
