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

function T.archive_dir_is_optional_and_expanded()
  assert(config.resolve({}).archive.dir == nil)
  local resolved = config.resolve({ archive = { dir = "~/games-archive" } }).archive.dir
  assert(resolved == vim.fs.joinpath(vim.env.HOME, "games-archive"), resolved)
  for _, archive in ipairs({ { dir = "" }, { dir = 42 }, "~/games" }) do
    assert(not pcall(config.resolve, { archive = archive }), "accepted " .. vim.inspect(archive))
  end
end

function T.board_and_assets_are_validated()
  local accepted = { board = { max_cell_height = 1 }, assets = { chess = "my.chess_art" } }
  local ok, err = pcall(config.resolve, accepted)
  assert(ok, err)
  for _, opts in ipairs({
    { board = { max_cell_height = 0 } },
    { board = { max_cell_height = 2.5 } },
    { assets = { chess = 1 } },
    { assets = "default" },
  }) do
    local rejected = pcall(config.resolve, opts)
    assert(not rejected, "accepted " .. vim.inspect(opts))
  end
end

function T.picker_and_keys_are_validated()
  for _, opts in ipairs({
    { picker = "fzf" },
    { picker = "select" },
    { keys = { dashboard = "<leader>gg", find = "<leader>gf" } },
  }) do
    local ok, err = pcall(config.resolve, opts)
    assert(ok, err)
  end
  for _, opts in ipairs({
    { picker = "mini" },
    { keys = "<leader>gg" },
    { keys = { finder = "<leader>gf" } },
    { keys = { dashboard = "" } },
  }) do
    local rejected = pcall(config.resolve, opts)
    assert(not rejected, "accepted " .. vim.inspect(opts))
  end
end

return T
