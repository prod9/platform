package observer

import "time"

// teeObserver requires non-nil children; Accumulate eliminates nil at the boundary.
type teeObserver []Observer

func Tee(obs ...Observer) Observer { return teeObserver(obs) }

func (t teeObserver) RunStart(unit string, at time.Time) {
	for _, obs := range t {
		obs.RunStart(unit, at)
	}
}

func (t teeObserver) CloneStart(unit string, at time.Time) {
	for _, obs := range t {
		obs.CloneStart(unit, at)
	}
}

func (t teeObserver) CloneDone(unit string, at time.Time, err error) {
	for _, obs := range t {
		obs.CloneDone(unit, at, err)
	}
}

func (t teeObserver) ConfigStart(unit string, at time.Time) {
	for _, obs := range t {
		obs.ConfigStart(unit, at)
	}
}

func (t teeObserver) ConfigDone(unit, engine string, at time.Time, err error) {
	for _, obs := range t {
		obs.ConfigDone(unit, engine, at, err)
	}
}

func (t teeObserver) StepStart(unit, step string, at time.Time) {
	for _, obs := range t {
		obs.StepStart(unit, step, at)
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

func (t teeObserver) PublishStart(unit string, at time.Time) {
	for _, obs := range t {
		obs.PublishStart(unit, at)
	}
}

func (t teeObserver) PublishDone(unit string, at time.Time, err error) {
	for _, obs := range t {
		obs.PublishDone(unit, at, err)
	}
}

func (t teeObserver) RunDone(unit, image, hash string, at time.Time, err error) {
	for _, obs := range t {
		obs.RunDone(unit, image, hash, at, err)
	}
}
