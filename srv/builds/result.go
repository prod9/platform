package builds

import (
	"context"
	"time"

	"fx.prodigy9.co/data"
)

// Result is the read view of the complete selected module set.
type Result struct {
	Status     Status
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string
	Modules    []ModuleResult
}

// ResultsFor loads the authorized page's selected modules and lightweight observations
// in two batches, including modules whose workers have not written any events.
func ResultsFor(ctx context.Context, builds []*Build) (map[int64]Result, error) {
	results := make(map[int64]Result, len(builds))
	if len(builds) == 0 {
		return results, nil
	}
	ids := make([]int64, len(builds))
	for i, build := range builds {
		ids[i] = build.ID
	}

	modules := []BuildModule{}
	err := data.Select(ctx, &modules, `SELECT `+moduleColumns+` FROM `+moduleFrom+`
		WHERE bm.build_id = ANY($1) ORDER BY bm.id`, ids)
	if err != nil {
		return nil, err
	}
	events := []*BuildEvent{}
	err = data.Select(ctx, &events, `
		SELECT e.id, e.build_module_id, e.kind, e.step, e.at, e.error,
		       e.image, e.hash, e.created_at
		FROM build_events e JOIN build_modules bm ON bm.id = e.build_module_id
		WHERE bm.build_id = ANY($1) ORDER BY e.id`, ids)
	if err != nil {
		return nil, err
	}

	streams := make(map[int64][]*BuildEvent, len(modules))
	for _, event := range events {
		streams[event.BuildModuleID] = append(streams[event.BuildModuleID], event)
	}
	selected := make(map[int64][]ModuleResult, len(builds))
	for _, module := range modules {
		selected[module.BuildID] = append(selected[module.BuildID], FoldModule(module, streams[module.ID]))
	}
	for _, id := range ids {
		results[id] = ReduceModules(selected[id])
	}
	return results, nil
}

func ReduceModules(modules []ModuleResult) Result {
	out := Result{Status: StatusQueued, Modules: modules}
	var firstErrorID int64
	var latestFinish time.Time
	terminal := 0
	for _, module := range modules {
		if !module.StartedAt.IsZero() && (out.StartedAt.IsZero() || module.StartedAt.Before(out.StartedAt)) {
			out.StartedAt = module.StartedAt
		}
		if module.FinishedAt.After(latestFinish) {
			latestFinish = module.FinishedAt
		}
		if module.Error != "" && (out.Error == "" || module.firstErrorID < firstErrorID) {
			out.Error, firstErrorID = module.Error, module.firstErrorID
		}
		switch module.Status {
		case StatusSucceeded, StatusFailed:
			terminal++
			out.Status = StatusRunning
		case StatusRunning:
			out.Status = StatusRunning
		}
	}

	if len(modules) > 0 && terminal == len(modules) {
		out.Status, out.FinishedAt = StatusSucceeded, latestFinish
		if out.Error != "" {
			out.Status = StatusFailed
		}
	}
	return out
}
