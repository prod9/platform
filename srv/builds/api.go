package builds

import (
	"net/http"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/srv/auth"
)

// APICtr serves the fragment's slice of the UI API. Wire structs are hand-written
// per handler; there is deliberately no shared api/ contract package (spec §No api/
// contract layer).
type APICtr struct{}

var _ controllers.Interface = APICtr{}

func (APICtr) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/api/builds", list)
	return nil
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

func list(resp http.ResponseWriter, req *http.Request) {
	if _, ok := auth.RequireUser(resp, req); !ok {
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
