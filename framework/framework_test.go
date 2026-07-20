package framework

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	r "github.com/stretchr/testify/require"
)

// sourceFileFor and typeIdentFor derive, from a framework's Name(), the two identifiers the
// package's uniform shape requires: the file it lives in and the Go type declaring it.
// Comparison is case-insensitive so acronym families (PNPM) need no spelling table.
func sourceFileFor(name string) string {
	return strings.ReplaceAll(name, "/", "_") + ".go"
}

func typeIdentFor(name string) string {
	return strings.ReplaceAll(name, "/", "")
}

// TestFrameworkShapeChecksBite guards the guard: the derivation must reject a framework
// whose type or file drifts from its Name(), or TestFrameworkShape below passes vacuously.
func TestFrameworkShapeChecksBite(t *testing.T) {
	r.Equal(t, "pnpm_static.go", sourceFileFor("pnpm/static"))
	r.NotEqual(t, "infra.go", sourceFileFor("platform/infra"))

	r.Equal(t, "pnpmstatic", typeIdentFor("pnpm/static"))
	r.NotEqual(t, strings.ToLower("Infra"), typeIdentFor("platform/infra"))
}

// TestFrameworkShape is the uniform-shape law: every framework's Go type and source file are
// mechanically derived from its Name(), so go/basic is GoBasic in go_basic.go and nothing
// else. It exists because the shape kept drifting back — orphan files holding one framework's
// data, types named for a family they no longer matched. A new framework that skips the
// convention fails here rather than in review.
func TestFrameworkShape(t *testing.T) {
	for _, fw := range knownFrameworks {
		name := fw.Name()

		typename := reflect.TypeOf(fw).Name()
		r.Equal(t, typeIdentFor(name), strings.ToLower(typename),
			"framework %q must be declared by type %q", name, typeIdentFor(name))

		srcfile := sourceFileFor(name)
		_, err := os.Stat(filepath.Join(".", srcfile))
		r.NoError(t, err, "framework %q must live in %s", name, srcfile)
	}
}
