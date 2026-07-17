// Package install is the platform server's installer fragment: the singleton install
// record, the ordered install-state surface (GET /api/install), and the migrations
// remediation. Boot mounts this fragment only while the server is not completely
// installed; product fragments have zero install awareness.
package install

import (
	"context"
	"errors"
	"time"

	"fx.prodigy9.co/data"
	"platform.prodigy9.co/srv/pgerr"
)

// ErrNotInstalled reports that the server is not bound to an org yet — either the
// singleton row is absent or its table has not been migrated in.
var ErrNotInstalled = errors.New("install: not installed")

// Record is the singleton install row binding the server to one GitHub org. App
// credentials are deliberately absent — they live in fx config, never in the DB.
type Record struct {
	ID                int64     `db:"id"`
	OrgID             int64     `db:"org_id"`
	OrgLogin          string    `db:"org_login"`
	InstallationID    int64     `db:"installation_id"`
	InstalledByUserID int64     `db:"installed_by_user_id"`
	InstalledByLogin  string    `db:"installed_by_login"`
	InstalledAt       time.Time `db:"installed_at"`
}

// Load reads the singleton install record. A missing row and a missing table both
// mean "not installed" — the table is absent until its migration runs, which is a
// valid pre-install state, not a failure.
func Load(ctx context.Context) (*Record, error) {
	record := &Record{}
	err := data.Get(ctx, record, `SELECT * FROM installations WHERE id = 1`)
	if data.IsNoRows(err) || pgerr.IsUndefinedTable(err) {
		return nil, ErrNotInstalled
	} else if err != nil {
		return nil, err
	}
	return record, nil
}
