package builds

import "time"

const (
	ObservationNotStarted ObservationState = "not_started"
	ObservationRecent     ObservationState = "recent"
	ObservationQuiet      ObservationState = "quiet"
	ObservationTerminal   ObservationState = "terminal"

	observationQuietAfter = 5 * time.Minute
)

type ObservationState string

// Observation describes persisted evidence age, independently of execution outcome.
type Observation struct {
	State             ObservationState `json:"state"`
	EvaluatedAt       time.Time        `json:"evaluated_at"`
	LastObservedAt    time.Time        `json:"last_observed_at"`
	QuietAfterSeconds int64            `json:"quiet_after_seconds"`
}

func (m ModuleResult) Observation(at time.Time) Observation {
	last := m.ClaimedAt
	if m.EngineAssignedAt.After(last) {
		last = m.EngineAssignedAt
	}
	if m.LastEventAt.After(last) {
		last = m.LastEventAt
	}
	observation := Observation{
		State:             ObservationRecent,
		EvaluatedAt:       at,
		LastObservedAt:    last,
		QuietAfterSeconds: int64(observationQuietAfter / time.Second),
	}

	switch {
	case m.Status == StatusSucceeded || m.Status == StatusFailed:
		observation.State = ObservationTerminal
	case m.ClaimedAt.IsZero():
		observation.State = ObservationNotStarted
	case at.Sub(last) >= observationQuietAfter:
		observation.State = ObservationQuiet
	}
	return observation
}
