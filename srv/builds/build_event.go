package builds

import "time"

const (
	EventRunStart     EventKind = "run_start"
	EventCloneStart   EventKind = "clone_start"
	EventCloneDone    EventKind = "clone_done"
	EventConfigStart  EventKind = "config_start"
	EventConfigDone   EventKind = "config_done"
	EventStepStart    EventKind = "step_start"
	EventStepDone     EventKind = "step_done"
	EventPublishStart EventKind = "publish_start"
	EventPublishDone  EventKind = "publish_done"
	EventRunDone      EventKind = "run_done"
)

// EventKind mirrors the engine Observer; captured output rides the step_done row.
type EventKind string

// BuildEvent is one engine Observer callback as recorded. The stream is the build's only
// state — everything a reader wants about how a build went is a fold of these rows.
//
// It carries the Build prefix deliberately: "event" is already live in this domain for
// GitHub App events and Kubernetes events.
type BuildEvent struct {
	ID            int64     `db:"id"`
	BuildModuleID int64     `db:"build_module_id"`
	Kind          EventKind `db:"kind"`
	Step          string    `db:"step"`
	At            time.Time `db:"at"`
	Error         string    `db:"error"`
	Image         string    `db:"image"`
	Hash          string    `db:"hash"`
	Stdout        string    `db:"stdout"`
	Stderr        string    `db:"stderr"`
	CreatedAt     time.Time `db:"created_at"`
}
