package install

import (
	"context"

	"fx.prodigy9.co/data/migrator"
	"github.com/jmoiron/sqlx"
)

// Step is one wizard unit: a name and a check. Check runs under the install-time
// assumption — the world may be partially built (no database, no schema, unset keys) —
// and must reach its verdict without issuing a query it can predict will fail
// (docs/spec/installation.md, install-safe checks). Steps are isolated: no check
// assumes another ran.
type Step interface {
	Name() string
	Check(ctx context.Context, db *sqlx.DB) (StepState, error)
}

// StepState is a check's verdict. Unknown is the zero value on purpose: an unset
// state reads as "nobody knows", never as a verdict.
type StepState string

const (
	UnknownState              StepState = "" // indeterminable — the check itself failed
	NotStartedState           StepState = "not_started"
	InterventionRequiredState StepState = "intervention_required"
	PartiallyReadyState       StepState = "partially_ready"
	FullyReadyState           StepState = "fully_ready"
)

// Entry is a Step's checked verdict shaped for the wire; GetState produces one per
// step, in wizard order. Message carries the check's error when it produced one.
type Entry struct {
	Name    string    `json:"name"`
	State   StepState `json:"state"`
	Message string    `json:"message,omitempty"`
}

// GetState checks every step in wizard order; db may be nil (no database
// configured). The order is the wizard's: every install-time value lives in
// settings, and the settings table exists only once migrations ran, so migrations
// precede both settings-backed entries and the claim stays last
// (docs/spec/installation.md).
func GetState(ctx context.Context, db *sqlx.DB, merged migrator.Source) []Entry {
	steps := []Step{
		dbReachable{},
		migrations{src: merged},
		appCredentials{},
		appInstalled{},
		fluxSetup{},
	}

	entries := make([]Entry, len(steps))
	for i, step := range steps {
		state, err := step.Check(ctx, db)

		entries[i] = Entry{Name: step.Name(), State: state}
		if err != nil {
			entries[i].Message = err.Error()
		}
	}
	return entries
}

// Complete reports whether every step is fully ready — the "completely installed"
// conjunction.
func Complete(entries []Entry) bool {
	for _, entry := range entries {
		if entry.State != FullyReadyState {
			return false
		}
	}
	return true
}
