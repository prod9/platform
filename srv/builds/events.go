package builds

import (
	"context"
	"time"

	"fx.prodigy9.co/data"
)

// EventKind is the engine callback a row transcribes. The set is closed and mirrors the
// Observer contract (docs/spec/engine.md); captured output rides the step_done row rather
// than earning a kind of its own.
type EventKind string

const (
	EventStepStarted EventKind = "step_started"
	EventStepDone    EventKind = "step_done"
	EventImageBuilt  EventKind = "image_built"
	EventPublished   EventKind = "published"
	EventRunDone     EventKind = "run_done"
)

// BuildEvent is one engine Observer callback as recorded. The stream is the build's only
// state — everything a reader wants about how a build went is a fold of these rows.
//
// It carries the Build prefix deliberately: "event" is already live in this domain for
// GitHub App events and Kubernetes events.
type BuildEvent struct {
	ID        int64     `db:"id"`
	BuildID   int64     `db:"build_id"`
	Kind      EventKind `db:"kind"`
	Unit      string    `db:"unit"`
	Step      string    `db:"step"`
	At        time.Time `db:"at"`
	Error     string    `db:"error"`
	Image     string    `db:"image"`
	Hash      string    `db:"hash"`
	Stdout    string    `db:"stdout"`
	Stderr    string    `db:"stderr"`
	CreatedAt time.Time `db:"created_at"`
}

// AppendEvent records one callback. It is the only writer of build_events: the stream is
// append-only, so there is no update or delete counterpart.
type AppendEvent struct {
	BuildID int64
	Kind    EventKind
	Unit    string
	Step    string
	At      time.Time
	Error   string
	Image   string
	Hash    string
	Stdout  string
	Stderr  string
}

func (a *AppendEvent) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		INSERT INTO build_events (build_id, kind, unit, step, at, error, image, hash, stdout, stderr)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		a.BuildID, a.Kind, a.Unit, a.Step, a.At, a.Error, a.Image, a.Hash, a.Stdout, a.Stderr)
}
