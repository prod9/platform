// Package observer defines the scalar reporting contract for a complete module operation.
package observer

import "time"

// Observer receives paired phases on each module goroutine; consumers serialize shared state.
type Observer interface {
	RunStart(unit string, at time.Time)
	CloneStart(unit string, at time.Time)
	CloneDone(unit string, at time.Time, err error)
	ConfigStart(unit string, at time.Time)
	ConfigDone(unit, engine string, at time.Time, err error)
	StepStart(unit, step string, at time.Time)
	StepOutput(unit, step string, at time.Time, stdout, stderr string)
	StepDone(unit, step string, at time.Time, err error)
	PublishStart(unit string, at time.Time)
	PublishDone(unit string, at time.Time, err error)
	RunDone(unit, image, hash string, at time.Time, err error)
}
