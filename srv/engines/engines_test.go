package engines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fx.prodigy9.co/app"
	"fx.prodigy9.co/data"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/engine"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/builds"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/repos"
	"platform.prodigy9.co/srv/srvtest"
)

func init() {
	for _, fragment := range []app.Interface{auth.App.App(), builds.App.App(), repos.App.App(), install.App.App()} {
		app.RegisterMigrations(fragment)
	}
}

func TestFleetFiltersAssignmentsAndKeepsPartialObservations(t *testing.T) {
	ctx, request := authorizedRequest(t)
	ongoing := recordAssignment(t, ctx, "visible", "worker:1234", builds.StatusRunning)
	recordAssignment(t, ctx, "visible", "worker:1234", builds.StatusSucceeded)
	recordAssignment(t, ctx, "private", "worker:1234", builds.StatusRunning)
	stubInspection(t, engine.Inspection{Source: "engines", Port: 1234, State: engine.RosterResolved,
		ObservedAt: time.Now(), Endpoints: []engine.EndpointObservation{
			{Address: "worker:1234", State: engine.EndpointReady, Version: "v1", ObservedAt: time.Now()},
			{Address: "broken:1234", State: engine.EndpointUnreachable, Err: errors.New("private diagnostic")},
		}}, nil)
	view, err := ReadFleet(request)
	require.NoError(t, err)
	require.Equal(t, engine.RosterResolved, view.RosterState)
	require.Len(t, view.Engines, 2)
	require.Equal(t, "broken:1234", view.Engines[0].Address)
	require.NotEmpty(t, view.Engines[0].Error)
	require.Empty(t, view.Engines[0].Assignments)
	require.Len(t, view.Engines[1].Assignments, 1)
	require.Equal(t, ongoing, view.Engines[1].Assignments[0].BuildModuleID)
	require.Equal(t, builds.StatusRunning, view.Engines[1].Assignments[0].Status)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "private diagnostic")
	require.NotContains(t, string(raw), `"repo":"private"`)
	var decoded FleetView
	require.NoError(t, json.Unmarshal(raw, &decoded))
	observation := decoded.Engines[1].Assignments[0].Observation
	require.Equal(t, builds.ObservationRecent, observation.State)
	require.False(t, observation.LastObservedAt.IsZero())
	require.False(t, observation.EvaluatedAt.IsZero())
	require.Equal(t, int64(300), observation.QuietAfterSeconds)
}

func TestInstanceAdmitsOnlyCurrentOrReadableHistory(t *testing.T) {
	ctx, request := authorizedRequest(t)
	recordAssignment(t, ctx, "visible", "gone:1234", builds.StatusSucceeded)
	recordAssignment(t, ctx, "private", "secret:1234", builds.StatusSucceeded)
	recordAssignment(t, ctx, "visible", "local", builds.StatusSucceeded)
	stubInspection(t, engine.Inspection{State: engine.RosterEmpty, ObservedAt: time.Now()}, nil)
	view, err := ReadInstance(request, "gone:1234")
	require.NoError(t, err)
	require.Equal(t, EndpointAbsent, view.Engine.State)
	require.Len(t, view.History, 1)
	require.Equal(t, builds.StatusSucceeded, view.History[0].Status)
	_, err = ReadInstance(request, "secret:1234")
	require.ErrorIs(t, err, ErrUnknownEndpoint)
	_, err = ReadInstance(request, "arbitrary.internal:1234")
	require.ErrorIs(t, err, ErrUnknownEndpoint)
	local, err := ReadInstance(request, "local")
	require.NoError(t, err)
	require.Equal(t, EndpointLocal, local.Engine.State)
	stubInspection(t, engine.Inspection{State: engine.RosterLookupFailed, ObservedAt: time.Now()}, nil)
	unknown, err := ReadInstance(request, "gone:1234")
	require.NoError(t, err)
	require.Equal(t, EndpointUnknown, unknown.Engine.State)
	require.NotEmpty(t, unknown.Error)
}

