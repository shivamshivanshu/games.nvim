local compose = require("games.ui.compose")
local float = require("games.ui.float")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local text_util = require("games.ui.text")

local Group = highlights.Group

local HEADING_HL = Group.KEY
local MEANING_HL = Group.LABEL
local INDENT = 2
local SAMPLE_GAP = 1

---@class games.HelpProps
---@field owner integer the buffer of the board it is the help of
---@field title string the game's name
---@field legend games.LegendEntry[]
---@field keys games.KvPair[] each bound key, its aliases beside it, and its label
---@field toggle_key string the key that opens the help, which closes it too

local M = {}

---@class (private) games.OpenHelp
---@field win integer
---@field owner integer

---@type games.OpenHelp|nil
local open_help

---@param entry games.LegendEntry
---@return integer
local function sample_width(entry)
  return compose.measure(compose.row(entry.sample)).w
end

---@param legend games.LegendEntry[]
---@return games.Node each sample, then its meaning, the meanings aligned
local function legend_rows(legend)
  local widest = vim.iter(legend):map(sample_width):fold(0, math.max)
  return compose.col(vim.tbl_map(function(entry)
    local row = vim.list_extend({}, entry.sample)
    row[#row + 1] = compose.text(text_util.spaces(widest - sample_width(entry) + SAMPLE_GAP))
    row[#row + 1] = compose.text(entry.meaning, MEANING_HL)
    return compose.row(row)
  end, legend))
end

---@param heading string
---@param body games.Node
---@return games.Node
local function section(heading, body)
  return compose.col({ compose.text(heading, HEADING_HL), compose.pad(INDENT, body) })
end

---The help's body: the legend, then the keys.
---@param legend games.LegendEntry[]
---@param keys games.KvPair[]
---@return games.Node
function M.node(legend, keys)
  return compose.col({
    section("Legend", legend_rows(legend)),
    compose.text(""),
    section("Keys", kv.panel(keys)),
  })
end

---Closes the help when it is open for props.owner; otherwise opens it, in place of any other float.
---@param props games.HelpProps
function M.toggle(props)
  local shown = open_help and vim.api.nvim_win_is_valid(open_help.win) and open_help
  if shown and shown.owner == props.owner then
    vim.api.nvim_win_close(shown.win, true)
    open_help = nil
    return
  end
  local win = float.open(("%s — help"):format(props.title), M.node(props.legend, props.keys))
  open_help = { win = win, owner = props.owner }
  vim.keymap.set("n", props.toggle_key, function()
    vim.api.nvim_win_close(win, true)
  end, { buffer = vim.api.nvim_win_get_buf(win), nowait = true, desc = "games: close" })
end

return M
