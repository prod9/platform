package cmd

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"dagger.io/dagger"
	"fx.prodigy9.co/cmd/prompts"
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
	attempt.Units = []*builder.BuildUnit{preview}

	sess, err := engine.New(context.Background())
	if err != nil {
		buildlog.Fatalln(err)
	}
	defer sess.Close()

	results, err := engine.Build(sess, attempt)
	if err != nil {
		buildlog.Fatalln(err)
	}

	result := results[0]
	if result.Err != nil {
		buildlog.Fatalln(result.Err)
	}

	startArgs, err := result.Container.DefaultArgs(sess.Context())
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
	if cmd := strings.TrimSpace(previewCmd); cmd != "" {
		preview.CommandName = cmd
		preview.CommandArgs = nil // TODO: Allow specifying args?
	}

	container := result.Container.
		WithExposedPort(previewPort).
		WithExec(startArgs).
		AsService()

	tunnel := sess.Client().Host().Tunnel(container, dagger.HostTunnelOpts{
		Native: true,
	})

	ctrlc.Do(func() {
		tunnel.Stop(sess.Context())
		os.Exit(0)
	})
	go func() {
		tunnel, err = tunnel.Start(sess.Context())
		if err != nil {
			buildlog.Fatalln(err)
		}
	}()

	time.Sleep(3 * time.Second)
	addr, err := tunnel.Endpoint(sess.Context(), dagger.ServiceEndpointOpts{
		Port: previewPort,
	})
	if err != nil {
		buildlog.Fatalln(err)
	}

	buildlog.HTTPServing(addr)
}
