package main

import (
	"log"

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
		VanityCmd,
	)
}

func main() {
	defer log.Println("exited.")
	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
