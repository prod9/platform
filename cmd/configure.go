package cmd

import (
	"log"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/config"
)

var ConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Parses and show effective configuration for the current directory",
	Run:   runConfigureCmd,
}

func runConfigureCmd(cmd *cobra.Command, args []string) {
	cfg, err := config.Configure(".")
	if err != nil {
		log.Fatalln(err)
	}

	if err := toml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
		log.Fatalln(err)
	}
}
