package install

import (
	"context"
	"net/http"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/render"
	"platform.prodigy9.co/srv/github"
)

type recordKey struct{}

// NewContext seeds ctx with the bound install record.
func NewContext(ctx context.Context, record *Record) context.Context {
	return context.WithValue(ctx, recordKey{}, record)
}

// FromContext returns the install record RecordContext (or NewContext) seeded.
func FromContext(ctx context.Context) (*Record, bool) {
	record, ok := ctx.Value(recordKey{}).(*Record)
	return record, ok
}

// RecordContext seeds every request with the current bound install record — the
// ambient-truth delivery for product fragments (docs/spec/installation.md). It fails
// closed: a product route never runs uninstalled.
func RecordContext(*config.Source) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if _, ok := data.LookupFromContext(req.Context()); !ok {
				// data.Get panics without a data context, and a route behind this
				// middleware must never run without the record — 500, not a crash.
				render.Error(resp, req, 500, ErrNotInstalled)
				return
			}

			record, err := Load(req.Context())
			if err != nil {
				render.Error(resp, req, 500, err)
				return
			}

			next.ServeHTTP(resp, req.WithContext(NewContext(req.Context(), record)))
		})
	}
}

// Bound resolves the install record: from the context when RecordContext seeded it,
// loaded otherwise — the one fallback every out-of-HTTP caller shares.
func Bound(ctx context.Context) (*Record, error) {
	if record, ok := FromContext(ctx); ok {
		return record, nil
	}
	return Load(ctx)
}

// Token mints a fresh ~1h installation token for one autonomous operation — mint per
// use, never store (docs/spec/platform-server.md §Two token types). It returns the App
// client that minted the token alongside it, so callers with further installation-scoped
// calls to make reuse it instead of constructing a second one. The record comes from the
// context when RecordContext seeded it; the worker path, which runs outside HTTP, falls
// back to loading it.
func Token(ctx context.Context) (string, *github.Client, error) {
	record, err := Bound(ctx)
	if err != nil {
		return "", nil, err
	}

	client, err := github.NewClient(ctx)
	if err != nil {
		return "", nil, err
	}
	token, err := client.InstallationToken(ctx, record.InstallationID)
	if err != nil {
		return "", nil, err
	}
	return token, client, nil
}
