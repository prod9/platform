package engines

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/srv/auth"
)

type EngineCtr struct{}

func (EngineCtr) Mount(_ *config.Source, router chi.Router) error {
	router.Get("/api/engines", list)
	router.Get("/api/engines/{addr}", get)
	return nil
}

func list(resp http.ResponseWriter, req *http.Request) {
	if _, ok := auth.RequireUser(resp, req); !ok {
		return
	}
	view, err := ReadFleet(req)
	if err != nil {
		fxlog.Error(err)
		render.Error(resp, req, 500, errors.New("engines: observation unavailable"))
		return
	}
	render.JSON(resp, req, view)
}

func get(resp http.ResponseWriter, req *http.Request) {
	address, err := url.PathUnescape(chi.URLParam(req, "addr"))
	if err != nil || strings.Contains(address, "/") {
		render.Error(resp, req, http.StatusNotFound, ErrUnknownEndpoint)
		return
	}
	view, ok := RequireRead(resp, req, address)
	if ok {
		render.JSON(resp, req, view)
	}
}

func RequireRead(resp http.ResponseWriter, req *http.Request, address string) (*InstanceView, bool) {
	if _, ok := auth.RequireUser(resp, req); !ok {
		return nil, false
	}
	view, err := ReadInstance(req, address)
	if errors.Is(err, ErrUnknownEndpoint) {
		render.Error(resp, req, 404, err)
		return nil, false
	}
	if err != nil {
		fxlog.Error(err)
		render.Error(resp, req, 500, errors.New("engines: observation unavailable"))
		return nil, false
	}
	return view, true
}
