# `render` validates every document against its Kubernetes schema

- **Date:** 2026-11-25
- **PR:** manual
- **Status:** accepted

Current contract: [Lua render](../spec/lua-render.md), "Validation".

## Decision

Before `render` returns, each document is checked against the schema for its own
`apiVersion` and `kind`: for built-in kinds, the schema in Kubernetes' published OpenAPI
v3 files; for custom resources, the `openAPIV3Schema` embedded in the
`CustomResourceDefinition`. Schemas are fetched once into the XDG cache. The check uses a
generic JSON Schema library, `google/jsonschema-go`. `platform` links no Kubernetes
library and has no setting for a Kubernetes version. A failing document stops the render
before anything is written.

What render does with a kind for which no schema is available is open, pending the
validation spike.

chakrit ruled the shape: "i don't want more settings"; against a convention in its place:
"convention wouldn't work here. that's stupid"; on the dependency: "i mean no kube-related
lib, but perhaps we can do yamlschema lib or something?", then "good". He proposed the
library as a question, "google publishes https://github.com/google/jsonschema-go this
seemed much better?", so it is the chosen library pending the spike's confirmation that it
reads Kubernetes' schemas.

## Rationale

**Why validate at all.** CUE checked a definition's own inputs but left defs open, so a
misspelled field in a raw resource or a patch reached the cluster. Lua checks less:
`infra-lua` checks input names and Lua types, and nothing checks the fields of a raw
resource or an edited upstream document. Without a schema check the first reader of a typo
is the API server, after the image is published and Flux has tried to apply it.

**Why not a convention.** This repository usually covers a remainder by a stated
convention rather than code. Here a convention would mean "check your manifests before you
publish", which nobody can follow by reading Lua, and which is the failure the check
exists for.

**Why no Kubernetes library and no version setting.** The Kubernetes client libraries
would bring a large dependency tree into a CLI for one job, schema lookup, that published
files already serve. A Kubernetes-version setting would be one more key every repo carries
and keeps current. A schema per `apiVersion` and `kind` is what a document already names.

**Why stop before writing.** A partly written tree, or an image built from one, is worse
than none: Flux prunes what an artifact omits.

## Consequences

- Render reaches the network for schemas on a cold cache, as it already does for rocks and
  upstream manifests.
- The spike settles where the OpenAPI files come from, how much the OpenAPI 3.0 dialect
  needs translating for a draft 2020-12 validator, and the missing-schema case; those land
  in the spec before validation is built.
