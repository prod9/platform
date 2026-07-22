package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"dagger.io/dagger"
	fxconfig "fx.prodigy9.co/config"
	"fx.prodigy9.co/ctrlc"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/internal/buildlog"
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
	cfg, err := conf.Load(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	modname, err := promptModule(cfg, args)
	if err != nil {
		buildlog.Fatalln(err)
	}

	eng := engine.New(fxconfig.Configure())
	defer eng.Close()

	ctx := engine.NewContext(context.Background(), eng)
	results, err := engine.Build(ctx, cfg, []string{modname}, newProgress())
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
	if custom := strings.TrimSpace(previewCmd); custom != "" {
		startArgs = strings.Fields(custom)
	}

	fromFlag, fromConfig := previewPort, result.Unit.Port
	port := fromFlag
	if port == 0 {
		port = fromConfig
	}
	if port == 0 {
		buildlog.Fatalln(errors.New("specify preview port with --port or port= key in platform.toml"))
	}
	if port < 1000 {
		buildlog.Fatalln(fmt.Errorf("preview port %d is reserved; use a port >= 1000", port))
	}

	container := result.Container.
		WithExposedPort(port).
		WithExec(startArgs).
		AsService()

	tunnel := result.Client().Host().Tunnel(container, dagger.HostTunnelOpts{
		Native: true,
	})

	ctrlc.Do(func() {
		if _, err := tunnel.Stop(ctx); err != nil {
			buildlog.Fatalln(err)
		}
		os.Exit(0)
	})
	tunnel, err = tunnel.Start(ctx)
	if err != nil {
		buildlog.Fatalln(err)
	}

	addr, err := tunnel.Endpoint(ctx, dagger.ServiceEndpointOpts{
		Port: previewPort,
	})
	if err != nil {
		buildlog.Fatalln(err)
	}

	buildlog.HTTPServing(addr)
}
