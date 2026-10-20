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

function T.palette_and_highlights_are_validated()
  local accepted = { palette = "vivid", highlights = { GamesKey = { fg = "#123456" } } }
  local ok, err = pcall(config.resolve, accepted)
  assert(ok, err)
  for _, opts in ipairs({
    { palette = "neon" },
    { highlights = "GamesKey" },
    { highlights = { GamesKye = { fg = "#123456" } } },
    { highlights = { GamesKey = "#123456" } },
  }) do
    local rejected = pcall(config.resolve, opts)
    assert(not rejected, "accepted " .. vim.inspect(opts))
  end
end

return T
