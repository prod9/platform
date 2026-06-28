package cmd

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"dagger.io/dagger"
	"fx.prodigy9.co/cmd/prompts"
	fxconfig "fx.prodigy9.co/config"
	"fx.prodigy9.co/ctrlc"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
)

var PreviewCmd = &cobra.Command{
	Use:   "preview [modules...]",
	Short: "Builds and starts up the container",
	Run:   runPreview,
}

var (
	previewPort int
	previewCmd  string
)

func init() {
	PreviewCmd.Flags().IntVarP(&previewPort, "port", "p", 0, "Binds port for preview")
	PreviewCmd.Flags().StringVarP(&previewCmd, "exec", "e", "", "Specify custom command to run")
}

func runPreview(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	attempt, err := builder.AttemptFrom(cfg, args, builder.LocalBuild)
	if err != nil {
		buildlog.Fatalln(err)
	}

	if len(attempt.Units) == 0 {
		buildlog.Fatalln(errors.New("no modules to preview"))
	}

	preview := attempt.Units[0] // at least 1 by this point
	if len(attempt.Units) > 1 {
		var names []string
		for _, unit := range attempt.Units {
			names = append(names, unit.Name)
		}

		p := prompts.New(nil, args)
		modname := p.List("preview which module?", attempt.Units[0].Name, names)

		for _, unit := range attempt.Units {
			if unit.Name == modname {
				preview = unit
			}
		}
		if preview == nil {
			buildlog.Fatalln(errors.New("no module to preview"))
		}
	}

	// build only the selected module
	attempt = builder.Attempt(attempt.Purpose, preview)

	eng := engine.New(fxconfig.Configure())
	defer eng.Close()

	ctx := engine.NewContext(context.Background(), eng)
	results, err := engine.Build(ctx, attempt)
	if err != nil {
		buildlog.Fatalln(err)
	}

	result := results[0]
	if result.Err != nil {
		buildlog.Fatalln(result.Err)
	}

	startArgs, err := result.Container.DefaultArgs(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}

	if previewPort <= 1 {
		if preview.Port == nil {
			buildlog.Fatalln(errors.New("specify preview port with --port or port= key in platform.toml"))
		} else {
			previewPort = *preview.Port
		}
	}
	if custom := strings.TrimSpace(previewCmd); custom != "" {
		preview.CommandName = custom
		preview.CommandArgs = nil // TODO: Allow specifying args?
	}

	container := result.Container.
		WithExposedPort(previewPort).
		WithExec(startArgs).
		AsService()

	tunnel := result.Client().Host().Tunnel(container, dagger.HostTunnelOpts{
		Native: true,
	})

	ctrlc.Do(func() {
		tunnel.Stop(ctx)
		os.Exit(0)
	})
	go func() {
		tunnel, err = tunnel.Start(ctx)
		if err != nil {
			buildlog.Fatalln(err)
		}
	}()

	time.Sleep(3 * time.Second)
	addr, err := tunnel.Endpoint(ctx, dagger.ServiceEndpointOpts{
		Port: previewPort,
	})
	if err != nil {
		buildlog.Fatalln(err)
	}

	buildlog.HTTPServing(addr)
}
