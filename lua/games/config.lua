local highlights = require("games.ui.highlights")

local M = {}

---@alias games.Glyphs "unicode"|"ascii"

---@class games.ServerAddress
---@field host string
---@field port integer

---@class games.Config
---@field server games.ServerAddress
---@field username? string asked for on the first :Games when nil
---@field password? string|fun(): string resolved at each connect, never during setup; asked for when nil
---@field autoconnect boolean
---@field glyphs games.Glyphs
---@field palette games.Palette
---@field highlights games.HlDefinitions Games* group -> nvim_set_hl attributes, over the palette

---@class games.SetupOpts
---@field server? {host?: string, port?: integer}
---@field username? string
---@field password? string|fun(): string
---@field autoconnect? boolean
---@field glyphs? games.Glyphs
---@field palette? games.Palette
---@field highlights? games.HlDefinitions

local GLYPHS = { unicode = true, ascii = true }
local PALETTES = { theme = true, vivid = true }

---@type games.Glyphs
M.DEFAULT_GLYPHS = "unicode"

local DEFAULTS = {
  server = { host = "127.0.0.1", port = 7777 },
  autoconnect = true,
  glyphs = M.DEFAULT_GLYPHS,
  palette = "theme",
  highlights = {},
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
  return config
end

return M
