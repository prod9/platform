package install

import (
	"net/http"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/httperrors"
	"fx.prodigy9.co/httpserver/render"
)

// ProductGate exposes product routes only after the durable claim exists.
func ProductGate(*config.Source) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			installed, ok := installedForRequest(resp, req)
			if !ok {
				return
			}
			if !installed {
				render.Error(resp, req, http.StatusConflict, ErrInstallationRequired)
				return
			}
			next.ServeHTTP(resp, req)
		})
	}
}

// InstallerGate hides installer routes after the durable claim exists.
func InstallerGate(*config.Source) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			installed, ok := installedForRequest(resp, req)
			if !ok {
				return
			}
			if installed {
				render.Error(resp, req, http.StatusNotFound, httperrors.ErrNotFound)
				return
			}
			next.ServeHTTP(resp, req)
		})
	}
}

func installedForRequest(resp http.ResponseWriter, req *http.Request) (bool, bool) {
	db, ok := data.LookupFromContext(req.Context())
	if !ok {
		render.Error(resp, req, http.StatusInternalServerError, errNoDB)
		return false, false
	}

	installed, err := IsInstalled(req.Context(), db)
	if err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return false, false
	}
	return installed, true
}
