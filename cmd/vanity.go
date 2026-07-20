package cmd

import (
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"fx.prodigy9.co/fxlog"
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
		fxlog.Log("request",
			fxlog.String("method", r.Method),
			fxlog.String("url", r.URL.Path),
			fxlog.Int("code", m.Code),
			fxlog.Duration("d", m.Duration),
			fxlog.Int64("written", m.Written),
		)
	})

	srv := &http.Server{
		Addr:    vanityListenAddr,
		Handler: wrapped,
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		if err := srv.Close(); err != nil {
			fxlog.Fatal(err)
		}
	}()

	fxlog.Log("serving", fxlog.String("addr", vanityListenAddr))
	// Close makes ListenAndServe return ErrServerClosed — that is the shutdown path, not a fault.
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fxlog.Fatal(err)
	}
}
