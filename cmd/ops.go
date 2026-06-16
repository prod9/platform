package cmd

import "github.com/spf13/cobra"

// OpsCmd groups the GitOps delivery spine — rendering infra CUE modules to
// manifests and publishing them as OCI config artifacts. Distinct from the
// top-level container-release `publish`.
var OpsCmd = &cobra.Command{
	Use:   "ops",
	Short: "GitOps delivery: render and publish infra manifests",
}

func init() {
	OpsCmd.AddCommand(RenderCmd, OpsPublishCmd)
}
