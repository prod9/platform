package builds

import (
	"context"
	"sync"
	"time"
)

// transcriber turns a run's Observer callbacks into the build's event stream. It is the
// engine's only route into the database: the database is the channel a webui reads a build
// through, so a callback that is not written here is a moment the build never had.
//
// Callbacks fire on the engine's per-unit goroutines and cannot return an error, so it
// serializes itself and holds the first write failure for the job to report.
type transcriber struct {
	ctx     context.Context
	buildID int64

	mu       sync.Mutex
	err      error
	captured map[string]capture
}

// capture is a step's output waiting for the row it rides on. Captured output is not an
// event of its own — it lands on the step_done row that ends the same step — so it is held
// from StepOutput until StepDone, keyed per step because units run concurrently.
type capture struct{ stdout, stderr string }

func newTranscriber(ctx context.Context, buildID int64) *transcriber {
	return &transcriber{ctx: ctx, buildID: buildID, captured: map[string]capture{}}
}

// Err is the first failure to write, and it is the whole of what a build job fails on: the
// build's own failure belongs in the stream, not in the job's return value.
func (t *transcriber) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.err
}

func (t *transcriber) StepStarted(unit, step string, at time.Time) {
	t.append(&AppendEvent{Kind: EventStepStarted, Unit: unit, Step: step, At: at})
}

func (t *transcriber) StepOutput(unit, step string, at time.Time, stdout, stderr string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.captured[unit+"/"+step] = capture{stdout, stderr}
}

func (t *transcriber) StepDone(unit, step string, at time.Time, err error) {
	out := t.takeCapture(unit, step)
	t.append(&AppendEvent{
		Kind: EventStepDone, Unit: unit, Step: step, At: at,
		Error:  errText(err),
		Stdout: out.stdout,
		Stderr: out.stderr,
	})
}

func (t *transcriber) ImageBuilt(unit, image string, at time.Time) {
	t.append(&AppendEvent{Kind: EventImageBuilt, Unit: unit, At: at, Image: image})
}

func (t *transcriber) Published(unit, image, hash string, at time.Time) {
	t.append(&AppendEvent{Kind: EventPublished, Unit: unit, At: at, Image: image, Hash: hash})
}

func (t *transcriber) RunDone(unit string, at time.Time, err error) {
	t.append(&AppendEvent{Kind: EventRunDone, Unit: unit, At: at, Error: errText(err)})
}

// takeCapture hands over what the step printed and forgets it, so a long build does not
// accumulate every line it ever printed.
func (t *transcriber) takeCapture(unit, step string) capture {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := unit + "/" + step
	out := t.captured[key]
	delete(t.captured, key)
	return out
}

// append writes one row and keeps the first failure. Later callbacks still try: a stream
// missing its tail is worse than one missing a row in the middle, and the job reports the
// failure either way.
func (t *transcriber) append(event *AppendEvent) {
	event.BuildID = t.buildID
	err := event.Execute(t.ctx, nil)

	t.mu.Lock()
	defer t.mu.Unlock()

	if err != nil && t.err == nil {
		t.err = err
	}
}

// errText is how a failure reaches a text column: the stream stores what went wrong, and
// nothing downstream unwraps a Go error out of it.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
