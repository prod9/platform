package builds

import "time"

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Status string

// ModuleResult reduces one selected module's claim and recorded observations.
type ModuleResult struct {
	BuildModule
	Status        Status
	StartedAt     time.Time
	FinishedAt    time.Time
	Error         string
	Image         string
	Hash          string
	LastEventKind EventKind
	LastEventAt   time.Time
	firstErrorID  int64
}

// FoldModule consumes one module's observations in ascending event-id order.
func FoldModule(module BuildModule, events []*BuildEvent) ModuleResult {
	out := ModuleResult{BuildModule: module, Status: StatusQueued, StartedAt: module.ClaimedAt}
	if !module.ClaimedAt.IsZero() {
		out.Status = StatusRunning
	}

	for _, event := range events {
		out.LastEventKind, out.LastEventAt = event.Kind, event.At
		if event.Error != "" && out.Error == "" {
			out.Error, out.firstErrorID = event.Error, event.ID
		}
		if event.Kind == EventRunDone {
			out.FinishedAt = event.At
			out.Image, out.Hash = event.Image, event.Hash
			out.Status = StatusSucceeded
		}
	}
	if out.Status == StatusSucceeded && out.Error != "" {
		out.Status = StatusFailed
	}
	return out
}
