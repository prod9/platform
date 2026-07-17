package github

import (
	"fmt"
	"regexp"
)

// CheckRepoPath admits only names GitHub itself allows (letters, digits, '-', plus
// '._' in repo names, never leading '.') — owner/repo land in filesystem paths and
// API URLs, so the whitelist is what keeps a hostile payload from escaping them.
func CheckRepoPath(owner, repo string) error {
	if !repoNamePattern.MatchString(owner) || !repoNamePattern.MatchString(repo) {
		return fmt.Errorf("github: invalid repo path: %q/%q", owner, repo)
	}
	return nil
}

var repoNamePattern = regexp.MustCompile(`^[A-Za-z0-9-][A-Za-z0-9._-]*$`)
