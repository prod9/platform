package main

import (
	// "log"

	"github.com/spf13/cobra"
	// "platform.prodigy9.co/builder"
	// "platform.prodigy9.co/config"
)

var PublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish the container image",
	Run:   runPublish,
}

// TODO: DRY with build cmd
func runPublish(cmd *cobra.Command, args []string) {
	// TODO:
	// cfg, err := config.Configure(".")
	// if err != nil {
	// 	log.Fatalln(err)
	// }

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
