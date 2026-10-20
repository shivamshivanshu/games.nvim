local finder = require("games.ui.finder")
local highlights = require("games.ui.highlights")
local predicates = require("games.core.predicates")

local M = {}

---@alias games.Glyphs "unicode"|"ascii"

---@class games.ServerAddress
---@field host string
---@field port integer

---@class games.ArchiveConfig
---@field dir? string absolute; archiving is off when nil

---@class games.BoardConfig
---@field max_cell_height? integer lines per board cell at most; 1 draws glyphs only, nil no cap

---@class games.KeysConfig normal-mode global keys; each is mapped only when set
---@field dashboard? string
---@field find? string

---@class games.Config
---@field server games.ServerAddress
---@field username? string asked for on the first :Games when nil
---@field password? string|fun(): string resolved at each connect, never during setup; asked for when nil
---@field autoconnect boolean
---@field glyphs games.Glyphs
---@field palette games.Palette
---@field highlights games.HlDefinitions Games* group -> nvim_set_hl attributes, over the palette
---@field archive games.ArchiveConfig
---@field board games.BoardConfig
---@field assets table<games.GameType, string> asset pack per game: a shipped name or a module name
---@field picker? games.PickerName the finder's picker; nil takes the first installed
---@field keys games.KeysConfig

---@class games.SetupOpts
---@field server? {host?: string, port?: integer}
---@field username? string
---@field password? string|fun(): string
---@field autoconnect? boolean
---@field glyphs? games.Glyphs
---@field palette? games.Palette
---@field highlights? games.HlDefinitions
---@field archive? {dir?: string}
---@field board? games.BoardConfig
---@field assets? table<games.GameType, string>
---@field picker? games.PickerName
---@field keys? games.KeysConfig

local GLYPHS = { unicode = true, ascii = true }
local PALETTES = { theme = true, vivid = true }
local KEY_NAMES = { dashboard = true, find = true }

---@type games.Glyphs
M.DEFAULT_GLYPHS = "unicode"

local DEFAULTS = {
  server = { host = "127.0.0.1", port = 7777 },
  autoconnect = true,
  glyphs = M.DEFAULT_GLYPHS,
  palette = "theme",
  highlights = {},
  archive = {},
  board = {},
  assets = {},
  keys = {},
}

---@param port any
---@return boolean
local function is_port(port)
  return type(port) == "number" and port == math.floor(port) and port > 0 and port < 65536
end

---@param definitions any
---@return boolean
---@return string|nil reason
local function valid_highlights(definitions)
  if type(definitions) ~= "table" then
    return false, "not a table"
  end
  for group, attrs in pairs(definitions) do
    if not highlights.ROLES[group] then
      return false, ("unknown group %s"):format(vim.inspect(group))
    end
    if type(attrs) ~= "table" then
      return false, ("%s: attributes must be a table"):format(group)
    end
  end
  return true
end

---@param value any
---@return boolean
local function is_nonempty_string(value)
  return type(value) == "string" and value ~= ""
end

---@param board any
---@return boolean
local function valid_board(board)
  local height = type(board) == "table" and board.max_cell_height
  return height == nil or predicates.is_positive_integer(height)
end

---@param assets any
---@return boolean
local function valid_assets(assets)
  return type(assets) == "table"
    and vim.iter(assets):all(function(game, pack)
      return type(game) == "string" and type(pack) == "string"
    end)
end

---@param keys any
---@return boolean
---@return string|nil reason
local function valid_keys(keys)
  if type(keys) ~= "table" then
    return false, "not a table"
  end
  for name, lhs in pairs(keys) do
    if KEY_NAMES[name] == nil then
      return false, ("unknown key %s"):format(vim.inspect(name))
    end
    if not is_nonempty_string(lhs) then
      return false, ("%s: a key must be a non-empty string"):format(name)
    end
  end
  if keys.dashboard and keys.find and vim.keycode(keys.dashboard) == vim.keycode(keys.find) then
    return false, ("dashboard and find are both %s"):format(keys.find)
  end
  return true
end

---Merges opts over the defaults and validates the result.
---@param opts games.SetupOpts
---@return games.Config
function M.resolve(opts)
  vim.validate("opts", opts, "table")
  local config = vim.tbl_deep_extend("force", DEFAULTS, opts)
  vim.validate("server.host", config.server.host, "string")
  vim.validate("server.port", config.server.port, is_port, "port number 1-65535")
  vim.validate("username", config.username, "string", true)
  vim.validate("password", config.password, { "string", "function" }, true)
  vim.validate("autoconnect", config.autoconnect, "boolean")
  vim.validate("glyphs", config.glyphs, function(glyphs)
    return GLYPHS[glyphs] ~= nil
  end, '"unicode" or "ascii"')
  vim.validate("palette", config.palette, function(palette)
    return PALETTES[palette] ~= nil
  end, '"theme" or "vivid"')
  vim.validate("highlights", config.highlights, valid_highlights, "Games* group -> attributes")
  vim.validate("archive", config.archive, "table")
  vim.validate("archive.dir", config.archive.dir, is_nonempty_string, true, "non-empty path")
  local archive_dir = config.archive.dir
  config.archive = { dir = archive_dir and vim.fs.abspath(vim.fs.normalize(archive_dir)) }
  vim.validate("board", config.board, valid_board, "{ max_cell_height = positive integer }")
  vim.validate("assets", config.assets, valid_assets, "game -> asset pack name")
  vim.validate("picker", config.picker, function(picker)
    return vim.tbl_contains(finder.PICKER_NAMES, picker)
  end, true, '"snacks", "telescope", "fzf" or "select"')
  vim.validate("keys", config.keys, valid_keys, "{ dashboard?: string, find?: string }")
  return config
end

return M
