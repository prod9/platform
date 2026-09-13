package builds

import "time"

// Build is one recorded request to build a repo at a commit — who asked and what for,
// never how it went. It is written once and never updated; how the build went lives in
// its BuildEvent stream.
type Build struct {
	ID         int64     `db:"id"`
	Trigger    Trigger   `db:"trigger"`
	RetryOf    int64     `db:"retry_of"`
	UserID     int64     `db:"user_id"`
	RepoID     int64     `db:"repo_id"`
	ManifestID int64     `db:"manifest_id"`
	Owner      string    `db:"owner"`
	Repo       string    `db:"repo"`
	CloneURL   string    `db:"clone_url"`
	Ref        string    `db:"ref"`
	SHA        string    `db:"sha"`
	CreatedAt  time.Time `db:"created_at"`
}
