package install

import (
	"context"
	"errors"
	"net/http"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
)

// InstallCtr serves the gated installer surface. The fragment stays mounted after
// claim; InstallerGate hides it with a 404.
type InstallCtr struct{}

var _ controllers.Interface = InstallCtr{}

func (c InstallCtr) Mount(cfg *config.Source, router chi.Router) error {
	router.Route("/api/install", func(r chi.Router) {
		r.Get("/", c.getState)
		r.Post("/migrations", c.runMigrations)
		r.Post("/server", c.saveServer)
		r.Post("/org", c.saveOrg)
		r.Post("/app", c.saveApp)
		r.Post("/credentials", c.saveCredentials)
		r.Post("/registry", c.saveRegistryToken)
		r.Post("/claim", c.claim)
	})
	return nil
}

func (c InstallCtr) saveServer(resp http.ResponseWriter, req *http.Request) {
	c.runUngated(resp, req, &SaveServer{})
}

func (c InstallCtr) saveOrg(resp http.ResponseWriter, req *http.Request) {
	c.runUngated(resp, req, &SaveOrg{})
}

func (c InstallCtr) saveApp(resp http.ResponseWriter, req *http.Request) {
	c.runUngated(resp, req, &SaveApp{})
}

func (c InstallCtr) saveCredentials(resp http.ResponseWriter, req *http.Request) {
	c.runUngated(resp, req, &SaveCredentials{})
}

func (c InstallCtr) saveRegistryToken(resp http.ResponseWriter, req *http.Request) {
	c.runUngated(resp, req, &SaveRegistryToken{})
}

func (c InstallCtr) runUngated(resp http.ResponseWriter, req *http.Request, action controllers.Action) {
	db := requestDB(req.Context())
	if db == nil {
		render.Error(resp, req, http.StatusServiceUnavailable, errNoDB)
		return
	}
	if err := controllers.ReadAction(req, action); err != nil {
		render.Error(resp, req, http.StatusBadRequest, err)
		return
	}

	ctx := req.Context()
	if err := action.Execute(ctx, nil); err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return
	}
	render.JSON(resp, req, GetState(ctx, db))
}

func (c InstallCtr) claim(resp http.ResponseWriter, req *http.Request) {
	user, ok := auth.RequireUser(resp, req)
	if !ok {
		return
	}

	action := &Claim{PlatformUserID: user.ID, PlatformUserLogin: user.Name}
	if err := controllers.ReadAction(req, action); err != nil {
		render.Error(resp, req, http.StatusBadRequest, err)
		return
	}

	ctx := req.Context()
	err := action.Execute(ctx, nil)
	if errors.Is(err, ErrAlreadyInstalled) {
		render.Error(resp, req, http.StatusConflict, err)
		return
	}
	if errors.Is(err, errNotOrgOwner) {
		render.Error(resp, req, http.StatusForbidden, err)
		return
	}
	if errors.Is(err, github.ErrNoApp) {
		render.Error(resp, req, http.StatusServiceUnavailable, err)
		return
	}
	if err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return
	}

	render.JSON(resp, req, GetState(ctx, requestDB(ctx)))
}

func (c InstallCtr) getState(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	render.JSON(resp, req, GetState(ctx, requestDB(ctx)))
}

func (c InstallCtr) runMigrations(resp http.ResponseWriter, req *http.Request) {
	db := requestDB(req.Context())
	if db == nil {
		render.Error(resp, req, http.StatusServiceUnavailable, errNoDB)
		return
	}

	ctx := req.Context()
	action := &RunMigrations{}
	if err := action.Execute(ctx, nil); err != nil {
		render.Error(resp, req, http.StatusInternalServerError, err)
		return
	}
	render.JSON(resp, req, GetState(ctx, db))
}

func requestDB(ctx context.Context) *sqlx.DB {
	db, ok := data.LookupFromContext(ctx)
	if !ok {
		return nil
	}
	return db
}
