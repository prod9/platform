package install

import (
	"context"
	"fmt"

	"fx.prodigy9.co/data/migrator"
	"github.com/jmoiron/sqlx"
)

// The wizard step names, in one place — each step's Entry carries its own, and the
// save actions name their step to resetSuffix by them.
const (
	stepDBReachable    = "db-reachable"
	stepMigrations     = "migrations"
	stepServer         = "server"
	stepOrg            = "org"
	stepAppCreated     = "app-created"
	stepAppCredentials = "app-credentials"
	stepRegistryToken  = "registry-token"
	stepAppInstalled   = "app-installed"
	stepClaimed        = "claimed"
)

// Step is one self-contained wizard unit. Check produces the step's whole Entry,
// running under the install-time assumption — the world may be partially built (no
// database, no schema, unset keys) — and must reach its verdict without issuing a
// query it can predict will fail (docs/spec/installation.md, install-safe checks).
// Checks share one environment (the boot DB and the merged migration source); each
// uses what it needs. Steps are isolated: no check assumes another ran. Reset clears
// the step's own operator-entered values — suffix invalidation calls it on every
// step after a save (§Redo and suffix invalidation); steps with nothing
// operator-entered no-op.
type Step interface {
	name() string
	Check(ctx context.Context, db *sqlx.DB, merged migrator.Source) Entry
	Reset(ctx context.Context) error
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
// Values are the step's non-secret form fields, so a re-opened panel can re-display
// what is saved — secrets never qualify, their presence is implied by the state
// (docs/spec/installation.md, the state surface).
type Entry struct {
	Name    string            `json:"name"`
	State   StepState         `json:"state"`
	Message string            `json:"message,omitempty"`
	Values  map[string]string `json:"values,omitempty"`
}

// entry builds a Step's verdict, folding a check error into the message.
func entry(name string, state StepState, err error) Entry {
	e := Entry{Name: name, State: state}
	if err != nil {
		e.Message = err.Error()
	}
	return e
}

// wizard is the one producer of the order — GetState checks it, resetSuffix walks
// it. The order matters twice over: it is the wizard's sequence and the
// invalidation suffix. Every install-time value lives in settings, and the settings
// table exists only once migrations ran, so migrations precede every settings-backed
// entry; server heads those (every later panel's server-side URL renders from it),
// org follows (everything after is done on pages its slug locates); and
// the claim stays last (docs/spec/installation.md).
var wizard = []Step{
	dbReachable{},
	migrations{},
	server{},
	org{},
	appCreated{},
	appCredentials{},
	registryToken{},
	appInstalled{},
	claimed{},
}

// GetState checks every step in wizard order; db may be nil (no database
// configured).
func GetState(ctx context.Context, db *sqlx.DB, merged migrator.Source) []Entry {
	entries := make([]Entry, len(wizard))
	for i, step := range wizard {
		entries[i] = step.Check(ctx, db, merged)
	}
	return entries
}

// resetSuffix resets every step after the named one — the suffix half of a save:
// the caller runs it in the same transaction as its own write (§Redo and suffix
// invalidation). The rule is deliberately uniform — suffix, not dependency graph.
// The save actions name their step by the constants above; a name outside the
// wizard is a programming error, reported rather than silently resetting nothing.
func resetSuffix(ctx context.Context, after string) error {
	past := false
	for _, step := range wizard {
		if past {
			if err := step.Reset(ctx); err != nil {
				return err
			}
		}
		past = past || step.name() == after
	}

	if !past {
		return fmt.Errorf("install: resetSuffix: no wizard step named %q", after)
	}
	return nil
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
