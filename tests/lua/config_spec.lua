local config = require("games.config")

---@param accepted table[] setup options that each resolve
local function assert_resolves(accepted)
  for _, opts in ipairs(accepted) do
    local ok, err = pcall(config.resolve, opts)
    assert(ok, err)
  end
end

---@param rejected table[] setup options that each fail to resolve
local function assert_rejects(rejected)
  for _, opts in ipairs(rejected) do
    assert(not pcall(config.resolve, opts), "accepted " .. vim.inspect(opts))
  end
end

local T = {}

function T.credentials_are_optional_but_type_checked()
  assert_resolves({ {}, { username = "alice" }, { password = "pw" } })
  assert_rejects({ { username = 42 }, { password = true } })
end

function T.palette_and_highlights_are_validated()
  assert_resolves({ { palette = "vivid", highlights = { GamesKey = { fg = "#123456" } } } })
  assert_rejects({
    { palette = "neon" },
    { highlights = "GamesKey" },
    { highlights = { GamesKye = { fg = "#123456" } } },
    { highlights = { GamesKey = "#123456" } },
  })
end

function T.archive_dir_is_optional_and_expanded()
  assert(config.resolve({}).archive.dir == nil)
  local resolved = config.resolve({ archive = { dir = "~/games-archive" } }).archive.dir
  assert(resolved == vim.fs.joinpath(vim.env.HOME, "games-archive"), resolved)
  assert_rejects({
    { archive = { dir = "" } },
    { archive = { dir = 42 } },
    { archive = "~/games" },
  })
end

function T.board_and_assets_are_validated()
  assert_resolves({ { board = { max_cell_height = 1 }, assets = { chess = "my.chess_art" } } })
  assert_rejects({
    { board = { max_cell_height = 0 } },
    { board = { max_cell_height = 2.5 } },
    { assets = { chess = 1 } },
    { assets = "default" },
  })
end

function T.picker_and_keys_are_validated()
  assert_resolves({
    { picker = "fzf" },
    { picker = "select" },
    { keys = { dashboard = "<leader>gg", find = "<leader>gf" } },
  })
  assert_rejects({
    { picker = "mini" },
    { keys = "<leader>gg" },
    { keys = { finder = "<leader>gf" } },
    { keys = { dashboard = "" } },
    { keys = { dashboard = "<leader>g", find = "<Leader>g" } },
  })
end

function T.puzzles_max_entries_is_validated()
  assert(config.resolve({ puzzles = { max_entries = 10 } }).puzzles.max_entries == 10)
  assert_rejects({
    { puzzles = { max_entries = 0 } },
    { puzzles = { max_entries = 2.5 } },
    { puzzles = 5000 },
  })
end

return T