func TestInstanceHistoryBoundsModuleAssignments(t *testing.T) {
	ctx, request := authorizedRequest(t)
	ongoing := recordAssignment(t, ctx, "visible", "gone:1234", builds.StatusRunning)
	var last int64
	for range 52 {
		last = recordAssignment(t, ctx, "visible", "gone:1234", builds.StatusSucceeded)
	}
	stubInspection(t, engine.Inspection{State: engine.RosterResolved, Endpoints: []engine.EndpointObservation{
		{Address: "gone:1234", State: engine.EndpointReady},
	}}, nil)
	view, err := ReadInstance(request, "gone:1234")
	require.NoError(t, err)
	require.Len(t, view.History, 50)
	require.Equal(t, last, view.History[0].BuildModuleID)
	require.Greater(t, view.History[0].BuildModuleID, view.History[49].BuildModuleID)
	require.Len(t, view.Engine.Assignments, 1)
	require.Equal(t, ongoing, view.Engine.Assignments[0].BuildModuleID)
	stubInspection(t, engine.Inspection{State: engine.RosterEmpty}, nil)
	absent, err := ReadInstance(request, "gone:1234")
	require.NoError(t, err)
	require.Equal(t, EndpointAbsent, absent.Engine.State)
	require.Len(t, absent.Engine.Assignments, 1)
	require.Equal(t, ongoing, absent.Engine.Assignments[0].BuildModuleID)
}

func TestObservationErrorsAndAuthenticationRemainRequestFailures(t *testing.T) {
	_, request := authorizedRequest(t)
	for _, failure := range []error{context.Canceled, errors.New("cleanup failed")} {
		stubInspection(t, engine.Inspection{}, failure)
		_, err := ReadFleet(request)
		require.ErrorIs(t, err, failure)
	}
	router := chi.NewRouter()
	require.NoError(t, (EngineCtr{}).Mount(nil, router))
	for _, path := range []string{"/api/engines", "/api/engines/worker:1234"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusUnauthorized, response.Code)
	}
}

func TestDiscoveryBoundIsAnObservationButCallerCancellationAndCleanupFail(t *testing.T) {
	_, request := authorizedRequest(t)
	report := engine.Inspection{State: engine.RosterLookupFailed}
	stubInspection(t, report, context.DeadlineExceeded)
	view, err := ReadFleet(request)
	require.NoError(t, err)
	require.Equal(t, engine.RosterLookupFailed, view.RosterState)
	require.NotEmpty(t, view.Error)

	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	_, err = inspect(ctx)
	require.ErrorIs(t, err, context.Canceled)

	cleanupErr := errors.New("cleanup failed")
	openInspection = func(context.Context) inspectionSession {
		return &inspectionStub{report: report, err: context.DeadlineExceeded, closeErr: cleanupErr}
	}
	_, err = ReadFleet(request)
	require.ErrorIs(t, err, cleanupErr)
}

