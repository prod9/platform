package install

// The ten wizard steps. Each check is isolated and install-safe: a missing
// database or schema is a verdict (intervention required / not started), never a
// query sent to fail (docs/spec/installation.md, install-safe checks).

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"github.com/jmoiron/sqlx"
	buildengine "platform.prodigy9.co/engine"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/migrate"
)

var errNoDatabase = errors.New("no database configured")

type dbReachable struct{}

func (dbReachable) name() string { return stepDBReachable }

func (s dbReachable) Check(ctx context.Context, db *sqlx.DB) Entry {
	if db == nil {
		return entry(s.name(), InterventionRequiredState, errNoDatabase)
	}
	if err := db.PingContext(ctx); err != nil {
		return entry(s.name(), InterventionRequiredState, err)
	}
	return entry(s.name(), FullyReadyState, nil)
}

func (dbReachable) Reset(context.Context) error { return nil }

type migrations struct{ src migrator.Source }

func (migrations) name() string { return stepMigrations }

func (m migrations) Check(ctx context.Context, db *sqlx.DB) Entry {
	if db == nil {
		return entry(m.name(), UnknownState, errNoDatabase)
	}

	applied, pending, dirty, err := migrate.State(ctx, db, m.src)
	if err != nil {
		return entry(m.name(), UnknownState, err)
	}
	if dirty {
		return entry(m.name(), InterventionRequiredState,
			errors.New("schema diverges from embedded migrations"))
	}

	switch {
	case pending == 0:
		return entry(m.name(), FullyReadyState, nil)
	case applied == 0:
		return entry(m.name(), NotStartedState, nil)
	default:
		return entry(m.name(), PartiallyReadyState, nil)
	}
}

func (migrations) Reset(context.Context) error { return nil }

type server struct{}

func (server) name() string { return stepServer }

