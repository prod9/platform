package builds

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestObservationSeparatesFreshnessFromOutcome(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		module ModuleResult
		state  ObservationState
		last   time.Time
	}{
		{"unclaimed", ModuleResult{Status: StatusQueued}, ObservationNotStarted, time.Time{}},
		{"recent claim", ModuleResult{BuildModule: BuildModule{ClaimedAt: now.Add(-time.Minute)}, Status: StatusRunning}, ObservationRecent, now.Add(-time.Minute)},
		{"quiet without first event", ModuleResult{BuildModule: BuildModule{ClaimedAt: now.Add(-6 * time.Minute)}, Status: StatusRunning}, ObservationQuiet, now.Add(-6 * time.Minute)},
		{"exact boundary", ModuleResult{BuildModule: BuildModule{ClaimedAt: now.Add(-5 * time.Minute)}, Status: StatusRunning}, ObservationQuiet, now.Add(-5 * time.Minute)},
		{"quiet step", ModuleResult{BuildModule: BuildModule{ClaimedAt: now.Add(-time.Hour)}, Status: StatusRunning, LastEventAt: now.Add(-6 * time.Minute)}, ObservationQuiet, now.Add(-6 * time.Minute)},
		{"later event", ModuleResult{BuildModule: BuildModule{ClaimedAt: now.Add(-time.Hour)}, Status: StatusRunning, LastEventAt: now}, ObservationRecent, now},
		{"latest assignment", ModuleResult{BuildModule: BuildModule{ClaimedAt: now.Add(-time.Hour), EngineAssignedAt: now}, Status: StatusRunning, LastEventAt: now.Add(-time.Minute)}, ObservationRecent, now},
		{"future timestamp", ModuleResult{BuildModule: BuildModule{ClaimedAt: now.Add(time.Minute)}, Status: StatusRunning}, ObservationRecent, now.Add(time.Minute)},
		{"old success", ModuleResult{Status: StatusSucceeded, LastEventAt: now.Add(-time.Hour)}, ObservationTerminal, now.Add(-time.Hour)},
		{"old failure", ModuleResult{Status: StatusFailed, LastEventAt: now.Add(-time.Hour)}, ObservationTerminal, now.Add(-time.Hour)},
	} {
		t.Run(test.name, func(t *testing.T) {
			observed := test.module.Observation(now)
			require.Equal(t, test.state, observed.State)
			require.Equal(t, test.last, observed.LastObservedAt)
			require.Equal(t, now, observed.EvaluatedAt)
			require.Equal(t, int64(300), observed.QuietAfterSeconds)
		})
	}
}
