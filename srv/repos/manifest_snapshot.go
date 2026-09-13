package repos

type ManifestSnapshot struct {
	ID     int64  `db:"id"`
	RepoID int64  `db:"repo_id"`
	SHA    string `db:"sha"`
}
