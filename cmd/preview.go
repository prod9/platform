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
	"platform.prodigy9.co/internal/termlog"
)

var PreviewCmd = &cobra.Command{
	Use:   "preview [modules...]",
	Short: "Builds and starts up the container",
	RunE:  runPreview,
}

var (
	previewPort int
	previewCmd  string
)

func init() {
	PreviewCmd.Flags().IntVarP(&previewPort, "port", "p", 0, "Binds port for preview")
	PreviewCmd.Flags().StringVarP(&previewCmd, "exec", "e", "", "Specify custom command to run")
}

func runPreview(cmd *cobra.Command, args []string) error {
	cfg, err := conf.Load(".")
	if err != nil {
		return err
	}

	modname, err := promptModule(cfg, args)
	if err != nil {
		return err
	}

	ctx := fxconfig.NewContext(context.Background(), fxconfig.Configure())
	sess := engine.NewSession(ctx)
	defer sess.Close()

	results, err := sess.Build(ctx, cfg, []string{modname}, newObserver())
	if err != nil {
		return err
	}

	result := results[0]
	if result.Err != nil {
		return result.Err
	}

	container := result.UnsafeContainer()

	startArgs, err := container.DefaultArgs(ctx)
	if err != nil {
		return err
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
		return errors.New("specify preview port with --port or port= key in platform.toml")
	}
	if port < 1000 {
		return fmt.Errorf("preview port %d is reserved; use a port >= 1000", port)
	}

	service := container.
		WithExposedPort(port).
		WithExec(startArgs).
		AsService()

	ctrlc.Do(func() { os.Exit(0) })

	// Up forwards the host port from the service itself and blocks until interrupted, so the
	// address is known before it is serving rather than read back off a tunnel handle.
	termlog.HTTPServing(fmt.Sprintf("http://localhost:%d", port))
	return service.Up(ctx, dagger.ServiceUpOpts{
		Ports: []dagger.PortForward{{Frontend: port, Backend: port}},
	})
}
