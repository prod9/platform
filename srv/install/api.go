package install

import (
	"context"
	"errors"
	"net/http"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/migrate"
)

var (
	errNoDB        = errors.New("install: no database configured")
	errNotOrgOwner = errors.New("install: session user is not an owner of the installation's org")
)

// StateCtr serves the gated installer surface: the ordered install-state read, the
// migrations remediation, and the org-owner claim. Boot mounts it only while the server is not completely
// installed, so its absence (a 404 on GET /api/install) is the SPA's "installed" signal.
//
// It carries the boot DB handle (possibly nil) and the merged migration set explicitly
// rather than reading them from AddDataContext: the installer runs before that middleware
// is wired (it is added only once a DB exists), so it cannot rely on request-scoped data.
type StateCtr struct {
	DB     *sqlx.DB
	Merged migrator.Source
}

var _ controllers.Interface = StateCtr{}

func (c StateCtr) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/api/install", c.getState)
	router.Post("/api/install/migrations", c.runMigrations)
	router.Post("/api/install/credentials", c.saveCredentials)
	router.Post("/api/install/claim", c.claim)
	router.Post("/api/install/flux", c.setupFlux)
	return nil
}

// setupFlux is the delivery step (POST /api/install/flux): session-gated like the
// claim — it mutates the org through the App, and by the time it is reachable the
// install record exists (docs/spec/installation.md, the flux-setup step).
func (c StateCtr) setupFlux(resp http.ResponseWriter, req *http.Request) {
	if _, ok := auth.RequireUser(resp, req); !ok {
		return
	}

	action := &SetupFlux{}
	if err := controllers.ReadAction(req, action); err != nil {
		render.Error(resp, req, 400, err)
		return
	}

	ctx := req.Context()
	if err := action.Execute(ctx, nil); errors.Is(err, ErrNotInstalled) {
		render.Error(resp, req, 409, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	render.JSON(resp, req, GetState(ctx, c.DB, c.Merged))
}

// saveCredentials is the wizard's credential step (POST /api/install/credentials).
// Deliberately ungated: no session can exist before the credentials enable login —
// the same accepted posture as the first-install migrations button
// (docs/spec/installation.md).
func (c StateCtr) saveCredentials(resp http.ResponseWriter, req *http.Request) {
	if c.DB == nil {
		render.Error(resp, req, 503, errNoDB)
		return
	}

	action := &SaveCredentials{}
	if err := controllers.ReadAction(req, action); err != nil {
		render.Error(resp, req, 400, err)
		return
	}

	ctx := req.Context()
	if err := action.Execute(ctx, nil); err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	render.JSON(resp, req, GetState(ctx, c.DB, c.Merged))
}

// claim is the org-owner claim (POST /api/install/claim): the session user proves
// active org ownership of the installation's org via the App API, and the install.*
// settings are filled. The webui install page posts here after GitHub's Setup URL
// redirect lands on it.
func (c StateCtr) claim(resp http.ResponseWriter, req *http.Request) {
	user, ok := auth.RequireUser(resp, req)
	if !ok {
		return
	}

	action := &ClaimInstall{UserID: user.ID, UserLogin: user.Name}
	if err := controllers.ReadAction(req, action); err != nil {
		render.Error(resp, req, 400, err)
		return
	}

	ctx := req.Context()
	client, err := github.NewClient(ctx)
	if errors.Is(err, github.ErrNoApp) {
		render.Error(resp, req, 503, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	org, owner, err := claimedOrgOwner(ctx, client, action.InstallationID, user.Name)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	} else if !owner {
		render.Error(resp, req, 403, errNotOrgOwner)
		return
	}

	action.OrgID, action.OrgLogin = org.ID, org.Login
	if err := action.Execute(ctx, nil); errors.Is(err, ErrAlreadyInstalled) {
		render.Error(resp, req, 409, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	render.JSON(resp, req, GetState(ctx, c.DB, c.Merged))
}

// claimedOrgOwner resolves the installation's org and answers whether user is an
// active owner of it — the App-identity reads behind the claim, kept apart from the
// handler's status-code branching.
func claimedOrgOwner(ctx context.Context, client *github.Client, installationID int64, user string) (*github.Org, bool, error) {
	org, err := client.InstallationOrg(ctx, installationID)
	if err != nil {
		return nil, false, err
	}

	token, err := client.InstallationToken(ctx, installationID)
	if err != nil {
		return nil, false, err
	}

	owner, err := client.IsOrgOwner(ctx, token, org.Login, user)
	if err != nil {
		return nil, false, err
	}
	return org, owner, nil
}

func (c StateCtr) getState(resp http.ResponseWriter, req *http.Request) {
	render.JSON(resp, req, GetState(req.Context(), c.DB, c.Merged))
}

func (c StateCtr) runMigrations(resp http.ResponseWriter, req *http.Request) {
	if c.DB == nil {
		render.Error(resp, req, 503, errNoDB)
		return
	}

	ctx := req.Context()
	if err := migrate.Run(ctx, c.DB, c.Merged); err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	render.JSON(resp, req, GetState(ctx, c.DB, c.Merged))
}
