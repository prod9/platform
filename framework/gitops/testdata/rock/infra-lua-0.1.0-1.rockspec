package = "infra-lua"
version = "0.1.0-1"
source = { url = "git+https://github.com/prod9/infra-lua" }
build = {
  type = "builtin",
  modules = {
    ["infra.tree"] = "src/infra/tree.lua",
  },
}
