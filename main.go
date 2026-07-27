package main

import (
	"os"

	"platform.prodigy9.co/cmd"
	"platform.prodigy9.co/internal/buildlog"
)

func main() {
	code := cmd.Execute()
	buildlog.Event("main", "done")
	os.Exit(code)
}
