local config = require("games.config")

local T = {}

function T.credentials_are_optional()
  local resolved = config.resolve({})
  assert(resolved.username == nil and resolved.password == nil, vim.inspect(resolved))
end

function T.credentials_of_the_wrong_type_still_error()
  for _, opts in ipairs({ { username = 42 }, { password = true } }) do
    local ok = pcall(config.resolve, opts)
    assert(not ok, "accepted " .. vim.inspect(opts))
  end
end

return T
