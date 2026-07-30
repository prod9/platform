package cmd

import (
	"errors"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/worker"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/srv/builds"
)

var WorkerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Runs queued builds and the sweeps that find them",
	RunE:  runWorkerCmd,
}

// workerPoll is how long a worker waits before looking for work again, and it doubles as
// the scan's cadence — so it is the latency between a pushed tag and a build starting.
// Tight on purpose: the query is a single indexed read.
const workerPoll = 5 * time.Second

func runWorkerCmd(cmd *cobra.Command, args []string) error {
	config.SetDefault(worker.PollingIntervalConfig, workerPoll)
	cfg := config.Configure()

	if err := seedScan(cmd, cfg); err != nil {
		return err
	}
	return worker.New(cfg, &builds.ScanBuilds{}, &builds.RunBuild{}).Start()
}

// seedScan queues the first sweep. Every later one is queued by the sweep before it, so
// without this a worker would poll an empty queue forever; with it, a restart is harmless
// because the scan is a singleton and a queued one already counts.
//
// It runs on its own connection: worker.Start() opens the pool it works from, and it blocks,
// so there is no moment after it in which to seed anything.
func seedScan(cmd *cobra.Command, cfg *config.Source) error {
	db, err := data.Connect(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx := data.NewContext(config.NewContext(cmd.Context(), cfg), db)

	_, err = worker.ScheduleNowIfNotExists(ctx, &builds.ScanBuilds{})
	if errors.Is(err, worker.ErrJobExists) {
		return nil
	}
	return err
}
