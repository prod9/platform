package srv

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
)

// maxWebhookBody caps webhook reads; real push payloads are a few KB.
const maxWebhookBody = 1 << 20

var errBadWebhookSignature = errors.New("srv: invalid webhook signature")

// Webhooks ingests GitHub webhook deliveries: it verifies the App webhook signature
// and records a queued build for each pushed version tag; runQueuedBuilds consumes the
// queue.
type Webhooks struct{}

var _ controllers.Interface = Webhooks{}

func (Webhooks) Mount(cfg *config.Source, router chi.Router) error {
	router.Post("/api/webhooks/github", githubWebhook)
	return nil
}

func githubWebhook(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	body, err := io.ReadAll(io.LimitReader(req.Body, maxWebhookBody))
	if err != nil {
		render.Error(resp, req, 400, err)
		return
	}

	app, err := loadGitHubApp(ctx)
	if errors.Is(err, ErrNoGitHubApp) {
		render.Error(resp, req, 503, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	if !verifyWebhookSignature(app.WebhookSecret, body, req.Header.Get("X-Hub-Signature-256")) {
		render.Error(resp, req, 401, errBadWebhookSignature)
		return
	}

	if req.Header.Get("X-GitHub-Event") != "push" {
		render.JSON(resp, req, webhookReceipt{Status: "ignored"})
		return
	}

	ev := pushEvent{}
	if err := json.Unmarshal(body, &ev); err != nil {
		render.Error(resp, req, 400, err)
		return
	}

	create := buildForPush(ev)
	if create == nil {
		render.JSON(resp, req, webhookReceipt{Status: "ignored"})
		return
	}
	if err := create.Execute(ctx, nil); err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	renderAccepted(resp, req, webhookReceipt{Status: "queued"})
}

// verifyWebhookSignature checks GitHub's X-Hub-Signature-256 header (sha256=<hex>)
// against an HMAC-SHA256 of the raw body; hmac.Equal keeps the compare constant-time.
func verifyWebhookSignature(secret string, body []byte, header string) bool {
	hexSig, hasPrefix := strings.CutPrefix(header, "sha256=")
	if !hasPrefix {
		return false
	}
	sig, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(sig, mac.Sum(nil))
}

// pushEvent is the subset of GitHub's push payload the tag-watch cares about.
type pushEvent struct {
	Ref        string         `json:"ref"`
	Deleted    bool           `json:"deleted"`
	After      string         `json:"after"`
	Repository pushRepository `json:"repository"`
}

type pushRepository struct {
	Name     string    `json:"name"`
	CloneURL string    `json:"clone_url"`
	Owner    pushOwner `json:"owner"`
}

type pushOwner struct {
	Login string `json:"login"`
}

// buildForPush decides whether a push warrants a build: only a still-existing version
// tag (refs/tags/v*) does — rolling repos cut no tags and stay CLI-published (see the
// delivery-verbs ADR). Everything else returns nil.
func buildForPush(ev pushEvent) *CreateBuild {
	tag, isTag := strings.CutPrefix(ev.Ref, "refs/tags/")
	if !isTag || !strings.HasPrefix(tag, "v") || ev.Deleted {
		return nil
	}

	return &CreateBuild{
		Owner:    ev.Repository.Owner.Login,
		Repo:     ev.Repository.Name,
		CloneURL: ev.Repository.CloneURL,
		Tag:      tag,
		SHA:      ev.After,
	}
}

// CreateBuild records a queued build row for a pushed version tag.
type CreateBuild struct {
	Owner    string
	Repo     string
	CloneURL string
	Tag      string
	SHA      string
}

func (c *CreateBuild) Execute(ctx context.Context, out any) error {
	return data.Exec(ctx, `
		INSERT INTO builds (owner, repo, clone_url, tag, sha)
		VALUES ($1, $2, $3, $4, $5)`,
		c.Owner, c.Repo, c.CloneURL, c.Tag, c.SHA)
}

type webhookReceipt struct {
	Status string `json:"status"`
}

// renderAccepted mirrors render.JSON at 202 — fx's render has no status-aware JSON.
func renderAccepted(resp http.ResponseWriter, req *http.Request, receipt webhookReceipt) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(resp).Encode(receipt); err != nil {
		render.Error(resp, req, 500, err)
	}
}
