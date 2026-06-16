package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/core/gitops"
	"platform.prodigy9.co/internal/plog"
)

var errMissingTo = errors.New("ops publish: --to oci://host/repo:tag is required")

var (
	opsPublishImage string
	opsPublishTo    string
)

var OpsPublishCmd = &cobra.Command{
	Use:   "publish [dir]",
	Short: "Publish rendered infra manifests as an OCI config artifact",
	Run:   runOpsPublish,
}

func init() {
	OpsPublishCmd.Flags().StringVar(&opsPublishImage, "image", "",
		"image tag to inject into the module's @tag(image)")
	OpsPublishCmd.Flags().StringVar(&opsPublishTo, "to", "",
		"OCI reference to push to, e.g. oci://ghcr.io/org/infra:staging")
}

func runOpsPublish(cmd *cobra.Command, args []string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	if opsPublishTo == "" {
		plog.Fatalln(errMissingTo)
	}

	manifests, err := gitops.Render(dir, opsPublishImage)
	if err != nil {
		plog.Fatalln(err)
	}

	target, tag, err := gitops.RemoteRepository(opsPublishTo)
	if err != nil {
		plog.Fatalln(err)
	}

	desc, err := gitops.Publish(context.Background(), target, tag, manifests)
	if err != nil {
		plog.Fatalln(err)
	}

	fmt.Fprintf(os.Stdout, "published %s@%s\n", opsPublishTo, desc.Digest)
}
