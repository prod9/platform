-- Definitions the testbed's apps share, loaded with require "defaults".
local defaults = {}

function defaults.labels(name)
  return {app = name}
end

return defaults
