local List = {}
return {
  from = "vendor",
  list = function(items) return setmetatable(items, List) end,
}
