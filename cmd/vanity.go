package cmd

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/felixge/httpsnoop"
	"github.com/spf13/cobra"
	"go.jonnrb.io/vanity"
	"platform.prodigy9.co/internal/plog"
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
		plog.HTTPRequest(
			r.Method,
			r.URL,
			m.Code,
			m.Duration,
			m.Written,
		)
	})

	srv := &http.Server{
		Addr:    vanityListenAddr,
		Handler: wrapped,
	}

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		srv.Close()
	}()

	plog.HTTPServing(vanityListenAddr)
	if err := srv.ListenAndServe(); err != nil {
		plog.Fatalln(err)
	}
}
