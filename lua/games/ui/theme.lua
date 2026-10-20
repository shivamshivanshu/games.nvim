local highlights = require("games.ui.highlights")
local notify = require("games.core.notify")

---@class games.ThemeOptions
---@field palette games.Palette
---@field highlights games.HlDefinitions always applied over the palette, cut to each group's role

local SEAT_COLOURS = 4
---@type table<games.Outcome, string>
local OUTCOME_HL =
  { won = "GamesWon", lost = "GamesLost", drawn = "GamesDrawn", aborted = "GamesMuted" }
local AUGROUP = "games.theme"

---@type games.ThemeOptions
local DEFAULT_OPTIONS = { palette = "theme", highlights = {} }

local M = {}

---@type games.Palette
local active_palette = DEFAULT_OPTIONS.palette
---@type table<string, vim.api.keyset.get_hl_info> group -> its definition as this module last left it
local applied = {}
local warned_no_truecolor = false
---@type table<string, true> groups whose override has been reported as cut
local warned_overrides = {}

---@param group string
---@return vim.api.keyset.get_hl_info
local function resolved(group)
  return vim.api.nvim_get_hl(0, { name = group, link = false })
end

---The group's own definition with links unresolved; the `default` flag is dropped because
---`:colorscheme` restores a default link without it, which would otherwise look foreign.
---@param group string
---@return vim.api.keyset.get_hl_info
local function own_definition(group)
  local definition = vim.api.nvim_get_hl(0, { name = group, create = false })
  definition.default = nil
  return definition
end

---Whether the group is blank or still exactly as this module left it. Anything else was set
---by someone else, a `:hi` or a colourscheme, and is theirs to keep.
---@param group string
---@return boolean
local function is_ours(group)
  local definition = own_definition(group)
  return vim.tbl_isempty(definition) or vim.deep_equal(definition, applied[group])
end

---@param group string
---@param attrs vim.api.keyset.highlight
local function define(group, attrs)
  vim.api.nvim_set_hl(0, group, vim.tbl_extend("force", attrs, { default = true, force = true }))
  applied[group] = own_definition(group)
end

---@param definitions games.HlDefinitions
---@param overrides games.HlDefinitions groups to leave to the overrides
local function define_defaults(definitions, overrides)
  for group, attrs in pairs(definitions) do
    if not overrides[group] and is_ours(group) then
      define(group, attrs)
    end
  end
end

---@param group string
---@param dropped string[]
local function warn_cut_override(group, dropped)
  if #dropped == 0 or warned_overrides[group] then
    return
  end
  warned_overrides[group] = true
  notify.say(
    ("games: highlights.%s: %s ignored; this group sets only its %s"):format(
      group,
      table.concat(dropped, ", "),
      highlights.ROLES[group] == highlights.Role.BACKGROUND and "background" or "foreground"
    ),
    vim.log.levels.WARN
  )
end

---@param overrides games.HlDefinitions
local function define_overrides(overrides)
  for group, definition in pairs(overrides) do
    local kept, dropped = highlights.override_for(group, definition, resolved)
    warn_cut_override(group, dropped)
    define(group, kept)
  end
end

---The palette that can be shown: vivid needs 'termguicolors' and falls back to theme without it.
---@param requested games.Palette
---@return games.Palette
local function showable_palette(requested)
  if requested ~= "vivid" or vim.o.termguicolors then
    return requested
  end
  if not warned_no_truecolor then
    warned_no_truecolor = true
    notify.say('games: palette "vivid" needs \'termguicolors\'; using "theme"', vim.log.levels.WARN)
  end
  return "theme"
end

---@param palette games.Palette
---@return games.HlDefinitions
local function palette_definitions(palette)
  if palette == "vivid" then
    return highlights.vivid_definitions()
  end
  return highlights.theme_definitions(resolved)
end

---Inks go last: they are chosen against the backgrounds the palette and overrides left.
---@param options games.ThemeOptions
local function apply_all(options)
  active_palette = showable_palette(options.palette)
  define_defaults(palette_definitions(active_palette), options.highlights)
  define_overrides(options.highlights)
  define_defaults(highlights.ink_definitions(resolved), options.highlights)
end

---Defines every Games* group from the palette, then options.highlights, and again after each
---`:colorscheme`. A palette default never replaces a group someone else has set since.
---@param options? games.ThemeOptions
function M.setup(options)
  options = options or DEFAULT_OPTIONS
  apply_all(options)
  vim.api.nvim_create_autocmd("ColorScheme", {
    group = vim.api.nvim_create_augroup(AUGROUP, { clear = true }),
    callback = function()
      apply_all(options)
    end,
  })
end

---The palette the groups are defined from, after any fallback.
---@return games.Palette
function M.palette()
  return active_palette
end

---Highlight group for the seat at position i of update.players; colours repeat after four seats.
---@param i integer
---@return string
function M.seat_hl(i)
  return "GamesSeat" .. ((i - 1) % SEAT_COLOURS + 1)
end

---@param outcome games.Outcome
---@return string
function M.outcome_hl(outcome)
  return OUTCOME_HL[outcome]
end

return M
