package srv

import (
	"context"
	"testing"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/dbname"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestMigrationsUsersAndIdentities(t *testing.T) {
	ctx := setupDB(t)

	var userID int64
	require.NoError(t, data.Get(ctx, &userID,
		`INSERT INTO users (name) VALUES ('octocat') RETURNING id`))

	insertIdentity := `
		INSERT INTO identities (user_id, provider, provider_id, kind)
		VALUES ($1, $2, $3, 'login')`
	require.NoError(t, data.Exec(ctx, insertIdentity, userID, "github", "12345"))

	var row struct {
		Email         string `db:"email"`
		EmailVerified bool   `db:"email_verified"`
		Metadata      string `db:"metadata"`
	}
	require.NoError(t, data.Get(ctx, &row, `
		SELECT email, email_verified, metadata::text AS metadata
		FROM identities
		WHERE provider = 'github' AND provider_id = '12345'`))
	require.Equal(t, "", row.Email)
	require.False(t, row.EmailVerified)
	require.JSONEq(t, `{}`, row.Metadata)

	sameIDOtherProvider := data.Exec(ctx, insertIdentity, userID, "sentry", "12345")
	require.NoError(t, sameIDOtherProvider)

	duplicate := data.Exec(ctx, insertIdentity, userID, "github", "12345")
	require.ErrorContains(t, duplicate, "duplicate key")

	orphan := data.Exec(ctx, insertIdentity, userID+1_000_000, "github", "67890")
	require.ErrorContains(t, orphan, "violates foreign key constraint")
}

func setupDB(t *testing.T) context.Context {
	skipWithoutPostgres(t)
	ctx := fxtest.ConnectTestDatabase(t)

	m := migrator.New(data.FromContext(ctx), migrator.FromFS(migrations))
	plans, dirty, err := m.Plan(ctx, migrator.IntentMigrate)
	require.NoError(t, err)
	require.False(t, dirty)

	for _, plan := range plans {
		require.NoError(t, m.Apply(ctx, plan))
	}
	return ctx
}

func skipWithoutPostgres(t *testing.T) {
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
