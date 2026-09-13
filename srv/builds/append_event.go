package builds

import (
	"context"
	"time"

	"fx.prodigy9.co/data"
)

// AppendEvent records one callback. It is the only writer of build_events: the stream is
// append-only, so there is no update or delete counterpart.
type AppendEvent struct {
	BuildModuleID int64
	Kind          EventKind
	Step          string
	At            time.Time
	Error         string
	Image         string
	Hash          string
	Stdout        string
	Stderr        string
	EngineHost    string
}

func (a *AppendEvent) Execute(ctx context.Context, out any) error {
	return data.Run(ctx, func(scope data.Scope) error {
		if a.Kind == EventConfigDone && a.Error == "" {
			if err := scope.Exec(`UPDATE build_modules
				SET engine_host = $2, engine_assigned_at = $3 WHERE id = $1`,
				a.BuildModuleID, a.EngineHost, a.At); err != nil {
				return err
			}
		}
		return scope.Exec(`INSERT INTO build_events
			(build_module_id, kind, step, at, error, image, hash, stdout, stderr)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			a.BuildModuleID, a.Kind, a.Step, a.At, a.Error, a.Image, a.Hash, a.Stdout, a.Stderr)
	})
}
