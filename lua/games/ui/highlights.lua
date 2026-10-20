local colour = require("games.ui.colour")

---@alias games.Palette "theme"|"vivid"

---What a group may set: a background layer sets no foreground, so it stacks under one; a
---foreground layer sets no background, so it stacks over one; a badge stands alone.
---@enum games.HlRole
local Role = { BACKGROUND = "bg", FOREGROUND = "fg", BADGE = "badge" }

---@alias games.HlAttr "fg"|"bg"

---@class games.HlSource
---@field group string colourscheme group to read
---@field attr? games.HlAttr attribute to read, by default the one the role sets

---@class games.HlGroupSpec
---@field name string
---@field role games.HlRole
---@field theme games.HlSource[] tried in order under the theme palette; none leaves the layer blank
---@field theme_style? vim.api.keyset.highlight attributes added under the theme palette
---@field vivid vim.api.keyset.highlight the whole definition under the vivid palette
---@field layered_over? string group this one is always stacked over, whose background shows where it sets none

---@alias games.HlDefinitions table<string, vim.api.keyset.highlight> group -> nvim_set_hl attributes

---@type table<games.HlRole, games.HlAttr>
local ROLE_ATTR = { [Role.BACKGROUND] = "bg", [Role.FOREGROUND] = "fg", [Role.BADGE] = "fg" }
---@type table<games.HlRole, table<string, true>|nil> attributes the role keeps; nil keeps all
local ROLE_KEEPS = {
  [Role.BACKGROUND] = { bg = true, ctermbg = true },
  [Role.FOREGROUND] = {
    fg = true,
    ctermfg = true,
    bold = true,
    italic = true,
    underline = true,
  },
}
---@type table<string, true> keys that say how a definition is set, not what it shows
local SETTING_KEYS = { default = true, force = true }
---@type table<string, true> top-level keys that are not attributes; cterm is partitioned on its own
local TOP_LEVEL_SKIPS = vim.tbl_extend("error", SETTING_KEYS, { cterm = true })
local OPPOSITE_ATTR = { fg = "bg", bg = "fg" }
local INK_SUFFIX = "Ink"
local LIGHT_SQUARE, DARK_SQUARE = "GamesSquareLight", "GamesSquareDark"

---@class (private) games.Ink
---@field gui integer
---@field cterm integer

---@type games.Ink
local DARK_INK = { gui = 0x1b1b1b, cterm = 234 }
---@type games.Ink
local LIGHT_INK = { gui = 0xf2f2f2, cterm = 255 }

---@param group string
---@param attr? games.HlAttr
---@return games.HlSource
local function from(group, attr)
  return { group = group, attr = attr }
end

