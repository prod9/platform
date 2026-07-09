package buildlog

import (
	"errors"
	"log/slog"
	"strings"

	"dagger.io/dagger"
)

// enrichErr expands a wrapped *dagger.ExecError into a structured error whose fields the
// slog handler renders. The raw error only carries "exit code: N" in its message and
// drops the failed command's output; enrichErr surfaces cmd, exit, and stderr as attrs.
// Non-exec errors pass through unchanged.
func enrichErr(err error) error {
	var execErr *dagger.ExecError
	if !errors.As(err, &execErr) {
		return err
	} else {
		return &enrichedExecError{err, execErr}
	}
}

type enrichedExecError struct {
	err     error
	execErr *dagger.ExecError
}

func (e *enrichedExecError) Error() string { return e.err.Error() }
func (e *enrichedExecError) Unwrap() error { return e.err }

// LogValue expands the failed exec into structured fields for the slog handler to render.
func (e *enrichedExecError) LogValue() slog.Value {
	if e.execErr == nil {
		return slog.Value{}
	}

	var attrs []slog.Attr
	if code := e.execErr.ExitCode; code >= 0 {
		attrs = append(attrs, slog.Int("exit", code))
	}
	if len(e.execErr.Cmd) > 0 {
		attrs = append(attrs, slog.String("cmd", strings.Join(e.execErr.Cmd, " ")))
	}
	if stderr := strings.TrimSpace(e.execErr.Stderr); stderr != "" {
		attrs = append(attrs, slog.String("stderr", stderr))
	}
	if stdout := strings.TrimSpace(e.execErr.Stdout); stdout != "" {
		attrs = append(attrs, slog.String("stdout", stdout))
	}
	return slog.GroupValue(attrs...)
}
