package cmd

import (
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/internal/plog"
	"platform.prodigy9.co/project"
)

var ConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Parses and show effective configuration for the current directory",
	Run:   runConfigureCmd,
}

func runConfigureCmd(cmd *cobra.Command, args []string) {
	cfg, err := project.Configure(".")
	if err != nil {
		plog.Fatalln(err)
	}

	if err := toml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
		plog.Fatalln(err)
	}
}
