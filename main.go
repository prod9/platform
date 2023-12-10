package main

import (
	"io/ioutil"
	"log"
	"os"

	fxcmd "fx.prodigy9.co/cmd"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/cmd"
)

var rootCmd = &cobra.Command{
	Use:   "platform",
	Short: "PRODIGY9 platform swiss army knife",
}

var quiet int

func init() {
	rootCmd.PersistentFlags().CountVarP(&quiet, "quiet", "q", "less verbose logging")

	rootCmd.AddCommand(
		cmd.BootstrapCmd,
		cmd.BuildCmd,
		cmd.ConfigureCmd,
		cmd.DeployCmd,
		cmd.DiscoverCmd,
		cmd.PublishCmd,
		cmd.ReleaseCmd,
		cmd.VanityCmd,

		fxcmd.PrintConfigCmd,
	)
}

func main() {
	defer log.Println("exited.")
	if err := rootCmd.ParseFlags(os.Args); err != nil {
		log.Fatalln(err)
	}

	if quiet >= 2 {
		log.SetOutput(ioutil.Discard)
	} else if quiet >= 1 {
		log.SetFlags(0) // reduce
	} else {
		// useful in CI to have date and time stamps
		log.SetFlags(log.Lshortfile + log.Ltime + log.Ldate)
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
