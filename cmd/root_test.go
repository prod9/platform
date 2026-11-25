package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/internal/termlog"
)

func TestExecuteReportsCleanupFailureWithChildStatus(t *testing.T) {
	previousCommand, previousStderr := rootCmd, os.Stderr
	path := filepath.Join(t.TempDir(), "stderr")
	output, err := os.Create(path)
	require.NoError(t, err)
	os.Stderr = output
	termlog.SetVerbosity(0)
	t.Cleanup(func() {
		rootCmd, os.Stderr = previousCommand, previousStderr
		termlog.SetVerbosity(0)
		require.NoError(t, output.Close())
	})
	rootCmd = &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			return errors.Join(exitError{code: 7}, errors.New("session cleanup failed"))
		}}
	rootCmd.SetArgs([]string{})

	require.Equal(t, 7, Execute())
	logged, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(logged), "session cleanup failed")
}
