package cmd

import (
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/internal/termlog"
)

var ConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Parses and show effective configuration for the current directory",
	Run:   runConfigureCmd,
}

func runConfigureCmd(cmd *cobra.Command, args []string) {
	cfg, err := conf.Load(".")
	if err != nil {
		termlog.Fatalln(err)
	}

	if err := toml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
		termlog.Fatalln(err)
	}
}
