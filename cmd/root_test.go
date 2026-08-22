package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
