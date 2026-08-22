package cmd

import (
	"errors"
	"time"

	"fx.prodigy9.co/app"
	fxcmd "fx.prodigy9.co/cmd"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/worker"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/srv"
	"platform.prodigy9.co/srv/builds"
)

var WorkerCmd = buildWorkerCmd()

// workerPoll is how long a worker waits before looking for work again, and it doubles as
// the scan's cadence — so it is the latency between a pushed tag and a build starting.
// Tight on purpose: the query is a single indexed read.
const workerPoll = 5 * time.Second

func buildWorkerCmd() *cobra.Command {
	cmd := fxcmd.BuildWorkerCommand(app.CollectJobs(srv.App)...)
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		config.SetDefault(worker.PollingIntervalConfig, workerPoll)
		return seedScan(cmd, config.Configure())
	}

	return cmd
}

// seedScan queues the first sweep. Every later one is queued by the sweep before it, so
// without this a worker would poll an empty queue forever; with it, a restart is harmless
// because the scan is a singleton and a queued one already counts.
//
// It runs on its own connection: worker.Start() opens the pool it works from, and it blocks,
// so there is no moment after it in which to seed anything.
func seedScan(cmd *cobra.Command, cfg *config.Source) (err error) {
	db, err := data.Connect(cfg)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, db.Close())
	}()

	ctx := data.NewContext(config.NewContext(cmd.Context(), cfg), db)

	_, err = worker.ScheduleNowIfNotExists(ctx, &builds.ScanBuilds{})
	if errors.Is(err, worker.ErrJobExists) {
		return nil
	}
	return err
}
