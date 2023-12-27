package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/internal/plog"
)

var DiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover the project type",
	Run:   runDiscover,
}

func runDiscover(cmd *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		plog.Fatalln(err)
	}

	if len(args) == 0 {
		args = append(args, wd)
	}
	for idx, arg := range args {
		if !filepath.IsAbs(arg) {
			if arg, err = filepath.Abs(filepath.Join(wd, arg)); err != nil {
				plog.Fatalln(err)
			} else {
				args[idx] = arg
			}
		}
	}

	for _, arg := range args {
		mods, err := builder.Discover(arg)
		if err != nil {
			plog.Fatalln(err)
		}

		for name, builder := range mods {
			plog.Dir("discover", name, builder.Name())
		}
	}
}
