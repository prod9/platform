package srv

import (
	"errors"
	"net/http"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/httperrors"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
)

// API serves the platform API under /api/ — the endpoints the web UI consumes. Wire
// structs are hand-written per handler; there is deliberately no shared api/ contract
// package (spec §No api/ contract layer).
type API struct{}

var _ controllers.Interface = API{}

func (API) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/api/health", health)
	router.Get("/api/me", me)
	router.Get("/api/builds", listBuilds)
	return nil
}

func health(resp http.ResponseWriter, req *http.Request) {
	render.JSON(resp, req, struct {
		Time time.Time `json:"time"`
	}{time.Now()})
}

// currentUser resolves the platform session cookie to its unexpired session's user;
// anything short of that is ErrNoSession.
func currentUser(req *http.Request) (*User, error) {
	cookie, err := req.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, ErrNoSession
	}

	user := &User{}
	err = data.Get(req.Context(), user, `
		SELECT users.* FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = $1 AND sessions.expires_at > now()`,
		hashSessionToken(cookie.Value))
	if data.IsNoRows(err) {
		return nil, ErrNoSession
	} else if err != nil {
		return nil, err
	}
	return user, nil
}

func me(resp http.ResponseWriter, req *http.Request) {
	user, err := currentUser(req)
	if errors.Is(err, ErrNoSession) {
		render.Error(resp, req, 401, httperrors.ErrUnauthorized)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	render.JSON(resp, req, struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}{user.ID, user.Name})
}

type buildResponse struct {
	ID        int64     `json:"id"`
	Owner     string    `json:"owner"`
	Repo      string    `json:"repo"`
	CloneURL  string    `json:"clone_url"`
	Tag       string    `json:"tag"`
	SHA       string    `json:"sha"`
	Status    string    `json:"status"`
	Error     string    `json:"error"`
	Image     string    `json:"image"`
	Digest    string    `json:"digest"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func listBuilds(resp http.ResponseWriter, req *http.Request) {
	if _, err := currentUser(req); errors.Is(err, ErrNoSession) {
		render.Error(resp, req, 401, httperrors.ErrUnauthorized)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	builds := []*Build{}
	err := data.Select(req.Context(), &builds, `
		SELECT * FROM builds ORDER BY id DESC LIMIT 50`)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	out := make([]buildResponse, len(builds))
	for i, build := range builds {
		out[i] = buildResponse{
			ID:        build.ID,
			Owner:     build.Owner,
			Repo:      build.Repo,
			CloneURL:  build.CloneURL,
			Tag:       build.Tag,
			SHA:       build.SHA,
			Status:    build.Status,
			Error:     build.Error,
			Image:     build.Image,
			Digest:    build.Digest,
			CreatedAt: build.CreatedAt,
			UpdatedAt: build.UpdatedAt,
		}
	}
	render.JSON(resp, req, out)
}