---Seats read Diagnostic* because every colourscheme keeps their conventional hues. Seats 1 and 2
---are blue and yellow/orange: that axis survives the common red-green colour blindness.
---@type games.HlGroupSpec[]
local GROUPS = {
  {
    name = "GamesSquareLight",
    role = Role.BACKGROUND,
    theme = { from("CursorLine"), from("Pmenu") },
    vivid = { bg = "#f0d9b5" },
  },
  {
    name = "GamesSquareDark",
    role = Role.BACKGROUND,
    theme = { from("ColorColumn"), from("StatusLine") },
    vivid = { bg = "#b58863" },
  },
  {
    name = "GamesLastMove",
    role = Role.BACKGROUND,
    theme = { from("Search"), from("CurSearch"), from("Visual") },
    vivid = { bg = "#cdd26a" },
  },
  {
    name = "GamesLastMoveDark",
    role = Role.BACKGROUND,
    theme = {},
    vivid = { bg = "#aaa23a" },
    layered_over = "GamesLastMove",
  },
  {
    name = "GamesCheck",
    role = Role.BACKGROUND,
    theme = { from("DiagnosticError", "fg"), from("ErrorMsg", "fg"), from("DiffDelete") },
    vivid = { bg = "#e05050" },
  },
  {
    name = "GamesWin",
    role = Role.BACKGROUND,
    theme = { from("IncSearch"), from("CurSearch"), from("Search") },
    vivid = { bg = "#629924" },
  },
  {
    name = "GamesHit",
    role = Role.BACKGROUND,
    theme = { from("DiffAdd"), from("DiagnosticOk", "fg") },
    vivid = { bg = "#6aaa64" },
  },
  {
    name = "GamesPresent",
    role = Role.BACKGROUND,
    theme = { from("DiffText"), from("DiagnosticWarn", "fg") },
    vivid = { bg = "#c9b458" },
  },
  {
    name = "GamesMiss",
    role = Role.BACKGROUND,
    theme = { from("DiffChange"), from("Visual") },
    vivid = { bg = "#787c7e" },
  },
  {
    name = "GamesMovable",
    role = Role.BACKGROUND,
    theme = { from("PmenuSel"), from("Visual") },
    vivid = { bg = "#829769" },
  },
  {
    name = "GamesPieceWhite",
    role = Role.FOREGROUND,
    theme = { from("Normal") },
    vivid = { fg = "#1b1b1b" },
  },
  {
    name = "GamesPieceBlack",
    role = Role.FOREGROUND,
    theme = { from("Identifier"), from("Special") },
    vivid = { fg = "#1b1b1b" },
  },
  {
    name = "GamesSeat1",
    role = Role.FOREGROUND,
    theme = { from("DiagnosticInfo") },
    vivid = { fg = "#56b4e9" },
  },
  {
    name = "GamesSeat2",
    role = Role.FOREGROUND,
    theme = { from("DiagnosticWarn") },
    vivid = { fg = "#e69f00" },
  },
  {
    name = "GamesSeat3",
    role = Role.FOREGROUND,
    theme = { from("DiagnosticError") },
    vivid = { fg = "#d55e00" },
  },
  {
    name = "GamesSeat4",
    role = Role.FOREGROUND,
    theme = { from("DiagnosticOk") },
    vivid = { fg = "#009e73" },
  },
  {
    name = "GamesKey",
    role = Role.FOREGROUND,
    theme = { from("Special"), from("Statement") },
    theme_style = { bold = true },
    vivid = { fg = "#e69f00", bold = true },
  },
  {
    name = "GamesLabel",
    role = Role.FOREGROUND,
    theme = { from("Comment") },
    vivid = { fg = "#a0a0a0" },
  },
  {
    name = "GamesMuted",
    role = Role.FOREGROUND,
    theme = { from("Comment") },
    vivid = { fg = "#808080" },
  },
  {
    name = "GamesWon",
    role = Role.FOREGROUND,
    theme = { from("DiagnosticOk") },
    theme_style = { bold = true },
    vivid = { fg = "#629924", bold = true },
  },
  {
    name = "GamesLost",
    role = Role.FOREGROUND,
    theme = { from("DiagnosticError") },
    theme_style = { bold = true },
    vivid = { fg = "#e05050", bold = true },
  },
  {
    name = "GamesDrawn",
    role = Role.FOREGROUND,
    theme = { from("DiagnosticWarn") },
    theme_style = { bold = true },
    vivid = { fg = "#c9b458", bold = true },
  },
  {
    name = "GamesClockLow",
    role = Role.FOREGROUND,
    theme = { from("DiagnosticError") },
    theme_style = { bold = true },
    vivid = { fg = "#e05050", bold = true },
  },
  {
    name = "GamesCheckSign",
    role = Role.FOREGROUND,
    theme = { from("DiagnosticError") },
    vivid = { fg = "#e05050" },
  },
  {
    name = "GamesDashSection",
    role = Role.FOREGROUND,
    theme = { from("Title"), from("Statement") },
    theme_style = { bold = true },
    vivid = { fg = "#56b4e9", bold = true },
  },
  {
    name = "GamesYourTurn",
    role = Role.BADGE,
    theme = { from("DiagnosticOk") },
    theme_style = { reverse = true, bold = true },
    vivid = { fg = "#ffffff", bg = "#629924", bold = true },
  },
}

