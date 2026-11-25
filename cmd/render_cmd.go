package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	fxconfig "fx.prodigy9.co/config"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/framework/gitops"
	"platform.prodigy9.co/internal/termlog"
	"platform.prodigy9.co/rocks"
)

var renderOut string

var RenderCmd = &cobra.Command{
	Use:   "render [dir]",
	Short: "Render an infra repo's Lua apps to a Kubernetes manifest tree",
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
	env := fxconfig.Configure()
	fetch := rocks.FetchPins{
		Pins:     cfg.Rocks,
		Username: fxconfig.Get(env, engine.RegistryUsernameConfig),
		Password: fxconfig.Get(env, engine.RegistryPasswordConfig),
	}

	rockDirs, err := fetch.Run(context.Background())
	if err != nil {
		termlog.Fatalln(err)
	}
	tree, err := gitops.Render(dir, cfg.Vars, rockDirs)
	if err != nil {
		termlog.Fatalln(err)
	}
	err = tree.WriteDir(renderOut)
	if err != nil {
		termlog.Fatalln(err)
	}

	for _, rel := range tree.Paths() {
		fmt.Fprintln(os.Stdout, filepath.Join(renderOut, rel))
	}
}
