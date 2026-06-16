package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/core/gitops"
	"platform.prodigy9.co/internal/plog"
)

var renderImage string

var RenderCmd = &cobra.Command{
	Use:   "render [dir]",
	Short: "Render an infra CUE module to Kubernetes manifests",
	Run:   runRender,
}

func init() {
	RenderCmd.Flags().StringVar(&renderImage, "image", "",
		"image tag to inject into the module's @tag(image)")
}

func runRender(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	manifests, err := gitops.Render(dir, renderImage)
	if err != nil {
		plog.Fatalln(err)
	}

	fmt.Fprintln(os.Stdout, manifests)
}
