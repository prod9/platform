package cmd

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/spf13/cobra"
	"platform.prodigy9.co/config"
)

var GitHubCmd = &cobra.Command{
	Use:   "github",
	Short: "Installs a YAML manifests for triggering GitHub action builds.",
	Run:   runGitHubCmd,
}

var ghaTemplate = `
name: platform

on:
  push:
    branches:
      - main

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v3
        with:
          ref: main
      - name: Install Go
        uses: actions/setup-go@v4
        with:
         go-version: {{.GoVersion}}
      - run: go version
      - name: Self-Build
        uses: dagger/dagger-for-github@v4
        with:
          cmds: do test
`

func runGitHubCmd(cmd *cobra.Command, args []string) {
	cfg, err := config.Configure(".")
	if err != nil {
		log.Fatalln(err)
	}

	yamlPath := filepath.Join(
		filepath.Dir(cfg.ConfigPath),
		".github/workflows/platform.yaml",
	)
	if err := os.MkdirAll(filepath.Dir(yamlPath), 0755); err != nil {
		log.Fatalln(err)
	}

	file, err := os.Create(yamlPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	tmpl := template.Must(template.New("").Parse(ghaTemplate))
	err = tmpl.Execute(file, map[string]string{
		"GoVersion": runtime.Version()[2:],
	})
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("wrote", yamlPath)
}
