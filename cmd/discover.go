package cmd

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
)

var DiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover the project type",
	Run:   runDiscover,
}

func runDiscover(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	if len(args) == 0 {
		args = append(args, wd)
	}
	for idx, arg := range args {
		if !filepath.IsAbs(arg) {
			if arg, err = filepath.Abs(filepath.Join(wd, arg)); err != nil {
				log.Fatalln(err)
			} else {
				args[idx] = arg
			}
		}
	}

	for _, arg := range args {
		log.Println("discovering:", arg)
		mods, err := builder.Discover(arg)
		if err != nil {
			log.Fatalln(err)
		}

		for name, builder := range mods {
			log.Println("discovered:", name, "=>", builder.Name())
		}
	}
}
