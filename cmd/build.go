package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/build"
	"platform.prodigy9.co/config"
)

var BuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Builds current directory",
	Run:   runBuild,
}

func runBuild(cmd *cobra.Command, args []string) {
	defer log.Println("exited.")

	cfg, err := config.Configure(".")
	if err != nil {
		log.Fatalln(err)
	}

	if len(args) > 0 {
		for len(args) > 0 {
			modname := args[0]
			args = args[1:]

			mod, ok := cfg.Modules[modname]
			if !ok {
				log.Fatalln("unknown module `" + modname + "`")
			}

			log.Println("building", modname)
			job := build.JobFromModule(cfg, modname, mod)
			if err := build.Build(job); err != nil {
				log.Fatalln(err)
			}
		}

	} else {
		for modname, mod := range cfg.Modules {
			log.Println("building", modname)
			job := build.JobFromModule(cfg, modname, mod)
			if err := build.Build(job); err != nil {
				log.Fatalln(err)
			}
		}
	}
}
