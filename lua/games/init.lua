local Archive = require("games.core.archive")
local Client = require("games.core.client")
local Conn = require("games.core.conn")
local config = require("games.config")
local notify = require("games.core.notify")
local theme = require("games.ui.theme")

local AUTOCONNECT_GROUP = "games.autoconnect"

local M = {}

M.REQUIRED_NVIM = "0.11"

---@return boolean
function M.neovim_supported()
  return vim.fn.has("nvim-" .. M.REQUIRED_NVIM) == 1
end

---@type games.Config|nil
M.config = nil

---@type games.Client|nil
M.client = nil

---@type games.Archive|nil nil while archive.dir is unset
M.archive = nil

---@class (private) games.GlobalKey
---@field lhs string
---@field run fun() the callback it was mapped to, which tells it from a user's remap

---@type games.GlobalKey[] the global keys the last setup() mapped
local mapped_keys = {}

---@param key games.GlobalKey
---@return boolean
local function still_ours(key)
  return vim.fn.maparg(key.lhs, "n", false, true).callback == key.run
end

---Maps each key keys sets, after unmapping the keys an earlier setup() mapped that nothing has
---remapped since.
---@param keys games.KeysConfig
local function map_global_keys(keys)
  for _, key in ipairs(vim.tbl_filter(still_ours, mapped_keys)) do
    vim.keymap.del("n", key.lhs)
  end
  mapped_keys = {}
  for name, fargs in pairs(config.KEY_COMMANDS) do
    local lhs = keys[name]
    if lhs then
      local function run()
        require("games.core.commands").run(fargs)
      end
      vim.keymap.set("n", lhs, run, { desc = "games: " .. name })
      mapped_keys[#mapped_keys + 1] = { lhs = lhs, run = run }
    end
  end
end

local function connect_current()
  if M.client then
    M.client.conn:connect()
  end
end

---Validates opts, replaces any previous connection, and connects after startup when autoconnect
---is set and both credentials are known.
---@param opts games.SetupOpts
function M.setup(opts)
  if not M.neovim_supported() then
    notify.error(("needs Neovim %s or newer"):format(M.REQUIRED_NVIM))
    return
  end
  M.config = config.resolve(opts)
  theme.setup({ palette = M.config.palette, highlights = M.config.highlights })
  map_global_keys(M.config.keys)
  if M.client then
    require("games.ui.dashboard").close()
    M.client.conn:close()
  end
  M.client = Client.new(Conn.new({
    host = M.config.server.host,
    port = M.config.server.port,
    username = M.config.username,
    password = M.config.password,
  }))
  notify.attach(M.client)
  local archive_dir = M.config.archive.dir
  M.archive = archive_dir and Archive.attach(M.client, archive_dir) or nil

  local group = vim.api.nvim_create_augroup(AUTOCONNECT_GROUP, { clear = true })
  if not (M.config.autoconnect and vim.tbl_isempty(M.client.conn:missing_credentials())) then
    return
  end
  if vim.v.vim_did_enter == 1 then
    vim.schedule(connect_current)
    return
  end
  vim.api.nvim_create_autocmd("VimEnter", {
    group = group,
    once = true,
    callback = function()
      vim.schedule(connect_current)
    end,
  })
end

---Options for every board this plugin opens, taken from the user's config.
---@return games.OpenOptions
function M.open_options()
  local resolved = assert(M.config, "games: setup() has not run")
  return {
    glyphs = resolved.glyphs,
    max_cell_height = resolved.board.max_cell_height,
    assets = resolved.assets,
  }
end

return M
