package ops

import "github.com/spf13/cobra"

// Cmd groups the GitOps delivery spine: initialise an infra repo, render its CUE
// apps to manifests, and publish them as OCI config artifacts. Added to the root
// as a single `ops` subcommand.
var Cmd = &cobra.Command{
	Use:   "ops",
	Short: "GitOps delivery: init, render, and publish infra manifests",
}

func init() {
	Cmd.AddCommand(InitCmd, RenderCmd, PublishCmd)
}
