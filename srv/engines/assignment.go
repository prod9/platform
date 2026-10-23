package engines

import (
	"context"
	"errors"
	"time"

	"fx.prodigy9.co/data"
	"platform.prodigy9.co/srv/builds"
)

const assignmentSource = `SELECT b.id, r.owner, r.repo, b.ref, b.sha,
	bm.id AS module_id, bm.engine_host AS address
	FROM build_modules bm JOIN builds b ON b.id = bm.build_id
	JOIN repos r ON r.id = b.repo_id
	WHERE lower(r.owner) || '/' || lower(r.repo) = ANY($1)`

type assignmentRow struct {
	builds.Build
	ModuleID int64  `db:"module_id"`
	Address  string `db:"address"`
}

func unfinishedAssignments(ctx context.Context, keys, addresses []string, evaluatedAt time.Time) (map[string][]AssignmentView, error) {
	rows := []assignmentRow{}
	if len(keys) != 0 && len(addresses) != 0 {
		err := data.Select(ctx, &rows, assignmentSource+`
			AND bm.engine_host = ANY($2)
			AND NOT EXISTS (SELECT 1 FROM build_events e
				WHERE e.build_module_id = bm.id AND e.kind = 'run_done')
			ORDER BY bm.id DESC`, keys, addresses)
		if err != nil {
			return nil, err
		}
	}
	return foldAssignments(ctx, rows, evaluatedAt)
}

func assignmentHistory(ctx context.Context, keys []string, address string, evaluatedAt time.Time) ([]AssignmentView, error) {
	rows := []assignmentRow{}
	if len(keys) != 0 {
		err := data.Select(ctx, &rows, assignmentSource+`
			AND bm.engine_host = $2 ORDER BY bm.id DESC LIMIT 50`, keys, address)
		if err != nil {
			return nil, err
		}
	}
	views, err := foldAssignments(ctx, rows, evaluatedAt)
	if err != nil {
		return nil, err
	}
	history := views[address]
	if history == nil {
		history = []AssignmentView{}
	}
	return history, nil
}

func foldAssignments(ctx context.Context, rows []assignmentRow, evaluatedAt time.Time) (map[string][]AssignmentView, error) {
	selected := []*builds.Build{}
	seen := map[int64]bool{}
	for i := range rows {
		if !seen[rows[i].Build.ID] {
			selected = append(selected, &rows[i].Build)
			seen[rows[i].Build.ID] = true
		}
	}
	results, err := builds.ResultsFor(ctx, selected)
	if err != nil {
		return nil, err
	}
	modules := map[int64]builds.ModuleResult{}
	for _, result := range results {
		for _, module := range result.Modules {
			modules[module.ID] = module
		}
	}
	views := map[string][]AssignmentView{}
	for _, row := range rows {
		module, ok := modules[row.ModuleID]
		if !ok {
			return nil, errors.New("engines: assignment disappeared during read")
		}
		views[row.Address] = append(views[row.Address], AssignmentView{
			BuildID: row.Build.ID, BuildModuleID: module.ID, Owner: row.Owner, Repo: row.Repo,
			Ref: row.Ref, SHA: row.SHA, Name: module.Name, Status: module.Status,
			ClaimedAt: module.ClaimedAt, EngineAssignedAt: module.EngineAssignedAt,
			LastEventKind: module.LastEventKind, LastEventAt: module.LastEventAt,
			Observation: module.Observation(evaluatedAt),
		})
	}
	return views, nil
}
