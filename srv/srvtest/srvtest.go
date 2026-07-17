// Package srvtest is the shared test scaffolding for srv fragments: postgres
// availability gating and per-test database setup. It imports no fragment — each
// fragment's tests pass in the migration sources they need, so srvtest stays usable
// from every fragment without import cycles.
package srvtest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"strings"
	"testing"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/dbname"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/migrate"
)

// SetupDB connects a fresh test database and applies the given migration sources,
// skipping the test when postgres is unreachable. SECRET is set so fragments using
// fx secret encryption work without per-test env plumbing.
func SetupDB(t *testing.T, sources ...migrator.Source) context.Context {
	SkipWithoutPostgres(t)
	t.Setenv("SECRET", "the cake is a lie")
	ctx := fxtest.ConnectTestDatabase(t)

	m := migrator.New(data.FromContext(ctx), migrate.Merged(sources...))
	plans, dirty, err := m.Plan(ctx, migrator.IntentMigrate)
	require.NoError(t, err)
	require.False(t, dirty)

	for _, plan := range plans {
		require.NoError(t, m.Apply(ctx, plan))
	}
	return ctx
}

// InstallToken is the installation token InstallationAPIMux mints.
const InstallToken = "ghs_installtoken"

// AppKey generates a throwaway RSA key and returns it with its PKCS#1 PEM form —
// the shape GitHub issues App private keys in.
func AppKey(t *testing.T) (*rsa.PrivateKey, string) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return key, string(keyPEM)
}

// InstallationAPIMux handles the two calls installation-token minting walks: the
// repo installation lookup (asserting a well-formed App JWT arrives) and the
// access-token create answering InstallToken. Fragment tests extend it with their
// own endpoints.
func InstallationAPIMux(t *testing.T) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/prod9/app/installation", func(resp http.ResponseWriter, req *http.Request) {
		auth := req.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(auth, "Bearer "))
		require.Len(t, strings.Split(strings.TrimPrefix(auth, "Bearer "), "."), 3)

		resp.Header().Set("Content-Type", "application/json")
		resp.Write([]byte(`{"id": 42}`))
	})
	mux.HandleFunc("POST /app/installations/42/access_tokens", func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(http.StatusCreated)
		resp.Write([]byte(`{"token": "` + InstallToken + `"}`))
	})
	return mux
}

// SkipWithoutPostgres skips the test unless DATABASE_URL points at a reachable
// postgres.
func SkipWithoutPostgres(t *testing.T) {
	url := config.Get(fxtest.Configure(), data.DatabaseURLConfig)
	if url == "" {
		t.Skip("DATABASE_URL unset; skipping postgres-backed test")
	}

	adminURL, err := dbname.Set(url, "postgres")
	if err != nil {
		t.Skipf("unusable DATABASE_URL: %s", err)
	}

	db, err := sqlx.Connect("pgx", adminURL)
	if err != nil {
		t.Skipf("postgres unreachable: %s", err)
	}
	db.Close()
}
