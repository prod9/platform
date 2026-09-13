package builds

import (
	"context"
	"time"

	"fx.prodigy9.co/data"
)

// Step is one selected module's step, reduced from its paired observations.
type Step struct {
	BuildModuleID int64
	Step          string
	StartedAt     time.Time
	FinishedAt    time.Time
	Error         string
	Stdout        string
	Stderr        string
}

func ReadSteps(ctx context.Context, buildID int64) ([]Step, error) {
	events := []*BuildEvent{}
	err := data.Select(ctx, &events, `
		SELECT e.* FROM build_events e JOIN build_modules bm ON bm.id = e.build_module_id
		WHERE bm.build_id = $1 AND e.kind IN ('step_start', 'step_done')
		ORDER BY e.id`, buildID)
	if err != nil {
		return nil, err
	}
	return Steps(events), nil
}

// Steps consumes observations in event-id order, preserving step start order.
func Steps(events []*BuildEvent) []Step {
	type stepKey struct {
		moduleID int64
		name     string
	}
	steps := []Step{}
	open := map[stepKey]int{}
	for _, event := range events {
		key := stepKey{moduleID: event.BuildModuleID, name: event.Step}
		switch event.Kind {
		case EventStepStart:
			open[key] = len(steps)
			steps = append(steps, Step{
				BuildModuleID: event.BuildModuleID,
				Step:          event.Step,
				StartedAt:     event.At,
			})
		case EventStepDone:
			if i, ok := open[key]; ok {
				steps[i].FinishedAt, steps[i].Error = event.At, event.Error
				steps[i].Stdout, steps[i].Stderr = event.Stdout, event.Stderr
				delete(open, key)
			}
		}
	}
	return steps
}
