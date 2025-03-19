package project

import (
	"strings"
	"testing"
	"time"

	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/internal/timeouts"
)

var ModNames []string = strings.Split("alpha,beta,gamma,delta,epsilon", ",")

func TestProject_inferValues(t *testing.T) {
	proj := testProject(1)
	proj.inferValues()
	r.Equal(t, "ghcr.io/prod9/platform", proj.Modules[ModNames[0]].ImageName)

	proj = testProject(2)
	proj.inferValues()
	r.Equal(t, "ghcr.io/prod9/platform/"+ModNames[0], proj.Modules[ModNames[0]].ImageName)
	r.Equal(t, "ghcr.io/prod9/platform/"+ModNames[1], proj.Modules[ModNames[1]].ImageName)
}

func testProject(modCount int) *Project {
	proj := &Project{
		Repository: "github.com/prod9/platform",
		Modules:    map[string]*Module{},
	}

	for i := range modCount {
		name := ModNames[i]
		proj.Modules[name] = &Module{
			WorkDir: "./" + name,
			Timeout: timeouts.Timeout(5 * time.Minute),
			Builder: "go/basic",
		}
	}
	return proj
}
