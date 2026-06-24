package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/core/gitops"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
)

var (
	opsPublishImage string
	opsPublishTag   string
)

var OpsPublishCmd = &cobra.Command{
	Use:   "publish [dir]",
	Short: "Publish rendered infra manifests as an OCI config artifact",
	Run:   runOpsPublish,
}

func init() {
	OpsPublishCmd.Flags().StringVar(&opsPublishImage, "image", "",
		"image tag to inject into the module's @tag(image)")
	OpsPublishCmd.Flags().StringVar(&opsPublishTag, "tag", "",
		"override the moving per-env tag (defaults to [ops] tag, else \"latest\")")
}

func runOpsPublish(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	cfg, err := project.Configure(dir)
	if err != nil {
		buildlog.Fatalln(err)
	}
	ref, err := cfg.Ops.Ref(opsPublishTag)
	if err != nil {
		buildlog.Fatalln(err)
	}

	tree, err := gitops.Render(dir, gitops.RenderOptions{
		Image: opsPublishImage,
		Vars:  cfg.Ops.Vars,
	})
	if err != nil {
		buildlog.Fatalln(err)
	}

	target, tag, err := gitops.RemoteRepository(ref)
	if err != nil {
		buildlog.Fatalln(err)
	}

	desc, err := gitops.Publish(context.Background(), target, tag, tree)
	if err != nil {
		buildlog.Fatalln(err)
	}

	fmt.Fprintf(os.Stdout, "published %s@%s\n", ref, desc.Digest)
}
