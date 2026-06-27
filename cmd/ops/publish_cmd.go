package ops

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/gitops"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
)

var publishTag string

var PublishCmd = &cobra.Command{
	Use:   "publish [dir]",
	Short: "Publish rendered infra manifests as an OCI config artifact",
	Run:   runPublish,
}

func init() {
	PublishCmd.Flags().StringVar(&publishTag, "tag", "",
		"override the moving per-env tag (defaults to [ops] tag, else \"latest\")")
}

func runPublish(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	cfg, err := project.Configure(dir)
	if err != nil {
		buildlog.Fatalln(err)
	}
	ref, err := cfg.Ops.Ref(publishTag)
	if err != nil {
		buildlog.Fatalln(err)
	}

	tree, err := gitops.Render(dir, gitops.RenderOptions{
		Vars: cfg.Ops.Vars,
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
