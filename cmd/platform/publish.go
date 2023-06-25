package main

import (
	// "log"

	"log"
	"os"

	"fx.prodigy9.co/cmd/prompts"
	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"platform.prodigy9.co/config"
	"platform.prodigy9.co/releases"
	// "platform.prodigy9.co/builder"
	// "platform.prodigy9.co/config"
)

var PublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish the container image",
	Run:   runPublish,
}

func runPublish(cmd *cobra.Command, args []string) {
	cfg, err := config.Configure(".")
	if err != nil {
		log.Fatalln(err)
	}

	strat, err := releases.FindStrategy(cfg.Strategy)
	if err != nil {
		log.Fatalln(err)
	}

	opts := &releases.Options{}
	if len(args) > 0 {
		opts.Name = args[0]
	}
	rel, err := strat.Recover(cfg, opts)
	if err != nil {
		log.Fatalln(err)
	}

	if err = toml.NewEncoder(os.Stdout).Encode(rel); err != nil {
		log.Fatalln(err)
	}
	sess := prompts.New(nil, nil)
	if !sess.YesNo("publish this release?") {
		return
	}

	// TODO: Actually build and push
	// 1. tag env
	// 2. push env

	// var jobs []*builder.Job
	// if len(args) == 0 {
	// 	for modname, mod := range cfg.Modules {
	// 		if job, err := builder.JobFromModule(cfg, modname, mod); err != nil {
	// 			log.Fatalln(err)
	// 		} else {
	// 			jobs = append(jobs, job)
	// 		}
	// 	}

	// } else {
	// 	for len(args) > 0 {
	// 		modname := args[0]
	// 		args = args[1:]

	// 		if mod, ok := cfg.Modules[modname]; !ok {
	// 			log.Fatalln("unknown module `" + modname + "`")
	// 		} else if job, err := builder.JobFromModule(cfg, modname, mod); err != nil {
	// 			log.Fatalln(err)
	// 		} else {
	// 			jobs = append(jobs, job)
	// 		}
	// 	}
	// }

	// if err := builder.Build(cfg, jobs...); err != nil {
	// 	log.Fatalln(err)
	// }

}
