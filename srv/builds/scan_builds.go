package builds

import (
	"context"
	"errors"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/worker"
)

// scanLimit bounds one sweep. A backlog drains across runs rather than in one transaction,
// so the queue tightens by scanning more often and never by scanning deeper.
const scanLimit = 100

// ScanBuilds turns records into work: it finds the builds nothing has reported on and
// schedules a job for each. It is the only thing that does — a controller appends a record
// and never schedules — which is what makes the trigger sources interchangeable.
//
// It is a singleton, keyed by its name, and rearms itself because fx's queue is one-shot.
type ScanBuilds struct{}

var _ worker.Interface = (*ScanBuilds)(nil)

func (*ScanBuilds) Name() string { return "scan_builds" }

func (s *ScanBuilds) Run(ctx context.Context) (err error) {
	// The rearm is deferred so the sweep keeps its cadence through a failure: a scan that
	// stopped scheduling itself would take the whole pipeline down with it.
	defer func() { err = errors.Join(err, s.rearm(ctx)) }()

	ids, err := unclaimedBuildIDs(ctx)
	if err != nil {
		return err
	}

	for _, id := range ids {
		if _, err := worker.ScheduleNow(ctx, &RunBuild{BuildID: id}); err != nil {
			return err
		}
	}
	return nil
}

// rearm queues the next sweep one poll interval out — the same knob that decides how
// promptly a worker picks anything up. ErrJobExists is the singleton holding: another
// process already queued it.
func (s *ScanBuilds) rearm(ctx context.Context) error {
	interval := config.Get(config.FromContext(ctx), worker.PollingIntervalConfig)

	_, err := worker.ScheduleInIfNotExists(ctx, s, interval)
	if errors.Is(err, worker.ErrJobExists) {
		return nil
	}
	return err
}

// unclaimedBuildIDs are the builds no worker has taken. A build job reports before it does
// anything, so the absence of an event is what makes a record still work — and what keeps
// an overlapping sweep from handing the same build to a second worker.
func unclaimedBuildIDs(ctx context.Context) ([]int64, error) {
	ids := []int64{}
	err := data.Select(ctx, &ids, `
		SELECT id FROM builds
		WHERE NOT EXISTS (SELECT 1 FROM build_events WHERE build_events.build_id = builds.id)
		ORDER BY id
		LIMIT $1`, scanLimit)
	if err != nil {
		return nil, err
	}
	return ids, nil
}
