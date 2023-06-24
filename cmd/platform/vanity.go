package main

import (
	"log"
	"net/http"

	"github.com/felixge/httpsnoop"
	"github.com/spf13/cobra"
	"go.jonnrb.io/vanity"
)

var vanityListenAddr string

var VanityCmd = &cobra.Command{
	Use:    "vanity",
	Short:  "Starts an HTTP server for redirecting go get to GitHub",
	Hidden: true,
	Run:    runVanityCmd,
}

func init() {
	VanityCmd.Flags().StringVar(
		&vanityListenAddr,
		"listen",
		"0.0.0.0:8000",
		"Specify the address for the HTTP server to listen on.",
	)
}

func runVanityCmd(cmd *cobra.Command, args []string) {
	handler := vanity.GitHubHandler("platform.prodigy9.co", "prod9", "platform", "https")
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(handler, w, r)
		log.Printf(
			"%s %s (code=%d dt=%s written=%d)\n",
			r.Method,
			r.URL,
			m.Code,
			m.Duration,
			m.Written,
		)
	})

	log.Println("serving", vanityListenAddr)
	if err := http.ListenAndServe(vanityListenAddr, wrapped); err != nil {
		log.Fatalln(err)
	}
}
