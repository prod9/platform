package main

import (
	"os"

	"platform.prodigy9.co/cmd"
	"platform.prodigy9.co/internal/termlog"
)

func main() {
	code := cmd.Execute()
	termlog.Event("main", "done")
	os.Exit(code)
}
