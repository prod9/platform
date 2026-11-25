-- Plain Lua resources with no vendored library (docs/spec/testing.md): a repo module
-- through require, a var set in platform.toml, a var left at its default, and a file
-- holding one resource beside a file holding a list of them.
local defaults = require "defaults"
local vars = platform.vars {
  INFRA_BASIC_HOST = "infra-basic.example.com",
  INFRA_BASIC_REPLICAS = 1,
}

local name = "infra-basic"
local labels = defaults.labels(name)

return {
  ["objects.yaml"] = {
    {apiVersion = "v1", kind = "Namespace", metadata = {name = name}},
    {
      apiVersion = "apps/v1",
      kind = "Deployment",
      metadata = {name = name, namespace = name, labels = labels},
      spec = {
        replicas = vars.INFRA_BASIC_REPLICAS,
        selector = {matchLabels = labels},
        template = {
          metadata = {labels = labels},
          spec = {
            containers = {
              -- Committed image literal: desired state lives in git, never injected at
              -- render time.
              {name = name, image = "ghcr.io/prod9/infra-basic:v1.0.0", ports = {{containerPort = 8080}}},
            },
          },
        },
      },
    },
    {
      apiVersion = "v1",
      kind = "Service",
      metadata = {name = name, namespace = name},
      spec = {selector = labels, ports = {{port = 80, targetPort = 8080}}},
    },
  },
  ["route.yaml"] = {
    apiVersion = "gateway.networking.k8s.io/v1",
    kind = "HTTPRoute",
    metadata = {name = name, namespace = name},
    spec = {
      parentRefs = {{name = "infra-basic-gateway"}},
      hostnames = {vars.INFRA_BASIC_HOST},
      rules = {{backendRefs = {{name = name, port = 80}}}},
    },
  },
}
