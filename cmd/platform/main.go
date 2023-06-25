package main

import (
	"log"

	fxcmd "fx.prodigy9.co/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "platform",
	Short: "PRODIGY9 platform swiss army knife",
}

func init() {
	rootCmd.AddCommand(
		BootstrapCmd,
		BuildCmd,
		ConfigureCmd,
		DeployCmd,
		PublishCmd,
		ReleaseCmd,
		VanityCmd,

		fxcmd.PrintConfigCmd,
	)
}

func main() {
	defer log.Println("exited.")
	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
