package builds

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"fx.prodigy9.co/data"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
)

func TestBuildReadFreshnessPreservesOutcomeAndRefreshesEvidence(t *testing.T) {
	ctx, cfg := setupInstalled(t)
	create := &Create{Owner: "prodigy9", Repo: "app", Ref: "refs/heads/main",
		SHA: "observedsha", UserID: 1, Trigger: TriggerWebUI}
	prepareCreate(t, ctx, create)
	build := &Build{}
	require.NoError(t, create.Execute(ctx, build))
	modules := modulesFor(t, ctx, build.ID)
	old := time.Now().Add(-10 * time.Minute).Truncate(time.Microsecond)
	require.NoError(t, data.Exec(ctx, `UPDATE build_modules
		SET claimed_at=$2, claimed_by='worker' WHERE id=$1`, modules[0].ID, old))
	token := startTestSession(t, ctx, auth.Repository{GitHubID: 1, Owner: "prodigy9",
		Name: "app", Permission: auth.PermissionRead})
	router := apiRouter(t, cfg)

	response := requestBuilds(t, ctx, router, token, "GET", "/api/repos/prodigy9/app/builds", "")
	require.Equal(t, http.StatusOK, response.Code)
	var listing []buildResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &listing))
	require.Len(t, listing, 1)
	require.Equal(t, StatusRunning, listing[0].Status)
	require.Equal(t, StatusRunning, listing[0].Modules[0].Status)
	require.Equal(t, ObservationQuiet, listing[0].Modules[0].Observation.State)
	require.True(t, old.Equal(listing[0].Modules[0].Observation.LastObservedAt))
	require.Equal(t, ObservationNotStarted, listing[0].Modules[1].Observation.State)
	require.Equal(t, listing[0].Modules[0].Observation.EvaluatedAt,
		listing[0].Modules[1].Observation.EvaluatedAt)
	require.EqualValues(t, 300, listing[0].Modules[0].Observation.QuietAfterSeconds)

	require.NoError(t, (&AppendEvent{BuildModuleID: modules[0].ID,
		Kind: EventStepStart, Step: "build", At: time.Now()}).Execute(ctx, nil))
	response = requestBuilds(t, ctx, router, token, "GET", "/api/repos/prodigy9/app/builds", "")
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &listing))
	require.Equal(t, StatusRunning, listing[0].Status)
	require.Equal(t, ObservationRecent, listing[0].Modules[0].Observation.State)
	require.True(t, listing[0].Modules[0].Observation.LastObservedAt.After(old))
}
