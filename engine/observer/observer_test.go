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

func (r *recorder) StepStart(unit, step string, _ time.Time) {
	r.lines = append(r.lines, "started "+unit+"/"+step)
}

func (r *recorder) StepOutput(unit, step string, _ time.Time, stdout, stderr string) {
	r.lines = append(r.lines, "output "+unit+"/"+step+"/"+stdout+"/"+stderr)
}

func (r *recorder) StepDone(unit, step string, _ time.Time, err error) {
	r.lines = append(r.lines, "done "+unit+"/"+step)
	r.errs = append(r.errs, err)
}

func (r *recorder) RunStart(unit string, _ time.Time) { r.lines = append(r.lines, "runstart "+unit) }

func (r *recorder) CloneStart(unit string, _ time.Time) {
	r.lines = append(r.lines, "clonestart "+unit)
}

func (r *recorder) CloneDone(unit string, _ time.Time, err error) {
	r.lines = append(r.lines, "clonedone "+unit)
	r.errs = append(r.errs, err)
}

func (r *recorder) ConfigStart(unit string, _ time.Time) {
	r.lines = append(r.lines, "configstart "+unit)
}

func (r *recorder) ConfigDone(unit, host string, _ time.Time, err error) {
	r.lines = append(r.lines, "configdone "+unit+"/"+host)
	r.errs = append(r.errs, err)
}

func (r *recorder) PublishStart(unit string, _ time.Time) {
	r.lines = append(r.lines, "publishstart "+unit)
}

func (r *recorder) PublishDone(unit string, _ time.Time, err error) {
	r.lines = append(r.lines, "publishdone "+unit)
	r.errs = append(r.errs, err)
}

func (r *recorder) RunDone(unit, image, hash string, _ time.Time, err error) {
	r.lines = append(r.lines, "rundone "+unit+"/"+image+"/"+hash)
	r.errs = append(r.errs, err)
}

func TestAccMintsTheOutcomeFromTheStream(t *testing.T) {
	acc, out := Accumulate(nil)
	at := time.Now()

	acc.StepStart("web", "build", at)
	acc.StepDone("web", "build", at, nil)
	acc.RunDone("web", "ghcr.io/p9/web", "", at, nil)

	require.Equal(t, "ghcr.io/p9/web", out.Image)
	require.Empty(t, out.Hash, "a build that never published has no hash")
	require.NoError(t, out.Err)
}

func TestAccKeepsTheFailureThatEndedTheRun(t *testing.T) {
	acc, out := Accumulate(nil)
	at := time.Now()

	acc.StepDone("web", "build", at, errFold)
	acc.RunDone("web", "", "", at, errFold)

	require.ErrorIs(t, out.Err, errFold)
	require.Empty(t, out.Image, "a failed run built no image")
}

func TestAccTakesTheHashFromThePublish(t *testing.T) {
	acc, out := Accumulate(nil)
	at := time.Now()

	acc.RunDone("web", "ghcr.io/p9/web:v1", "sha256:abc", at, nil)

	require.Equal(t, "ghcr.io/p9/web:v1", out.Image, "publishing renames the image")
	require.Equal(t, "sha256:abc", out.Hash)
}

func TestAccFoldsWhileAlsoFeedingTheCaller(t *testing.T) {
	caller := &recorder{}
	acc, out := Accumulate(caller)
	at := time.Now()

	acc.RunDone("web", "ghcr.io/p9/web", "", at, errFold)

	require.Equal(t, "ghcr.io/p9/web", out.Image, "composing a caller must not drop the fold")
	require.ErrorIs(t, out.Err, errFold)
	require.Equal(t, []string{"rundone web/ghcr.io/p9/web/"}, caller.lines)
}

// Step output belongs to its consumer; accumulating it would make Outcome grow
// with every line a build prints.
func TestAccPassesCapturedOutputThroughWithoutFoldingIt(t *testing.T) {
	caller := &recorder{}
	acc, out := Accumulate(caller)
	at := time.Now()

	acc.StepOutput("web", "build", at, "compiled", "warning")
	acc.RunDone("web", "", "", at, nil)

	require.Equal(t, []string{"output web/build/compiled/warning", "rundone web//"}, caller.lines)
	require.Equal(t, Outcome{}, *out, "captured output is nobody's outcome")
}

func TestTeeForwardsEveryCallbackToEveryChild(t *testing.T) {
	first, second, at := &recorder{}, &recorder{}, time.Now()

	obs := Tee(first, second)
	obs.RunStart("web", at)
	obs.CloneStart("web", at)
	obs.CloneDone("web", at, nil)
	obs.ConfigStart("web", at)
	obs.ConfigDone("web", "local", at, nil)
	obs.StepStart("web", "build", at)
	obs.StepOutput("web", "build", at, "compiled", "warning")
	obs.StepDone("web", "build", at, nil)
	obs.PublishStart("web", at)
	obs.PublishDone("web", at, nil)
	obs.RunDone("web", "ghcr.io/p9/web", "sha256:abc", at, nil)
	want := []string{
		"runstart web", "clonestart web", "clonedone web", "configstart web", "configdone web/local",
		"started web/build", "output web/build/compiled/warning", "done web/build",
		"publishstart web", "publishdone web", "rundone web/ghcr.io/p9/web/sha256:abc",
	}
	require.Equal(t, want, first.lines)
	require.Equal(t, want, second.lines)
}
