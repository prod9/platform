package builds

import "time"

// Step is one unit's step within one attempt, reduced from its step_started/step_done
// pair. It is flat by convention (spec §Data-domain structs stay flat): the attempt
// ordinal is a field indexing into the same stream's folded attempts, never a nesting.
type Step struct {
	Attempt    int
	Unit       string
	Step       string
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string
	Stdout     string
	Stderr     string
}

// Steps reduces a build's stream into its steps across all attempts, flat, in order of
// first appearance. It rides the same span walk as fold, so the two folds cannot
// disagree on where an attempt ends. A step with no step_done yet stays listed with a
// zero FinishedAt — the detail view shows what is running, not only what finished.
func Steps(events []*BuildEvent) []Step {
	steps, attempt, span := []Step{}, 0, newSpan()
	open := map[[2]string]int{} // unit+step → index into steps, reset per attempt
	for _, event := range events {
		if span.done() {
			attempt++
			span, open = newSpan(), map[[2]string]int{}
		}
		span.absorb(event)

		key := [2]string{event.Unit, event.Step}
		switch event.Kind {
		case EventStepStarted:
			open[key] = len(steps)
			steps = append(steps, Step{
				Attempt:   attempt,
				Unit:      event.Unit,
				Step:      event.Step,
				StartedAt: event.At,
			})
		case EventStepDone:
			if i, ok := open[key]; ok {
				steps[i].FinishedAt = event.At
				steps[i].Error = event.Error
				steps[i].Stdout, steps[i].Stderr = event.Stdout, event.Stderr
			}
		}
	}
	return steps
}
