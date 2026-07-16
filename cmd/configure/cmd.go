package configure

import (
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
)

var Cmd = &cobra.Command{
	Use:   "configure",
	Short: "Parses and show effective configuration for the current directory",
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		buildlog.Fatalln(err)
	}

	if err := toml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
		buildlog.Fatalln(err)
	}
}
