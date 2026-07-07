package ops

import "github.com/spf13/cobra"

// Cmd groups the GitOps delivery spine: render an infra repo's CUE apps to manifests
// and publish them. Added to the root as a single `ops` subcommand. (Infra-repo
// scaffolding is the top-level `init`; these two collapse into top-level `render`/
// `publish` once infra becomes a builder.)
var Cmd = &cobra.Command{
	Use:   "ops",
	Short: "GitOps delivery: render and publish infra manifests",
}

func init() {
	Cmd.AddCommand(RenderCmd, PublishCmd)
}
