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

func TestRootCommandServerOwnsDataMigrate(t *testing.T) {
	migrateCmd, remaining, err := rootCmd.Find([]string{"srv", "data", "migrate"})
	require.NoError(t, err)
	require.Empty(t, remaining)
	require.Equal(t, "platform srv data migrate", migrateCmd.CommandPath())
}

func TestRootCommandHasCollectedWorker(t *testing.T) {
	workerCmd, remaining, err := rootCmd.Find([]string{"worker"})
	require.NoError(t, err)
	require.Empty(t, remaining)
	require.Equal(t, "platform worker", workerCmd.CommandPath())
}

func TestServerStartupRequiresSecretWithoutDatabase(t *testing.T) {
	t.Setenv("SECRET", "")
	t.Setenv("DATABASE_URL", "://invalid")
	cmd := buildSrvCmd()
	cmd.SetContext(t.Context())

	err := cmd.PreRunE(cmd, nil)
	require.EqualError(t, err, "srv: SECRET is required")
}

func TestServerStartupAllowsSecretWithoutDatabase(t *testing.T) {
	t.Setenv("SECRET", "test-secret")
	t.Setenv("DATABASE_URL", "://invalid")
	cmd := buildSrvCmd()
	cmd.SetContext(t.Context())

	require.NoError(t, cmd.PreRunE(cmd, nil))
}
