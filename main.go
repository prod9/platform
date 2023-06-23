package main

import (
	"log"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/cmd"
)

func main() {
	defer log.Println("exited.")

	rootCmd := &cobra.Command{
		Use:   "platform",
		Short: "PRODIGY9 platform swiss army knife",
	}

	rootCmd.AddCommand(
		cmd.BuildCmd,
		cmd.BootstrapCmd,
		cmd.ConfigureCmd,
		cmd.VanityCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
