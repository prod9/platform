// Package git is platform's one git-exec boundary. Package-level funcs answer
// repo-shape questions (IsRoot) and run project-independent commands (Run); Context
// runs git against a project's repository, caching the per-process constants.
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run executes one git command in dir, returning trimmed stdout; git's stderr is
// captured into the error instead of streamed. For callers with no project config —
// server-side mirrors and worktrees — where errors surface through logs, not a
// terminal.
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	outbuf, errbuf := &strings.Builder{}, &strings.Builder{}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = outbuf, errbuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git: %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(errbuf.String()))
	}

	return strings.TrimSpace(outbuf.String()), nil
}

// IsRoot reports whether dir is itself a git repo root — a .git entry directly in dir,
// without walking up. A nested standalone repo (an infra repo developed in-place under
// another checkout) must have its own .git; the parent's does not count.
func IsRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
