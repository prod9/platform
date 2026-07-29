package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/framework/gitops"
	"platform.prodigy9.co/internal/termlog"
)

var renderOut string

var RenderCmd = &cobra.Command{
	Use:   "render [dir]",
	Short: "Render an infra CUE module's apps to a Kubernetes manifest tree",
	Run:   runRender,
}

func init() {
	RenderCmd.Flags().StringVar(&renderOut, "out", "k8s",
		"output directory for the rendered <component>/<file> tree")
}

func runRender(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	cfg, err := conf.Load(dir)
	if err != nil {
		termlog.Fatalln(err)
	}

	tree, err := gitops.Render(dir, gitops.RenderOptions{
		Vars: cfg.Vars,
	})
	if err != nil {
		termlog.Fatalln(err)
	}
	if err := tree.WriteDir(renderOut); err != nil {
		termlog.Fatalln(err)
	}

	for _, rel := range tree.Paths() {
		fmt.Fprintln(os.Stdout, filepath.Join(renderOut, rel))
	}
}
