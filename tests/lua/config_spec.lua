local config = require("games.config")

local T = {}

function T.credentials_are_optional_but_type_checked()
  for _, opts in ipairs({ {}, { username = "alice" }, { password = "pw" } }) do
    local ok, err = pcall(config.resolve, opts)
    assert(ok, err)
  end
  for _, opts in ipairs({ { username = 42 }, { password = true } }) do
    local ok = pcall(config.resolve, opts)
    assert(not ok, "accepted " .. vim.inspect(opts))
  end
end

return T
