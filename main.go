package main

import (
	"log"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/cmd"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "platform",
		Short: "PRODIGY9 platform swiss army knife",
	}

	rootCmd.AddCommand(
		cmd.BuildCmd,
		cmd.ConfigureCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
