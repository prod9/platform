package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var errFold = errors.New("fold failed")

func TestAccMintsTheOutcomeFromTheStream(t *testing.T) {
	acc, at := &AccObserver{}, time.Now()

	acc.StepStarted("web", "build", at)
	acc.StepDone("web", "build", at, nil)
	acc.ImageBuilt("web", "ghcr.io/p9/web", at)
	acc.RunDone("web", at, nil)

	require.Equal(t, "ghcr.io/p9/web", acc.Image())
	require.Empty(t, acc.Hash(), "a build that never published has no hash")
	require.NoError(t, acc.Err())
}

func TestAccKeepsTheFailureThatEndedTheRun(t *testing.T) {
	acc, at := &AccObserver{}, time.Now()

	acc.StepDone("web", "build", at, errFold)
	acc.RunDone("web", at, errFold)

	require.ErrorIs(t, acc.Err(), errFold)
	require.Empty(t, acc.Image(), "a failed run built no image")
}

func TestAccTakesTheHashFromThePublish(t *testing.T) {
	acc, at := &AccObserver{}, time.Now()

	acc.ImageBuilt("web", "ghcr.io/p9/web", at)
	acc.RunDone("web", at, nil)
	acc.Published("web", "ghcr.io/p9/web:v1", "sha256:abc", at)

	require.Equal(t, "ghcr.io/p9/web:v1", acc.Image(), "publishing renames the image")
	require.Equal(t, "sha256:abc", acc.Hash())
}

func TestTeeForwardsEveryCallbackToEveryChild(t *testing.T) {
	first, second, at := &recorder{}, &recorder{}, time.Now()

	obs := Tee(first, second)
	obs.StepStarted("web", "build", at)
	obs.StepDone("web", "build", at, nil)
	obs.ImageBuilt("web", "ghcr.io/p9/web", at)
	obs.Published("web", "ghcr.io/p9/web", "sha256:abc", at)
	obs.RunDone("web", at, nil)

	want := []string{
		"started web/build", "done web/build",
		"built web/ghcr.io/p9/web",
		"published web/ghcr.io/p9/web/sha256:abc",
		"rundone web",
	}
	require.Equal(t, want, first.lines)
	require.Equal(t, want, second.lines)
}