func TestAPIKeepsEmptyArraysAndAdmitsCurrentEndpointWithoutHistory(t *testing.T) {
	_, request := authorizedRequest(t)
	router := chi.NewRouter()
	require.NoError(t, (EngineCtr{}).Mount(nil, router))
	stubInspection(t, engine.Inspection{State: engine.RosterUnconfigured}, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"engines":[]`)
	stubInspection(t, engine.Inspection{State: engine.RosterResolved, Endpoints: []engine.EndpointObservation{
		{Address: "worker:1234", State: engine.EndpointVersionUnavailable, Err: errors.New("private details")},
	}}, nil)
	detail := httptest.NewRequest(http.MethodGet, "/api/engines/worker%3A1234", nil).WithContext(request.Context())
	detail.AddCookie(&http.Cookie{Name: "platform_session", Value: "reader-session"})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, detail)
	require.Equal(t, http.StatusOK, response.Code)
	var view InstanceView
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &view))
	require.Equal(t, "worker:1234", view.Engine.Address)
	require.Equal(t, EndpointVersionUnavailable, view.Engine.State)
	require.NotNil(t, view.History)
	require.NotNil(t, view.Engine.Assignments)
	require.NotContains(t, response.Body.String(), "private details")
}

func TestInstanceUsesOneFreshnessEvaluationForHistoryAndAssignments(t *testing.T) {
	ctx, request := authorizedRequest(t)
	recordAssignment(t, ctx, "visible", "worker:1234", builds.StatusRunning)
	stubInspection(t, engine.Inspection{State: engine.RosterResolved, Endpoints: []engine.EndpointObservation{
		{Address: "worker:1234", State: engine.EndpointReady},
	}}, nil)
	view, err := ReadInstance(request, "worker:1234")
	require.NoError(t, err)
	require.Len(t, view.History, 1)
	require.Len(t, view.Engine.Assignments, 1)
	require.False(t, view.History[0].Observation.EvaluatedAt.IsZero())
	require.Equal(t, view.History[0].Observation, view.Engine.Assignments[0].Observation)
}

func TestDatabaseFailureRemainsAnError(t *testing.T) {
	ctx, request := authorizedRequest(t)
	stubInspection(t, engine.Inspection{State: engine.RosterEmpty}, nil)
	require.NoError(t, data.FromContext(ctx).Close())
	_, err := ReadFleet(request)
	require.Error(t, err)
}

func stubInspection(t *testing.T, report engine.Inspection, err error) {
	t.Helper()
	previous := openInspection
	t.Cleanup(func() { openInspection = previous })
	openInspection = func(context.Context) inspectionSession { return &inspectionStub{report: report, err: err} }
}

type inspectionStub struct {
	report   engine.Inspection
	err      error
	closeErr error
}

func (s *inspectionStub) Inspect(ctx context.Context) (engine.Inspection, error) {
	if err := ctx.Err(); err != nil {
		return s.report, err
	}
	return s.report, s.err
}

func (s *inspectionStub) Close() error { return s.closeErr }

func authorizedRequest(t *testing.T) (context.Context, *http.Request) {
	t.Helper()
	ctx := srvtest.SetupDB(t)
	var userID int64
	require.NoError(t, data.Get(ctx, &userID, `INSERT INTO users(name) VALUES ('reader') RETURNING id`))
	require.NoError(t, (&auth.CreateSession{UserID: userID, Token: "reader-session", ExpiresAt: time.Now().Add(time.Hour)}).Execute(ctx, nil))
	require.NoError(t, data.Exec(ctx, `INSERT INTO session_repositories
		(session_id, github_repository_id, owner, name, permission)
		SELECT id, 1, 'prod9', 'visible', 'read' FROM sessions WHERE user_id=$1`, userID))
	request := httptest.NewRequest(http.MethodGet, "/api/engines", nil).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: "platform_session", Value: "reader-session"})
	return ctx, request
}

func recordAssignment(t *testing.T, ctx context.Context, repo, address string, status builds.Status) int64 {
	t.Helper()
	userID, err := auth.SystemUserID(ctx)
	require.NoError(t, err)
	require.NoError(t, data.Exec(ctx, `INSERT INTO repos(owner,repo,registered_by)
		VALUES ('prod9',$1,$2) ON CONFLICT (owner,repo) DO NOTHING`, repo, userID))
	raw := fmt.Sprintf("repository='github.com/prod9/%s'\n[modules.api]\nframework='go/basic'\n", repo)
	manifest, err := repos.ParseManifest([]byte(raw), repo)
	require.NoError(t, err)
	create := &builds.Create{Trigger: builds.TriggerWebUI, UserID: userID, Owner: "prod9", Repo: repo,
		Ref: "refs/heads/main", SHA: "abc123", CloneURL: "https://github.com/prod9/" + repo + ".git",
		ManifestRaw: raw, Manifest: *manifest}
	build := &builds.Build{}
	require.NoError(t, create.Execute(ctx, build))
	var moduleID int64
	require.NoError(t, data.Get(ctx, &moduleID, `SELECT id FROM build_modules WHERE build_id=$1`, build.ID))
	require.NoError(t, (&builds.ClaimModule{ID: moduleID, Hostname: "worker"}).Execute(ctx, &builds.BuildModule{}))
	require.NoError(t, (&builds.AppendEvent{BuildModuleID: moduleID, Kind: builds.EventConfigDone,
		At: time.Now(), EngineHost: address}).Execute(ctx, nil))
	if status == builds.StatusSucceeded {
		require.NoError(t, (&builds.AppendEvent{BuildModuleID: moduleID, Kind: builds.EventRunDone, At: time.Now()}).Execute(ctx, nil))
	}
	return moduleID
}
