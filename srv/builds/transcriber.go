package builds

import (
	"context"
	"sync"
	"time"
)

// transcriber records one claimed module's lifecycle and preserves write failures.
type transcriber struct {
	ctx      context.Context
	moduleID int64
	mu       sync.Mutex
	err      error
	complete bool
	captured map[string]capture
}

type capture struct{ stdout, stderr string }

func newTranscriber(ctx context.Context, moduleID int64) *transcriber {
	return &transcriber{ctx: ctx, moduleID: moduleID, captured: map[string]capture{}}
}

func (t *transcriber) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.err
}

func (t *transcriber) Complete() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.complete
}

func (t *transcriber) RunStart(_ string, at time.Time) {
	t.append(&AppendEvent{Kind: EventRunStart, At: at})
}

func (t *transcriber) CloneStart(_ string, at time.Time) {
	t.append(&AppendEvent{Kind: EventCloneStart, At: at})
}

func (t *transcriber) CloneDone(_ string, at time.Time, err error) {
	t.append(&AppendEvent{Kind: EventCloneDone, At: at, Error: errText(err)})
}

func (t *transcriber) ConfigStart(_ string, at time.Time) {
	t.append(&AppendEvent{Kind: EventConfigStart, At: at})
}

func (t *transcriber) ConfigDone(_, host string, at time.Time, err error) {
	t.append(&AppendEvent{Kind: EventConfigDone, At: at, Error: errText(err), EngineHost: host})
}

func (t *transcriber) StepStart(_, step string, at time.Time) {
	t.append(&AppendEvent{Kind: EventStepStart, Step: step, At: at})
}

func (t *transcriber) StepOutput(_, step string, _ time.Time, stdout, stderr string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.captured[step] = capture{stdout, stderr}
}

func (t *transcriber) StepDone(_, step string, at time.Time, err error) {
	t.mu.Lock()
	out := t.captured[step]
	delete(t.captured, step)
	t.mu.Unlock()

	t.append(&AppendEvent{Kind: EventStepDone, Step: step, At: at,
		Error: errText(err), Stdout: out.stdout, Stderr: out.stderr})
}

func (t *transcriber) PublishStart(_ string, at time.Time) {
	t.append(&AppendEvent{Kind: EventPublishStart, At: at})
}

func (t *transcriber) PublishDone(_ string, at time.Time, err error) {
	t.append(&AppendEvent{Kind: EventPublishDone, At: at, Error: errText(err)})
}

func (t *transcriber) RunDone(_, image, hash string, at time.Time, err error) {
	t.append(&AppendEvent{Kind: EventRunDone, At: at, Error: errText(err), Image: image, Hash: hash})
}

func (t *transcriber) append(event *AppendEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return
	}

	event.BuildModuleID = t.moduleID
	t.err = event.Execute(t.ctx, nil)
	if t.err == nil && event.Kind == EventRunDone {
		t.complete = true
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
