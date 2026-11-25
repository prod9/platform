local List = {}
return {
  from = "rock",
  list = function(items) return setmetatable(items, List) end,
}
