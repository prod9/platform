package engine

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dagger.io/dagger"
	"github.com/stretchr/testify/require"
)

type inspectionConn struct {
	queryErr error
	closeErr error
	closed   atomic.Bool
	cancel   context.CancelFunc
}

func (c *inspectionConn) Do(req *http.Request) (*http.Response, error) {
	if c.cancel != nil {
		c.cancel()
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":{"version":"v0.21.7"}}`)),
	}, nil
}

func (*inspectionConn) Host() string { return "inspection.test" }

func (c *inspectionConn) Close() error {
	c.closed.Store(true)
	return c.closeErr
}

func TestInspectEmptyFleetNeverConnects(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		state  RosterState
	}{
		{name: "unconfigured", state: RosterUnconfigured},
		{name: "empty DNS", source: "engines.test", state: RosterEmpty},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("DAGGER_ENGINE", test.source)
			stubLookup(t, func(context.Context, string) ([]string, error) { return nil, nil })
			var connections atomic.Int32
			stubInspectionConnect(t, func(context.Context) (*dagger.Client, error) {
				connections.Add(1)
				return nil, errors.New("unexpected connection")
			})
			session := NewSession(rosterCtx())

			report, err := session.Inspect(t.Context())

			require.NoError(t, err)
			require.Equal(t, test.state, report.State)
			require.Equal(t, test.source, report.Source)
			require.NotZero(t, report.ObservedAt)
			require.NotNil(t, report.Endpoints)
			require.Empty(t, report.Endpoints)
			require.Zero(t, connections.Load())
			require.NoError(t, session.Close())
		})
	}
}

func TestInspectUniqueSortedRosterAndSessionCleanup(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "engines.test")
	t.Setenv("DAGGER_ENGINE_PORT", "4321")
	stubLookup(t, func(context.Context, string) ([]string, error) {
		return []string{"10.0.0.2", "10.0.0.1", "10.0.0.2"}, nil
	})
	conn := &inspectionConn{closeErr: errors.New("cleanup failed")}
	var connections atomic.Int32
	stubInspectionConnect(t, func(ctx context.Context) (*dagger.Client, error) {
		connections.Add(1)
		return dagger.Connect(ctx, dagger.WithConn(conn))
	})
	session := NewSession(rosterCtx())

	report, err := session.Inspect(t.Context())

	require.NoError(t, err)
	require.Equal(t, RosterResolved, report.State)
	require.Equal(t, 4321, report.Port)
	require.Len(t, report.Endpoints, 2)
	require.Equal(t, "10.0.0.1:4321", report.Endpoints[0].Address)
	require.Equal(t, "10.0.0.2:4321", report.Endpoints[1].Address)
	for _, endpoint := range report.Endpoints {
		require.Equal(t, EndpointReady, endpoint.State)
		require.Equal(t, "v0.21.7", endpoint.Version)
		require.NoError(t, endpoint.Err)
		require.NotZero(t, endpoint.ObservedAt)
	}
	require.EqualValues(t, 2, connections.Load())
	require.False(t, conn.closed.Load())
	require.ErrorIs(t, session.Close(), conn.closeErr)
	require.True(t, conn.closed.Load())
	require.Equal(t, "v0.21.7", report.Endpoints[0].Version)
}

func TestInspectPartialConnectionAndVersionFailures(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "engines.test")
	stubLookup(t, func(context.Context, string) ([]string, error) {
		return []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, nil
	})
	dialErr := errors.New("connection refused")
	queryErr := errors.New("version unavailable")
	var connections atomic.Int32
	stubInspectionConnect(t, func(ctx context.Context) (*dagger.Client, error) {
		switch connections.Add(1) {
		case 1:
			return nil, dialErr
		case 2:
			return dagger.Connect(ctx, dagger.WithConn(&inspectionConn{queryErr: queryErr}))
		default:
			return dagger.Connect(ctx, dagger.WithConn(&inspectionConn{}))
		}
	})
	session := NewSession(rosterCtx())

	report, err := session.Inspect(t.Context())

	require.NoError(t, err)
	require.Len(t, report.Endpoints, 3)
	states := map[EndpointState]EndpointObservation{}
	for _, endpoint := range report.Endpoints {
		states[endpoint.State] = endpoint
	}
	require.Len(t, states, 3)
	require.ErrorIs(t, states[EndpointUnreachable].Err, dialErr)
	require.ErrorContains(t, states[EndpointVersionUnavailable].Err, queryErr.Error())
	require.Equal(t, "v0.21.7", states[EndpointReady].Version)
	require.NoError(t, session.Close())
}

