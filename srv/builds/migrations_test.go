package builds

import (
	"context"
	"testing"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/srvtest"
)

const moduleMigration = "202609130900_create_build_modules"

// Historical intent cannot be reconstructed from legacy events. The upgrade preserves
// those records separately and keeps new IDs beyond all previously allocated IDs.
// See docs/spec/platform-server.md §Build lifecycle (event-sourced).
func TestModuleMigrationPreservesHistoricalBuilds(t *testing.T) {
	for _, test := range []struct {
		name             string
		buildID, eventID int64
	}{
		{name: "sequence allocations"},
		{name: "explicit historical IDs", buildID: 100, eventID: 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, runner, upgrade := beforeModuleMigration(t)
			userID, err := auth.SystemUserID(ctx)
			require.NoError(t, err)
			var firstID, retryID int64
			require.NoError(t, data.Get(ctx, &firstID, `INSERT INTO builds
		(id, trigger, user_id, owner, repo, clone_url, ref, sha)
		VALUES (COALESCE(NULLIF($1, 0), nextval('builds_id_seq')), 'github-push', $2,
		'prod9', 'app', 'https://github.com/prod9/app.git',
		'refs/heads/main', 'old-sha') RETURNING id`, test.buildID, userID))
			require.NoError(t, data.Get(ctx, &retryID, `INSERT INTO builds
		(trigger, retry_of, user_id, owner, repo, clone_url, ref, sha)
		VALUES ('retry', $1, $2, 'prod9', 'app', 'https://github.com/prod9/app.git',
		'refs/heads/main', 'retry-sha') RETURNING id`, firstID, userID))
			require.NoError(t, data.Exec(ctx, `INSERT INTO build_events
		(id, build_id, kind, unit, step, at, error, image, hash, stdout, stderr)
		VALUES (COALESCE(NULLIF($1, 0), nextval('build_events_id_seq')), $2, 'step_done',
		'api', 'test', '2026-09-01T00:00:00Z',
		'old failure', 'old image', 'old hash', 'old stdout', 'old stderr')`, test.eventID, retryID))
			var allocatedBuild, allocatedEvent int64
			require.NoError(t, data.Get(ctx, &allocatedBuild, `SELECT nextval('builds_id_seq')`))
			require.NoError(t, data.Get(ctx, &allocatedEvent, `SELECT nextval('build_events_id_seq')`))
			oldBuilds := historicalRows(t, ctx, `SELECT jsonb_agg(to_jsonb(b) ORDER BY id)::text FROM builds b`)
			oldEvents := historicalRows(t, ctx, `SELECT jsonb_agg(to_jsonb(e) ORDER BY id)::text FROM build_events e`)

			require.NoError(t, runner.Apply(ctx, upgrade))
			require.Equal(t, oldBuilds, historicalRows(t, ctx,
				`SELECT jsonb_agg(to_jsonb(b) ORDER BY id)::text FROM build_history.builds b`))
			require.Equal(t, oldEvents, historicalRows(t, ctx,
				`SELECT jsonb_agg(to_jsonb(e) ORDER BY id)::text FROM build_history.build_events e`))
			require.NoError(t, runner.Apply(ctx, moduleRollback(t, ctx, runner)))
			require.Equal(t, oldBuilds, historicalRows(t, ctx,
				`SELECT jsonb_agg(to_jsonb(b) ORDER BY id)::text FROM builds b`))
			require.Equal(t, oldEvents, historicalRows(t, ctx,
				`SELECT jsonb_agg(to_jsonb(e) ORDER BY id)::text FROM build_events e`))
			plans, dirty, err := runner.Plan(ctx, migrator.IntentMigrate)
			require.NoError(t, err)
			require.False(t, dirty)
			require.Len(t, plans, 1)
			require.NoError(t, runner.Apply(ctx, plans[0]))
			build := queueTestBuild(t, ctx, "app")
			require.Equal(t, max(allocatedBuild, firstID, retryID)+1, build.ID)
			module := modulesFor(t, ctx, build.ID)[0]
			require.NoError(t, (&AppendEvent{BuildModuleID: module.ID, Kind: EventRunStart,
				At: time.Now()}).Execute(ctx, nil))
			var eventID int64
			require.NoError(t, data.Get(ctx, &eventID, `SELECT id FROM build_events WHERE build_module_id = $1`, module.ID))
			require.Equal(t, max(allocatedEvent, test.eventID)+1, eventID)

			rollback := moduleRollback(t, ctx, runner)
			require.ErrorContains(t, runner.Apply(ctx, rollback), "cannot roll back")
			require.Equal(t, oldBuilds, historicalRows(t, ctx,
				`SELECT jsonb_agg(to_jsonb(b) ORDER BY id)::text FROM build_history.builds b`))
			var activeID int64
			require.NoError(t, data.Get(ctx, &activeID, `SELECT id FROM builds`))
			require.Equal(t, build.ID, activeID)
		})
	}
}

