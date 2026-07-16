package main

import (
	"platform.prodigy9.co/cmd"
	"platform.prodigy9.co/internal/buildlog"
)

func main() {
	defer buildlog.Event("exited")
	if err := cmd.Execute(); err != nil {
		buildlog.Fatalln(err)
	}
}
