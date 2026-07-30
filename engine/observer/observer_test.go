package observer

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var errFold = errors.New("fold failed")

// recorder keeps every callback as a readable line, so a test asserts on the sequence a
// caller would see rather than on five separate counters.
type recorder struct {
	lines []string
	errs  []error
}

func (r *recorder) StepStarted(unit, step string, _ time.Time) {
	r.lines = append(r.lines, "started "+unit+"/"+step)
}

func (r *recorder) StepOutput(unit, step string, _ time.Time, stdout, stderr string) {
	r.lines = append(r.lines, "output "+unit+"/"+step+"/"+stdout+"/"+stderr)
}

func (r *recorder) StepDone(unit, step string, _ time.Time, err error) {
	r.lines = append(r.lines, "done "+unit+"/"+step)
	r.errs = append(r.errs, err)
}

func (r *recorder) ImageBuilt(unit, image string, _ time.Time) {
	r.lines = append(r.lines, "built "+unit+"/"+image)
}

func (r *recorder) Published(unit, image, hash string, _ time.Time) {
	r.lines = append(r.lines, "published "+unit+"/"+image+"/"+hash)
}

func (r *recorder) RunDone(unit string, _ time.Time, err error) {
	r.lines = append(r.lines, "rundone "+unit)
	r.errs = append(r.errs, err)
}

func TestAccMintsTheOutcomeFromTheStream(t *testing.T) {
	acc, out := Accumulate(nil)
	at := time.Now()

	acc.StepStarted("web", "build", at)
	acc.StepDone("web", "build", at, nil)
	acc.ImageBuilt("web", "ghcr.io/p9/web", at)
	acc.RunDone("web", at, nil)

	require.Equal(t, "ghcr.io/p9/web", out.Image)
	require.Empty(t, out.Hash, "a build that never published has no hash")
	require.NoError(t, out.Err)
}

func TestAccKeepsTheFailureThatEndedTheRun(t *testing.T) {
	acc, out := Accumulate(nil)
	at := time.Now()

	acc.StepDone("web", "build", at, errFold)
	acc.RunDone("web", at, errFold)

	require.ErrorIs(t, out.Err, errFold)
	require.Empty(t, out.Image, "a failed run built no image")
}

func TestAccTakesTheHashFromThePublish(t *testing.T) {
	acc, out := Accumulate(nil)
	at := time.Now()

	acc.ImageBuilt("web", "ghcr.io/p9/web", at)
	acc.RunDone("web", at, nil)
	acc.Published("web", "ghcr.io/p9/web:v1", "sha256:abc", at)

	require.Equal(t, "ghcr.io/p9/web:v1", out.Image, "publishing renames the image")
	require.Equal(t, "sha256:abc", out.Hash)
}

func TestAccFoldsWhileAlsoFeedingTheCaller(t *testing.T) {
	caller := &recorder{}
	acc, out := Accumulate(caller)
	at := time.Now()

	acc.ImageBuilt("web", "ghcr.io/p9/web", at)
	acc.RunDone("web", at, errFold)

	require.Equal(t, "ghcr.io/p9/web", out.Image, "composing a caller must not drop the fold")
	require.ErrorIs(t, out.Err, errFold)
	require.Equal(t, []string{"built web/ghcr.io/p9/web", "rundone web"}, caller.lines)
}

// TestAccPassesCapturedOutputThroughWithoutFoldingIt pins the split the sixth callback
// rests on: a step's output is for whoever stores it, and folding it would make Outcome
// grow with every line a build prints.
func TestAccPassesCapturedOutputThroughWithoutFoldingIt(t *testing.T) {
	caller := &recorder{}
	acc, out := Accumulate(caller)
	at := time.Now()

	acc.StepOutput("web", "build", at, "compiled", "warning")
	acc.RunDone("web", at, nil)

	require.Equal(t, []string{"output web/build/compiled/warning", "rundone web"}, caller.lines)
	require.Equal(t, Outcome{}, *out, "captured output is nobody's outcome")
}

func TestTeeForwardsEveryCallbackToEveryChild(t *testing.T) {
	first, second, at := &recorder{}, &recorder{}, time.Now()

	obs := Tee(first, second)
	obs.StepStarted("web", "build", at)
	obs.StepOutput("web", "build", at, "compiled", "warning")
	obs.StepDone("web", "build", at, nil)
	obs.ImageBuilt("web", "ghcr.io/p9/web", at)
	obs.Published("web", "ghcr.io/p9/web", "sha256:abc", at)
	obs.RunDone("web", at, nil)

	want := []string{
		"started web/build", "output web/build/compiled/warning", "done web/build",
		"built web/ghcr.io/p9/web",
		"published web/ghcr.io/p9/web/sha256:abc",
		"rundone web",
	}
	require.Equal(t, want, first.lines)
	require.Equal(t, want, second.lines)
}