func TestModuleMigrationEmptyRoundTripPreservesSequenceState(t *testing.T) {
	ctx, runner, upgrade := beforeModuleMigration(t)
	require.NoError(t, runner.Apply(ctx, upgrade))
	require.NoError(t, runner.Apply(ctx, moduleRollback(t, ctx, runner)))
	plans, dirty, err := runner.Plan(ctx, migrator.IntentMigrate)
	require.NoError(t, err)
	require.False(t, dirty)
	require.Len(t, plans, 1)
	require.Equal(t, moduleMigration, plans[0].Migration.Name)
	require.NoError(t, runner.Apply(ctx, plans[0]))

	build := queueTestBuild(t, ctx, "app")
	require.Equal(t, int64(1), build.ID)
	module := modulesFor(t, ctx, build.ID)[0]
	require.NoError(t, (&AppendEvent{BuildModuleID: module.ID, Kind: EventRunStart,
		At: time.Now()}).Execute(ctx, nil))
	var eventID int64
	require.NoError(t, data.Get(ctx, &eventID, `SELECT id FROM build_events WHERE build_module_id = $1`, module.ID))
	require.Equal(t, int64(1), eventID)
}

func TestModuleMigrationRollbackPreservesAllocatedIDs(t *testing.T) {
	ctx, runner, upgrade := beforeModuleMigration(t)
	require.NoError(t, runner.Apply(ctx, upgrade))
	var allocatedBuild, allocatedEvent int64
	for range 2 {
		require.NoError(t, data.Get(ctx, &allocatedBuild, `SELECT nextval('builds_id_seq')`))
	}
	for range 3 {
		require.NoError(t, data.Get(ctx, &allocatedEvent, `SELECT nextval('build_events_id_seq')`))
	}

	require.NoError(t, runner.Apply(ctx, moduleRollback(t, ctx, runner)))
	plans, dirty, err := runner.Plan(ctx, migrator.IntentMigrate)
	require.NoError(t, err)
	require.False(t, dirty)
	require.Len(t, plans, 1)
	require.NoError(t, runner.Apply(ctx, plans[0]))
	build := queueTestBuild(t, ctx, "app")
	require.Equal(t, allocatedBuild+1, build.ID)
	module := modulesFor(t, ctx, build.ID)[0]
	require.NoError(t, (&AppendEvent{BuildModuleID: module.ID, Kind: EventRunStart,
		At: time.Now()}).Execute(ctx, nil))
	var eventID int64
	require.NoError(t, data.Get(ctx, &eventID, `SELECT id FROM build_events WHERE build_module_id = $1`, module.ID))
	require.Equal(t, allocatedEvent+1, eventID)
}

func TestModuleMigrationArchiveConflictPreservesOldModel(t *testing.T) {
	ctx, runner, upgrade := beforeModuleMigration(t)
	userID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)
	require.NoError(t, data.Exec(ctx, `INSERT INTO builds
		(trigger, user_id, owner, repo, clone_url, ref, sha)
		VALUES ('github-push', $1, 'prod9', 'app', 'https://github.com/prod9/app.git',
		'refs/heads/main', 'old-sha')`, userID))
	oldBuilds := historicalRows(t, ctx, `SELECT jsonb_agg(to_jsonb(b) ORDER BY id)::text FROM builds b`)
	require.NoError(t, data.Exec(ctx, `CREATE SCHEMA build_history`))

	require.ErrorContains(t, runner.Apply(ctx, upgrade), "already exists")
	require.Equal(t, oldBuilds, historicalRows(t, ctx,
		`SELECT jsonb_agg(to_jsonb(b) ORDER BY id)::text FROM builds b`))
	plans, dirty, err := runner.Plan(ctx, migrator.IntentMigrate)
	require.NoError(t, err)
	require.False(t, dirty)
	require.Len(t, plans, 1)
	require.Equal(t, moduleMigration, plans[0].Migration.Name)
}

func beforeModuleMigration(t *testing.T) (context.Context, *migrator.Migrator, migrator.Plan) {
	srvtest.SkipWithoutPostgres(t)
	t.Setenv("SECRET", "the cake is a lie")
	t.Chdir(t.TempDir())
	ctx := fxtest.ConnectTestDatabase(t)
	runner := migrator.New(data.FromContext(ctx), migrator.FromAuto(config.FromContext(ctx)))
	plans, dirty, err := runner.Plan(ctx, migrator.IntentMigrate)
	require.NoError(t, err)
	require.False(t, dirty)
	for _, plan := range plans {
		if plan.Migration.Name == moduleMigration {
			return ctx, runner, plan
		}
		require.NoError(t, runner.Apply(ctx, plan))
	}
	require.FailNow(t, "module migration is absent from the registered source")
	return ctx, runner, migrator.Plan{}
}

func moduleRollback(t *testing.T, ctx context.Context, runner *migrator.Migrator) migrator.Plan {
	plans, dirty, err := runner.Plan(ctx, migrator.IntentRollback)
	require.NoError(t, err)
	require.False(t, dirty)
	require.Len(t, plans, 1)
	require.Equal(t, moduleMigration, plans[0].Migration.Name)
	return plans[0]
}

func historicalRows(t *testing.T, ctx context.Context, query string) string {
	var rows string
	require.NoError(t, data.Get(ctx, &rows, query))
	return rows
}
