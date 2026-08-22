package system

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppMountsSystemController(t *testing.T) {
	require.Len(t, App.App().Middlewares(), 1)
	require.Len(t, App.App().Controllers(), 1)
	require.IsType(t, SystemCtr{}, App.App().Controllers()[0])
}