// Check reads the server's public URL and surfaces it in values — the one field its
// re-opened panel pre-fills (docs/spec/installation.md, the state surface).
func (s server) Check(ctx context.Context, db *sqlx.DB) Entry {
	return settingsBacked(ctx, db, s.name(), func(ctx context.Context) (map[string]string, error) {
		publicURL, err := github.LoadPublicURL(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{"public_url": publicURL}, nil
	}, github.ErrNoPublicURL)
}

func (server) Reset(ctx context.Context) error { return github.ClearPublicURL(ctx) }

type org struct{}

func (org) name() string { return stepOrg }

// Check reads the primary-org slug and surfaces it in values — the one field its
// re-opened panel pre-fills (docs/spec/installation.md, the state surface).
func (s org) Check(ctx context.Context, db *sqlx.DB) Entry {
	return settingsBacked(ctx, db, s.name(), func(ctx context.Context) (map[string]string, error) {
		slug, err := github.LoadOrg(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{"org": slug}, nil
	}, github.ErrNoOrg)
}

func (org) Reset(ctx context.Context) error { return github.ClearOrg(ctx) }

type appCreated struct{}

func (appCreated) name() string { return stepAppCreated }

// Check reads the creation-time quartet — what GitHub's creation form yields; the
// generated keys are app-credentials' concern. The app id, slug, and client id
// surface in values; the webhook secret never does
// (docs/spec/installation.md, the state surface).
func (s appCreated) Check(ctx context.Context, db *sqlx.DB) Entry {
	return settingsBacked(ctx, db, s.name(), func(ctx context.Context) (map[string]string, error) {
		creation, err := github.LoadAppCreation(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{
			"app_id":    strconv.FormatInt(creation.AppID, 10),
			"app_slug":  creation.Slug,
			"client_id": creation.ClientID,
		}, nil
	}, github.ErrNoApp)
}

func (appCreated) Reset(ctx context.Context) error { return github.ClearAppCreation(ctx) }

type appCredentials struct{}

func (appCredentials) name() string { return stepAppCredentials }

// Check goes one step past presence: with credentials saved it reads GET /app and
// compares the App's permissions against the required set — saved-but-under-scoped
// is partially ready, and the message names the gap
// (docs/spec/installation.md, the credentials check).
func (s appCredentials) Check(ctx context.Context, db *sqlx.DB) Entry {
	if db == nil {
		return entry(s.name(), UnknownState, errNoDatabase)
	}

	ready, err := settingsSchemaReady(ctx, db)
	if err != nil {
		return entry(s.name(), UnknownState, err)
	}
	if !ready {
		return entry(s.name(), NotStartedState, nil)
	}

	client, err := github.NewClient(data.NewContext(ctx, db))
	if errors.Is(err, github.ErrNoApp) {
		return entry(s.name(), NotStartedState, nil)
	} else if err != nil {
		return entry(s.name(), UnknownState, err)
	}

	perms, err := client.AppPermissions(ctx)
	if err != nil {
		return entry(s.name(), UnknownState, err)
	}
	missing := github.MissingPermissions(perms)
	if len(missing) > 0 {
		return entry(s.name(), PartiallyReadyState,
			errors.New("app is missing permissions — "+strings.Join(missing, ", ")))
	}
	return entry(s.name(), FullyReadyState, nil)
}

func (appCredentials) Reset(ctx context.Context) error { return github.ClearAppKeys(ctx) }

type registryToken struct{}

func (registryToken) name() string { return stepRegistryToken }

// Check requires the one ghcr key — the registry the wizard covers; presence is
// the whole verdict, the token proves itself on the first publish
// (docs/spec/installation.md, "The registry token").
func (s registryToken) Check(ctx context.Context, db *sqlx.DB) Entry {
	return settingsBacked(ctx, db, s.name(), func(ctx context.Context) (map[string]string, error) {
		_, err := github.LoadRegistryToken(ctx, ghcrHost)
		return nil, err
	}, github.ErrNoRegistryToken)
}

func (registryToken) Reset(ctx context.Context) error {
	return github.ClearRegistryToken(ctx, ghcrHost)
}

type engine struct{}

func (engine) name() string { return stepEngine }

// Check reads the engine binding and surfaces it in values. While the setting is
// unset the not-started entry's values carry the deployment's DAGGER_ENGINE env
// seed, so the wizard panel pre-fills what infra provisioned — the save is what
// locks it in (docs/spec/installation.md, the engine step). Presence is the whole
// verdict: the check never dials.
func (s engine) Check(ctx context.Context, db *sqlx.DB) Entry {
	e := settingsBacked(ctx, db, s.name(), func(ctx context.Context) (map[string]string, error) {
		hosts, err := github.LoadEngineHosts(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{"hosts": hosts}, nil
	}, github.ErrNoEngineHosts)

	cfg := config.FromContext(ctx)
	if e.State == NotStartedState && cfg != nil {
		if seed := config.Get(cfg, buildengine.DaggerEngineConfig); seed != "" {
			e.Values = map[string]string{"hosts": seed}
		}
	}
	return e
}

func (engine) Reset(ctx context.Context) error { return github.ClearEngineHosts(ctx) }

type appInstalled struct{}

func (appInstalled) name() string { return stepAppInstalled }

// Check asks GitHub with the App's own credentials whether the bound org is among
// the App's installations — no session involved, the truth lives on GitHub. Missing
// prerequisites (schema, App credentials, org) read as not started: install is not
// the next move until they exist (docs/spec/installation.md, the state surface).
func (s appInstalled) Check(ctx context.Context, db *sqlx.DB) Entry {
	if db == nil {
		return entry(s.name(), UnknownState, errNoDatabase)
	}

	ready, err := settingsSchemaReady(ctx, db)
	if err != nil {
		return entry(s.name(), UnknownState, err)
	}
	if !ready {
		return entry(s.name(), NotStartedState, nil)
	}

	dataCtx := data.NewContext(ctx, db)
	org, err := github.LoadOrg(dataCtx)
	if errors.Is(err, github.ErrNoOrg) {
		return entry(s.name(), NotStartedState, nil)
	} else if err != nil {
		return entry(s.name(), UnknownState, err)
	}

	client, err := github.NewClient(dataCtx)
	if errors.Is(err, github.ErrNoApp) {
		return entry(s.name(), NotStartedState, nil)
	} else if err != nil {
		return entry(s.name(), UnknownState, err)
	}

	installed, err := client.Installations(ctx)
	if err != nil {
		return entry(s.name(), UnknownState, err)
	}
	for _, candidate := range installed {
		if strings.EqualFold(candidate.Login, org) {
			return entry(s.name(), FullyReadyState, nil)
		}
	}
	return entry(s.name(), NotStartedState, nil)
}

// Reset is a no-op: the step holds no server-side values — undoing it is
// uninstalling the App on GitHub, which the next check simply observes.
func (appInstalled) Reset(context.Context) error { return nil }

type claimed struct{}

func (claimed) name() string { return stepClaimed }

func (s claimed) Check(ctx context.Context, db *sqlx.DB) Entry {
	return settingsBacked(ctx, db, s.name(), func(ctx context.Context) (map[string]string, error) {
		_, err := Load(ctx)
		return nil, err
	}, ErrNotInstalled)
}

// Reset empties the install.* values — the not-yet-claimed state, so a fresh claim
// converges (its put-if-absent row already reads empty as unclaimed).
func (claimed) Reset(ctx context.Context) error {
	return data.Run(ctx, func(s data.Scope) error {
		for _, key := range installKeys {
			upsert := &settings.Upsert{Key: key, Value: ""}
			if err := upsert.Execute(s.Context(), &settings.Settings{}); err != nil {
				return err
			}
		}
		return nil
	})
}

// settingsBacked is the shared shape of the settings-reading checks: probe for
// the settings schema first — the probe always parses, so a pre-install server never
// sends a failing statement — then read, folding the reader's absent sentinel into
// not started. The read returns the step's non-secret values for the ready Entry.
func settingsBacked(ctx context.Context, db *sqlx.DB, name string,
	read func(context.Context) (map[string]string, error), absent error) Entry {
	if db == nil {
		return entry(name, UnknownState, errNoDatabase)
	}

	ready, err := settingsSchemaReady(ctx, db)
	if err != nil {
		return entry(name, UnknownState, err)
	}
	if !ready {
		return entry(name, NotStartedState, nil)
	}

	values, err := read(data.NewContext(ctx, db))
	if errors.Is(err, absent) {
		return entry(name, NotStartedState, nil)
	} else if err != nil {
		return entry(name, UnknownState, err)
	}

	e := entry(name, FullyReadyState, nil)
	e.Values = values
	return e
}

func settingsSchemaReady(ctx context.Context, db *sqlx.DB) (bool, error) {
	var ready bool
	err := db.GetContext(ctx, &ready, `SELECT to_regclass('public.settings') IS NOT NULL`)
	return ready, err
}
