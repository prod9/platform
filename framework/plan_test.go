package framework

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPlanSteps pins every framework's step order. Step granularity is log granularity —
// each exec-heavy stage earns its own Step so it is timed and its output flushed
// separately — so a change here is a change to what an operator sees during a build, not
// an implementation detail.
func TestPlanSteps(t *testing.T) {
	for _, c := range []struct {
		fw    Framework
		steps []Step
	}{
		{GoBasic{}, []Step{StepBase, StepDeps, StepTest, StepBuild, StepBuildRunner}},
		{GoWorkspace{}, []Step{StepBase, StepDeps, StepTest, StepBuild, StepBuildRunner}},
		{PNPMBasic{}, []Step{StepBase, StepDeps, StepBuild, StepBuildRunner}},
		{PNPMStatic{}, []Step{StepBase, StepDeps, StepBuild, StepBuildRunner}},
		{PNPMWorkspace{}, []Step{StepBase, StepDeps, StepBuild, StepBuildRunner}},
		{Dockerfile{}, []Step{StepBuild}},
		{PlatformInfra{}, []Step{StepDeps, StepBuild}},
	} {
		t.Run(c.fw.Name(), func(t *testing.T) {
			require.Equal(t, c.steps, c.fw.Plan(&BuildUnit{Framework: c.fw}))
		})
	}
}
