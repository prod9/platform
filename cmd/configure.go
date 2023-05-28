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
	Run:   runConfigure,
}

func runConfigure(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	cfg, err := config.Configure(wd)
	if err != nil {
		log.Fatalln(err)
	}

	if err := toml.NewEncoder(os.Stdout).Encode(cfg); err != nil {
		log.Fatalln(err)
	}
}
