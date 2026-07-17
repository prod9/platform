package timeref

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIsValid(t *testing.T) {
	require.True(t, IsValid(refFormat))
}

func TestParse(t *testing.T) {
	moment, err := Parse("v202306291214")
	require.NoError(t, err)
	require.Equal(t, time.Date(2023, 6, 29, 12, 14, 0, 0, time.UTC), moment)

	_, err = Parse("v20260717")
	require.Error(t, err)
	_, err = Parse("v0.9.10")
	require.Error(t, err)
}
