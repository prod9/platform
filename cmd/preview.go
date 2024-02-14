package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dagger.io/dagger"
	"fx.prodigy9.co/cmd/prompts"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal/plog"
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
		plog.Fatalln(err)
	}

	jobs, err := builder.JobsFromArgs(cfg, args)
	if err != nil {
		plog.Fatalln(err)
	}

	if len(jobs) == 0 {
		plog.Fatalln(errors.New("no modules to preview"))
	}

	preview := jobs[0] // at least 1 by this point
	if len(jobs) > 1 {
		var names []string
		for _, job := range jobs {
			names = append(names, job.Name)
		}

		p := prompts.New(nil, args)
		modname := p.List("preview which module?", jobs[0].Name, names)

		for _, job := range jobs {
			if job.Name == modname {
				preview = job
			}
		}
		if preview == nil {
			plog.Fatalln(errors.New("no module to preview"))
		}
	}

	sess, err := builder.NewSession(context.Background())
	if err != nil {
		plog.Fatalln(err)
	}
	defer sess.Close()

	results, err := builder.Build(sess, preview)
	if err != nil {
		plog.Fatalln(err)
	}

	result := results[0]
	if result.Err != nil {
		plog.Fatalln(result.Err)
	}

	startArgs, err := result.Container.DefaultArgs(sess.Context())
	if err != nil {
		plog.Fatalln(err)
	}

	if previewPort <= 1 {
		previewPort = preview.Port
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
	tunnel, err = tunnel.Start(sess.Context())
	if err != nil {
		plog.Fatalln(err)
	}

	addr, err := tunnel.Endpoint(sess.Context(), dagger.ServiceEndpointOpts{
		Port: previewPort,
	})
	if err != nil {
		plog.Fatalln(err)
	}

	plog.HTTPServing(addr)

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		tunnel.Stop(sess.Context())
		os.Exit(0)
	}()

	_ = time.Sleep
	for {
		time.Sleep(24 * time.Hour)
	}
}
