package vanity

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"fx.prodigy9.co/fxlog"
	"github.com/felixge/httpsnoop"
	"github.com/spf13/cobra"
	govanity "go.jonnrb.io/vanity"
)

var vanityListenAddr string

var Cmd = &cobra.Command{
	Use:    "vanity",
	Short:  "Starts an HTTP server for redirecting go get to GitHub",
	Hidden: true,
	Run:    run,
}

func init() {
	Cmd.Flags().StringVar(
		&vanityListenAddr,
		"listen",
		"0.0.0.0:8000",
		"Specify the address for the HTTP server to listen on.",
	)
}

func run(cmd *cobra.Command, args []string) {
	handler := govanity.GitHubHandler("platform.prodigy9.co", "prod9", "platform", "https")
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
		srv.Close()
	}()

	fxlog.Log("serving", fxlog.String("addr", vanityListenAddr))
	if err := srv.ListenAndServe(); err != nil {
		fxlog.Fatal(err)
	}
}