func TestInspectLookupFailurePreservesSource(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "engines.test")
	lookupErr := errors.New("DNS unavailable")
	stubLookup(t, func(context.Context, string) ([]string, error) { return nil, lookupErr })
	session := NewSession(rosterCtx())

	report, err := session.Inspect(t.Context())

	require.ErrorIs(t, err, lookupErr)
	require.Equal(t, RosterLookupFailed, report.State)
	require.Equal(t, "engines.test", report.Source)
	require.NotZero(t, report.ObservedAt)
	require.NoError(t, session.Close())
}

func TestInspectCancellationAndBoundReachConnection(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "engines.test")
	stubLookup(t, func(context.Context, string) ([]string, error) { return []string{"10.0.0.1"}, nil })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var hasBound atomic.Bool
	stubInspectionConnect(t, func(ctx context.Context) (*dagger.Client, error) {
		deadline, ok := ctx.Deadline()
		hasBound.Store(ok && time.Until(deadline) <= 3*time.Second)
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	})
	session := NewSession(rosterCtx())

	report, err := session.Inspect(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.True(t, hasBound.Load())
	require.Len(t, report.Endpoints, 1)
	require.ErrorIs(t, report.Endpoints[0].Err, context.Canceled)
	require.NoError(t, session.Close())
}

func TestInspectCancellationDuringVersionRead(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "engines.test")
	stubLookup(t, func(context.Context, string) ([]string, error) { return []string{"10.0.0.1"}, nil })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	conn := &inspectionConn{cancel: cancel}
	stubInspectionConnect(t, func(ctx context.Context) (*dagger.Client, error) {
		return dagger.Connect(ctx, dagger.WithConn(conn))
	})
	session := NewSession(rosterCtx())

	report, err := session.Inspect(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, report.Endpoints, 1)
	require.Equal(t, EndpointVersionUnavailable, report.Endpoints[0].State)
	require.ErrorIs(t, report.Endpoints[0].Err, context.Canceled)
	require.NoError(t, session.Close())
	require.True(t, conn.closed.Load())
}

func TestInspectBoundRetainsOtherEndpointObservations(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "engines.test")
	stubLookup(t, func(context.Context, string) ([]string, error) {
		return []string{"10.0.0.1", "10.0.0.2"}, nil
	})
	var connections atomic.Int32
	stubInspectionConnect(t, func(ctx context.Context) (*dagger.Client, error) {
		conn := &inspectionConn{}
		if connections.Add(1) == 2 {
			// Leave the request blocked until the inspection's own bound expires.
			conn.cancel = func() {}
		}
		return dagger.Connect(ctx, dagger.WithConn(conn))
	})
	session := NewSession(rosterCtx())

	report, err := session.Inspect(t.Context())

	require.NoError(t, err)
	require.Equal(t, RosterResolved, report.State)
	require.Len(t, report.Endpoints, 2)
	states := map[EndpointState]EndpointObservation{}
	for _, endpoint := range report.Endpoints {
		states[endpoint.State] = endpoint
	}
	require.Equal(t, "v0.21.7", states[EndpointReady].Version)
	require.ErrorIs(t, states[EndpointVersionUnavailable].Err, context.DeadlineExceeded)
	require.NoError(t, session.Close())
}

func TestInspectCanceledBeforeDiscovery(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "engines.test")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	stubLookup(t, func(context.Context, string) ([]string, error) {
		return nil, errors.New("unexpected lookup")
	})
	session := NewSession(rosterCtx())

	report, err := session.Inspect(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, "engines.test", report.Source)
	require.Equal(t, RosterLookupFailed, report.State)
	require.NotNil(t, report.Endpoints)
	require.Empty(t, report.Endpoints)
	require.NoError(t, session.Close())
}

func stubInspectionConnect(t *testing.T, connect func(context.Context) (*dagger.Client, error)) {
	t.Helper()
	previous := connectEngine
	connectEngine = func(ctx context.Context, _ ...dagger.ClientOpt) (*dagger.Client, error) {
		return connect(ctx)
	}
	t.Cleanup(func() { connectEngine = previous })
}
