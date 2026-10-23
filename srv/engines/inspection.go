package engines

import (
	"context"
	"errors"

	"platform.prodigy9.co/engine"
)

var openInspection = func(ctx context.Context) inspectionSession { return engine.NewSession(ctx) }

// inspectionSession is the engine's public read/cleanup boundary; tests substitute
// observations here without bypassing the server's error and cleanup handling.
type inspectionSession interface {
	Inspect(context.Context) (engine.Inspection, error)
	Close() error
}

func inspect(ctx context.Context) (engine.Inspection, error) {
	session := openInspection(ctx)
	report, err := session.Inspect(ctx)
	if report.State == engine.RosterLookupFailed && ctx.Err() == nil {
		err = nil // Discovery failure is a reported observation, not an empty fleet.
	}
	return report, errors.Join(err, session.Close())
}
