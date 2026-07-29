package builds

import "time"

// Status is a build attempt's state as folded from its events. It is computed per read
// and never stored: a status column would be a cache of the stream, and a cache that does
// not exist cannot go stale.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// BuildAttempt is one Start→terminal span of a build's stream, reduced. It is an output
// type — the display shape a reader folds out of the events — and never an input to the
// build path.
type BuildAttempt struct {
	Status     Status
	StartedAt  time.Time
	FinishedAt time.Time
	Image      string
	Hash       string
	Error      string
}

// fold reduces a build's stream into its attempts, oldest first. A retry appends to the
// same stream rather than starting a new one, so a terminal boundary is where one attempt
// ends and the next begins.
func fold(events []*BuildEvent) []BuildAttempt {
	attempts, span := []BuildAttempt{}, newSpan()
	for _, event := range events {
		if span.done() {
			attempts = append(attempts, span.attempt())
			span = newSpan()
		}
		span.absorb(event)
	}

	if !span.empty() {
		attempts = append(attempts, span.attempt())
	}
	return attempts
}

// Latest is the attempt a list view shows: the newest span, or a queued attempt when the
// engine has reported nothing yet.
func Latest(events []*BuildEvent) BuildAttempt {
	attempts := fold(events)
	if len(attempts) == 0 {
		return BuildAttempt{Status: StatusQueued}
	}
	return attempts[len(attempts)-1]
}

// span accumulates one attempt while the fold walks the stream. A build runs its units
// concurrently, so an attempt is terminal only once every unit that reported anything has
// also reported its run_done — a single run_done is one unit finishing, not the build.
type span struct {
	partial  BuildAttempt
	started  map[string]bool
	finished map[string]bool
}

func newSpan() *span {
	return &span{
		partial:  BuildAttempt{Status: StatusRunning},
		started:  map[string]bool{},
		finished: map[string]bool{},
	}
}

func (s *span) absorb(event *BuildEvent) {
	if s.empty() {
		s.partial.StartedAt = event.At
	}
	s.started[event.Unit] = true

	switch event.Kind {
	case EventImageBuilt:
		s.partial.Image = event.Image
	case EventPublished:
		// Pushing renames the image: the tag a run built under is not the ref that
		// ended up in the registry.
		s.partial.Image, s.partial.Hash = event.Image, event.Hash
	case EventRunDone:
		s.finished[event.Unit] = true
		s.partial.FinishedAt = event.At
	}

	// The first error reported is the cause; a unit's terminal error is usually that same
	// error arriving a second time.
	if event.Error != "" && s.partial.Error == "" {
		s.partial.Error = event.Error
	}
}

func (s *span) done() bool {
	return !s.empty() && len(s.finished) == len(s.started)
}

func (s *span) empty() bool {
	return len(s.started) == 0
}

// attempt mints the span's finished shape: a still-running span reports neither an
// outcome nor a finish time, whatever its units have said so far.
func (s *span) attempt() BuildAttempt {
	out := s.partial
	if !s.done() {
		out.Status, out.FinishedAt = StatusRunning, time.Time{}
		return out
	}

	out.Status = StatusSucceeded
	if out.Error != "" {
		out.Status = StatusFailed
	}
	return out
}
