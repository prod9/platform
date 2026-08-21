package install

import (
	"net/http"
	"sync/atomic"

	"fx.prodigy9.co/httpserver/httperrors"
	"fx.prodigy9.co/httpserver/render"
)

// Gate is the process-local installation state shared by the composed fragments.
// Composition seeds it from the database; the claim action flips it only after its
// transaction commits.
type Gate struct{ installed atomic.Bool }

func NewGate(installed bool) *Gate {
	gate := &Gate{}
	gate.installed.Store(installed)
	return gate
}

func (g *Gate) Read() bool { return g.installed.Load() }
func (g *Gate) Flip()      { g.installed.Store(true) }

// ProductGate keeps product routes mounted in both compositions while the install
// record is still absent. API callers receive a stable state error; the UI can route
// itself into the installer from that response.
func ProductGate(gate *Gate) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if !gate.Read() {
				render.Error(resp, req, http.StatusConflict, ErrInstallationRequired)
				return
			}
			next.ServeHTTP(resp, req)
		})
	}
}

// InstallerGate hides installer routes after the install claim without requiring a
// process restart or changing the mounted fragment tree.
func InstallerGate(gate *Gate) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if gate.Read() {
				render.Error(resp, req, http.StatusNotFound, httperrors.ErrNotFound)
				return
			}
			next.ServeHTTP(resp, req)
		})
	}
}
