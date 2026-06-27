package dsl

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func decodeDocs(t *testing.T, src string) []Doc {
	t.Helper()
	dec := yaml.NewDecoder(strings.NewReader(src))

	var docs []Doc
	for {
		var d Doc
		err := dec.Decode(&d)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if d != nil {
			docs = append(docs, d)
		}
	}
	return docs
}

func mustApply(t *testing.T, directives string, docs []Doc) []Doc {
	t.Helper()
	out, err := Apply(directives, Options{Docs: docs})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return out
}

// cert-manager: scope to a named Deployment, append controller flags idempotently.
func TestApplyCertManager(t *testing.T) {
	src := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cert-manager
spec:
  template:
    spec:
      containers:
        - name: cert-manager-controller
          args:
            - --v=2
`
	directives := `
# focus down to the controller container, then patch it relatively
focus .[].kind "Deployment"
focus .metadata.name "cert-manager"
focus .spec.template.spec.containers.[].name "cert-manager-controller"
append-if-absent .args "--enable-gateway-api"
append-if-absent .args "--feature-gates=ListenerSets=true"
`
	want := []any{"--v=2", "--enable-gateway-api", "--feature-gates=ListenerSets=true"}

	out := mustApply(t, directives, decodeDocs(t, src))
	argsPath := mustPath(t, ".spec.template.spec.containers[0].args")
	got, _ := Get(out[0], argsPath)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}

	// idempotent: a second apply changes nothing
	out2 := mustApply(t, directives, out)
	got2, _ := Get(out2[0], argsPath)
	if !reflect.DeepEqual(got2, want) {
		t.Fatalf("re-apply args = %#v, want %#v", got2, want)
	}
}

func TestApplyCumulativeFocus(t *testing.T) {
	src := `
kind: Deployment
metadata:
  name: a
---
kind: Deployment
metadata:
  name: b
`
	directives := `
focus .[].kind "Deployment"
focus .metadata.name "a"
set .marked "true"
`
	out := mustApply(t, directives, decodeDocs(t, src))

	// A quoted value is a string literal.
	if v, ok := Get(out[0], mustPath(t, ".marked")); !ok || v != "true" {
		t.Errorf("doc a .marked = %#v (ok=%v), want \"true\"", v, ok)
	}
	if _, ok := Get(out[1], mustPath(t, ".marked")); ok {
		t.Error("doc b should not be marked")
	}
}

func TestApplyResetAndKindChange(t *testing.T) {
	src := `
kind: Deployment
metadata:
  name: nginx-gateway
spec:
  replicas: 1
---
kind: NginxProxy
metadata:
  name: nginx-gateway
`
	directives := `
focus .[].kind "Deployment"
set .kind "DaemonSet"
remove .spec.replicas

reset
focus .[].kind "NginxProxy"
set-if-absent .spec.serverTokens "off"
`
	out := mustApply(t, directives, decodeDocs(t, src))

	if v, _ := Get(out[0], mustPath(t, ".kind")); v != "DaemonSet" {
		t.Errorf("doc 0 kind = %v, want DaemonSet", v)
	}
	if _, ok := Get(out[0], mustPath(t, ".spec.replicas")); ok {
		t.Error("doc 0 replicas should be removed")
	}
	if v, _ := Get(out[1], mustPath(t, ".spec.serverTokens")); v != "off" {
		t.Errorf("doc 1 serverTokens = %v, want off", v)
	}
}

func TestApplySetIfAbsentKeepsExisting(t *testing.T) {
	src := `
kind: NginxProxy
spec:
  serverTokens: on
`
	out := mustApply(t, "focus .[].kind \"NginxProxy\"\nset-if-absent .spec.serverTokens \"off\"", decodeDocs(t, src))
	if v, _ := Get(out[0], mustPath(t, ".spec.serverTokens")); v != "on" {
		t.Errorf("serverTokens = %v, want on (unchanged)", v)
	}
}

func TestApplyRemoveDoc(t *testing.T) {
	src := `
kind: ConfigMap
metadata:
  name: keep
---
kind: Secret
metadata:
  name: argocd-secret
`
	directives := `
focus .[].kind "Secret"
focus .metadata.name "argocd-secret"
remove-doc
`
	out := mustApply(t, directives, decodeDocs(t, src))

	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if v, _ := Get(out[0], mustPath(t, ".kind")); v != "ConfigMap" {
		t.Errorf("remaining doc kind = %v, want ConfigMap", v)
	}
}

// Value typing: a bare token is a variable reference (native type), a quoted
// token is a string literal.
func TestApplySetValueTyping(t *testing.T) {
	src := "kind: Deployment\n"
	directives := "focus .[].kind \"Deployment\"\nset .spec.replicas replicas\nset .spec.note \"3\""

	out, err := Apply(directives, Options{Docs: decodeDocs(t, src), Vars: Vars{"replicas": 3}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if v, _ := Get(out[0], mustPath(t, ".spec.replicas")); v != 3 {
		t.Errorf("replicas = %#v, want int 3 (bare var reference)", v)
	}
	if v, _ := Get(out[0], mustPath(t, ".spec.note")); v != "3" {
		t.Errorf("note = %#v, want string \"3\" (quoted literal)", v)
	}
}

func TestApplyUnknownVerb(t *testing.T) {
	if _, err := Apply("frobnicate .x", Options{Docs: decodeDocs(t, "kind: X\n")}); err == nil {
		t.Error("expected error for unknown verb")
	}
}
