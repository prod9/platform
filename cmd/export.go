package cmd

import (
	"context"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal/plog"
	"platform.prodigy9.co/project"
)

var ExportCmd = &cobra.Command{
	Use:   "export [modules...]",
	Short: "Builds and exports the container to a docker-compatible format",
	Run:   runExport,
}

func runExport(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		plog.Fatalln(err)
	}

	jobs, err := builder.JobsFromArgs(cfg, args)
	if err != nil {
		plog.Fatalln(err)
	}

	sess, err := builder.NewSession(context.Background())
	if err != nil {
		plog.Fatalln(err)
	}
	defer sess.Close()

	results, err := builder.Build(sess, jobs...)
	if err != nil {
		plog.Fatalln(err)
	}

	for _, result := range results {
		if result.Err != nil {
			plog.Error(result.Err)
			continue
		}

		id, err := result.Container.ID(sess.Context())
		if err != nil {
			plog.Fatalln(err)
		}
		if len(id) > 16 {
			id = id[len(id)-16:]
		}

		outname := result.Job.Name + ".docker"
		_, err = result.Container.Export(sess.Context(), outname)
		if err != nil {
			plog.Fatalln(err)
		} else {
			plog.Image("export", outname, string(id))
		}
	}
}