---@type games.HlGroupSpec[] background groups, each with an ink group for the text over it
local ACCENTS = vim.tbl_filter(function(spec)
  return spec.role == Role.BACKGROUND
end, GROUPS)

---@type table<string, string> background group -> its ink group
local INK_OF = vim.iter(ACCENTS):fold({}, function(inks, spec)
  inks[spec.name] = spec.name .. INK_SUFFIX
  return inks
end)

local M = {}

M.Role = Role
M.GROUPS = GROUPS

---@type table<string, games.HlRole> every group, inks included
M.ROLES = vim.iter(INK_OF):fold(
  vim.iter(GROUPS):fold({}, function(roles, spec)
    roles[spec.name] = spec.role
    return roles
  end),
  function(roles, _, ink)
    roles[ink] = Role.FOREGROUND
    return roles
  end
)

---@param attrs table<string, any>
---@param keeps table<string, true>
---@param skips table<string, true> neither kept nor reported
---@param prefix string names a dropped attribute
---@return table<string, any> kept
---@return string[] dropped
local function partition(attrs, keeps, skips, prefix)
  local kept, dropped = {}, {}
  for attr, value in pairs(attrs) do
    if keeps[attr] then
      kept[attr] = value
    elseif not skips[attr] then
      dropped[#dropped + 1] = prefix .. attr
    end
  end
  return kept, dropped
end

---The part of a definition its role may set, and the names of the attributes it may not.
---@param role games.HlRole
---@param definition vim.api.keyset.highlight
---@return vim.api.keyset.highlight kept
---@return string[] dropped sorted
function M.for_role(role, definition)
  local keeps = ROLE_KEEPS[role]
  if not keeps then
    return definition, {}
  end
  local kept, dropped = partition(definition, keeps, TOP_LEVEL_SKIPS, "")
  local cterm_kept, cterm_dropped = partition(definition.cterm or {}, keeps, SETTING_KEYS, "cterm.")
  kept.cterm = not vim.tbl_isempty(cterm_kept) and cterm_kept or nil
  local all_dropped = vim.list_extend(dropped, cterm_dropped)
  table.sort(all_dropped)
  return kept, all_dropped
end

---A user's definition of group, a link resolved to its target's attributes, cut to the group's
---role.
---@param group string
---@param definition vim.api.keyset.highlight
---@param read fun(group: string): vim.api.keyset.get_hl_info the resolved definition of group
---@return vim.api.keyset.highlight kept
---@return string[] dropped sorted
function M.override_for(group, definition, read)
  local link = definition.link --[[@as string|nil]]
  local source = link and read(link) --[[@as vim.api.keyset.highlight]]
  return M.for_role(M.ROLES[group], source or definition)
end

---hl with the ink of its topmost background layer added, for text that sets no foreground of
---its own.
---@param hl games.Hl|nil
---@return games.Hl|nil
function M.with_ink(hl)
  if not hl then
    return nil
  end
  local layers = type(hl) == "table" and hl or { hl }
  local ink = INK_OF[layers[#layers]]
  if not ink then
    return hl
  end
  return vim.list_extend(vim.list_extend({}, layers), { ink })
end

---The ink that contrasts more with a background: dark on a light one, light on a dark one.
---@param background_rgb integer|nil
---@param ink_rgb fun(ink: games.Ink): integer
---@return games.Ink|nil
local function contrasting_ink(background_rgb, ink_rgb)
  if not background_rgb then
    return nil
  end
  local function contrast(ink)
    return colour.contrast(background_rgb, ink_rgb(ink))
  end
  return contrast(DARK_INK) >= contrast(LIGHT_INK) and DARK_INK or LIGHT_INK
end

---@param ink games.Ink
---@return integer
local function gui_rgb(ink)
  return ink.gui
end

---@param ink games.Ink
---@return integer
local function cterm_rgb(ink)
  return colour.xterm_rgb(ink.cterm)
end

---Ink groups chosen against the backgrounds the accents show now, so they follow any override.
---@param read fun(group: string): vim.api.keyset.get_hl_info the resolved definition of group
---@return games.HlDefinitions
function M.ink_definitions(read)
  return vim.iter(ACCENTS):fold({}, function(definitions, spec)
    local own = read(spec.name)
    local under = spec.layered_over and read(spec.layered_over) or {}
    local cterm_bg = own.ctermbg or under.ctermbg
    local gui_ink = contrasting_ink(own.bg or under.bg, gui_rgb)
    local cterm_ink = contrasting_ink(cterm_bg and colour.xterm_rgb(cterm_bg), cterm_rgb)
    definitions[INK_OF[spec.name]] = {
      fg = gui_ink and gui_ink.gui,
      ctermfg = cterm_ink and cterm_ink.cterm,
    }
    return definitions
  end)
end

---The colour a source group shows for attr, as target: a reversed group shows its foreground
---as background and vice versa.
---@param info vim.api.keyset.get_hl_info
---@param attr games.HlAttr
---@param target games.HlAttr
---@return vim.api.keyset.highlight
local function shown_as(info, attr, target)
  local stored = info.reverse and OPPOSITE_ATTR[attr] or attr
  return { [target] = info[stored], ["cterm" .. target] = info["cterm" .. stored] }
end

---@param read fun(group: string): vim.api.keyset.get_hl_info
---@param sources games.HlSource[]
---@param role games.HlRole
---@return vim.api.keyset.highlight
local function first_colour(read, sources, role)
  local target = ROLE_ATTR[role]
  for _, source in ipairs(sources) do
    local found = M.for_role(role, shown_as(read(source.group), source.attr or target, target))
    if not vim.tbl_isempty(found) then
      return found
    end
  end
  return {}
end

---Luminance of a definition's background in gui terms, and in cterm terms.
---@param definition vim.api.keyset.highlight with integer colours, as derived ones have
---@return number|nil gui
---@return number|nil cterm
local function background_luminance(definition)
  local gui, cterm = tonumber(definition.bg), tonumber(definition.ctermbg)
  return gui and colour.luminance(gui), cterm and colour.luminance(colour.xterm_rgb(cterm))
end

---Whether background a is lighter than b, judged in gui colours when both have one, else in
---cterm colours; nil when the two cannot be compared.
---@param a vim.api.keyset.highlight
---@param b vim.api.keyset.highlight
---@return boolean|nil
local function is_lighter(a, b)
  local a_gui, a_cterm = background_luminance(a)
  local b_gui, b_cterm = background_luminance(b)
  if a_gui and b_gui then
    return a_gui > b_gui
  end
  if a_cterm and b_cterm then
    return a_cterm > b_cterm
  end
end

---definitions with the two square shades swapped if the light one came out darker.
---@param definitions games.HlDefinitions
---@return games.HlDefinitions
local function squares_by_lightness(definitions)
  local light, dark = definitions[LIGHT_SQUARE], definitions[DARK_SQUARE]
  if not is_lighter(dark, light) then
    return definitions
  end
  return vim.tbl_extend("force", definitions, { [LIGHT_SQUARE] = dark, [DARK_SQUARE] = light })
end

---Definitions derived from the colourscheme, taking from each source only the attribute the
---group's role sets.
---@param read fun(group: string): vim.api.keyset.get_hl_info the resolved definition of group
---@return games.HlDefinitions
function M.theme_definitions(read)
  local definitions = vim.iter(GROUPS):fold({}, function(derived, spec)
    local found = first_colour(read, spec.theme, spec.role)
    derived[spec.name] = vim.tbl_extend("force", spec.theme_style or {}, found)
    return derived
  end)
  return squares_by_lightness(definitions)
end

---@return games.HlDefinitions
function M.vivid_definitions()
  return vim.iter(GROUPS):fold({}, function(definitions, spec)
    definitions[spec.name] = spec.vivid
    return definitions
  end)
end

return M
