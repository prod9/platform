package cmd

import (
	"context"
	"errors"

	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/internal/buildlog"
)

var ExportCmd = &cobra.Command{
	Use:   "export [modules...]",
	Short: "Builds and exports the container to a docker-compatible format",
	RunE:  runExport,
}

func runExport(cmd *cobra.Command, args []string) error {
	cfg, err := conf.Load(".")
	if err != nil {
		return err
	}

	ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
	sess := engine.NewSession(ctx)
	defer sess.Close()

	results, err := sess.Build(ctx, cfg, args, newObserver())
	if err != nil {
		return err
	}

	// Every unit that did build gets written out, whatever its neighbours did: a module
	// that succeeded has already paid for its image, and which goroutine failed first is
	// no reason to withhold it. The joined error decides the exit code at the end.
	errs := []error{failedUnits(results)}
	for _, result := range results {
		if result.Err != nil {
			continue
		}
		errs = append(errs, exportImage(ctx, result))
	}
	return errors.Join(errs...)
}

// exportImage writes one built container to <module>.docker beside the config and logs the
// image id it wrote, trimmed to the tail that is legible in a terminal.
func exportImage(ctx context.Context, result engine.BuildResult) error {
	container := result.UnsafeContainer()

	id, err := container.ID(ctx)
	if err != nil {
		return err
	}
	if len(id) > 16 {
		id = id[len(id)-16:]
	}

	outname := result.Unit.Name + ".docker"
	if _, err := container.Export(ctx, outname); err != nil {
		return err
	}

	buildlog.Image("export", outname, string(id))
	return nil
}
