package gitops_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"platform.prodigy9.co/framework/gitops"
)

// listMarkLua stands in for infra-lua's defs.tree: a private metatable marks a list.
const listMarkLua = `
local List = {}
local function list(items) return setmetatable(items, List) end
`

// encodeLua runs src, which returns a list of documents, with listMarkLua's mark.
func encodeLua(t *testing.T, src string) ([]byte, error) {
	t.Helper()
	state := newTestSandbox(t, t.TempDir())
	require.NoError(t, state.DoString(listMarkLua+src+"\nreturn docs, List"))
	docs := state.Get(-2)
	mark := state.CheckTable(-1)
	state.Pop(2)
	return gitops.EncodeDocuments(docs, mark)
}

// docs/spec/lua-render.md, "YAML": apiVersion, kind and metadata lead each document,
// every other key is sorted, and a list of documents is one multi-document file.
func TestEncodeDocumentsOrdersKeys(t *testing.T) {
	out, err := encodeLua(t, `
local docs = {
  {data = {b = "2", a = "1"}, immutable = true, metadata = {namespace = "x", name = "web"},
   kind = "ConfigMap", apiVersion = "v1"},
  {metadata = {name = "x"}, kind = "Namespace", apiVersion = "v1"},
}`)
	require.NoError(t, err)
	require.Equal(t, `apiVersion: v1
kind: ConfigMap
metadata:
    name: web
    namespace: x
data:
    a: "1"
    b: "2"
immutable: true
---
apiVersion: v1
kind: Namespace
metadata:
    name: x
`, string(out))
}

// docs/spec/lua-render.md, "YAML": integral numbers are integers, and an empty table is
// a mapping unless infra-lua marks it as a list.
func TestEncodeDocumentsNumbersAndEmptyTables(t *testing.T) {
	out, err := encodeLua(t, `
local docs = {{kind = "X", spec = {
  replicas = 2, ratio = 0.5, big = 1e15, ports = {80, 443},
  empty_map = {}, empty_list = list {},
}}}`)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out, &doc))
	require.Equal(t, map[string]any{
		"kind": "X",
		"spec": map[string]any{
			"replicas":   2,
			"ratio":      0.5,
			"big":        1000000000000000,
			"ports":      []any{80, 443},
			"empty_map":  map[string]any{},
			"empty_list": []any{},
		},
	}, doc)
}

// docs/spec/lua-render.md, "YAML": a value with no YAML form fails, naming where it is.
func TestEncodeDocumentsRejectsBadValues(t *testing.T) {
	cases := map[string]string{
		"function": `local docs = {{kind = "X", spec = {template = {run = function() end}}}}`,
		"mixed":    `local docs = {{kind = "X", spec = {template = {run = {1, a = 2}}}}}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := encodeLua(t, src)
			require.ErrorContains(t, err, "[1].spec.template.run")
		})
	}
}
