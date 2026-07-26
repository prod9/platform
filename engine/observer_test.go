package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var errFold = errors.New("fold failed")

func TestAccMintsTheOutcomeFromTheStream(t *testing.T) {
	acc, out := accumulate(nil)
	at := time.Now()

	acc.StepStarted("web", "build", at)
	acc.StepDone("web", "build", at, nil)
	acc.ImageBuilt("web", "ghcr.io/p9/web", at)
	acc.RunDone("web", at, nil)

	require.Equal(t, "ghcr.io/p9/web", out.image)
	require.Empty(t, out.hash, "a build that never published has no hash")
	require.NoError(t, out.err)
}

func TestAccKeepsTheFailureThatEndedTheRun(t *testing.T) {
	acc, out := accumulate(nil)
	at := time.Now()

	acc.StepDone("web", "build", at, errFold)
	acc.RunDone("web", at, errFold)

	require.ErrorIs(t, out.err, errFold)
	require.Empty(t, out.image, "a failed run built no image")
}

func TestAccTakesTheHashFromThePublish(t *testing.T) {
	acc, out := accumulate(nil)
	at := time.Now()

	acc.ImageBuilt("web", "ghcr.io/p9/web", at)
	acc.RunDone("web", at, nil)
	acc.Published("web", "ghcr.io/p9/web:v1", "sha256:abc", at)

	require.Equal(t, "ghcr.io/p9/web:v1", out.image, "publishing renames the image")
	require.Equal(t, "sha256:abc", out.hash)
}

func TestAccFoldsWhileAlsoFeedingTheCaller(t *testing.T) {
	caller := &recorder{}
	acc, out := accumulate(caller)
	at := time.Now()

	acc.ImageBuilt("web", "ghcr.io/p9/web", at)
	acc.RunDone("web", at, errFold)

	require.Equal(t, "ghcr.io/p9/web", out.image, "composing a caller must not drop the fold")
	require.ErrorIs(t, out.err, errFold)
	require.Equal(t, []string{"built web/ghcr.io/p9/web", "rundone web"}, caller.lines)
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
