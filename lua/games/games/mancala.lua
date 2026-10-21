local board = require("games.ui.widgets.board")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")
local theme = require("games.ui.theme")

---@alias games.MancalaSeat "south"|"north"

---One seat's row, in its own numbering: pits[1] is its pit 1, on its left, and pits[6] its pit 6,
---next to its store.
---@class games.MancalaSide
---@field pits integer[]
---@field store integer

---@class games.MancalaView
---@field south games.MancalaSide
---@field north games.MancalaSide
---@field legal_pits integer[] the viewer's own: the pits it may sow, empty unless it is to move

---@class games.MancalaUpdate: games.GameUpdate
---@field view games.MancalaView

---@class games.MancalaSowedEvent: games.Event
---@field kind "sowed"
---@field by games.MancalaSeat
---@field pit integer
---@field captured integer seeds the sowing took into by's store
---@field again boolean whether by moves again, its last seed in its store

---@class games.MancalaCollectedEvent: games.Event
---@field kind "collected"
---@field by games.MancalaSeat
---@field seeds integer left in by's pits at the end, moved to its store

local PITS = 6
local MAX_SEEDS = 48
---Columns: the top seat's store, six pits, then the bottom seat's store.
local COLUMNS = PITS + 2
local CELL_WIDTH = 4
local BLANK_KEY = "blank"
local SOW_KIND = "sow"
local LEGAL_HL = "GamesMovable"
local KEY_HL, VALUE_HL = "GamesKey", "GamesLabel"
local TOP_ROW, BOTTOM_ROW = "top row, store on the left", "bottom row, store on the right"

---@enum games.MancalaKeyKind
local KeyKind = { PIT = "pit", LEGAL = "legal", STORE = "store" }

---Under the bottom row, the pits of the seat that sits there; the stores are unlabelled.
---@type string[]
local COLUMN_LABELS =
  vim.list_extend(vim.list_extend({ "" }, vim.tbl_map(tostring, vim.fn.range(1, PITS))), { "" })
local ROW_LABELS = { "", "" }

---@param kind games.MancalaKeyKind
---@param seeds integer
---@return string the asset key, e.g. "legal:4"
local function key_for(kind, seeds)
  return ("%s:%d"):format(kind, seeds)
end

---The seat drawn at the bottom: the viewer's own, or south for anyone else.
---@param update games.MancalaUpdate
---@param me games.UserID
---@return games.MancalaSeat bottom
---@return games.MancalaSeat top
local function sides(update, me)
  if players.seat_of_user(update, me) == "north" then
    return "north", "south"
  end
  return "south", "north"
end

---@param update games.MancalaUpdate
---@param seat games.MancalaSeat
---@return games.Hl
local function seat_hl(update, seat)
  return theme.seat_hl(assert(players.seat_index(update, seat)))
end

---Counter-clockwise from the bottom left: the bottom seat's pits left to right into its store on
---the right, then the top seat's right to left into its store on the left.
---@param update games.MancalaUpdate
---@param me games.UserID
---@return fun(r: integer, c: integer): games.GridCell
local function cells(update, me)
  local view = update.view
  local bottom, top = sides(update, me)
  local legal = {}
  for _, pit in ipairs(view.legal_pits) do
    legal[pit] = true
  end
  return function(r, c)
    if (r == 1 and c == COLUMNS) or (r == 2 and c == 1) then
      return { BLANK_KEY }
    end
    local seat = r == 1 and top or bottom
    local pit = r == 1 and COLUMNS - c or c - 1
    local side = view[seat]
    if pit > PITS or pit < 1 then
      return { key_for(KeyKind.STORE, side.store), seat_hl(update, seat) }
    end
    local kind = (seat == bottom and legal[pit]) and KeyKind.LEGAL or KeyKind.PIT
    return { key_for(kind, side.pits[pit]), seat_hl(update, seat) }
  end
end

---@param view games.MancalaView
---@return games.GridMark[] the viewer's legal pits, on the bottom row
local function marks_for(view)
  return vim.tbl_map(function(pit)
    return { row = 2, col = pit + 1, hl = LEGAL_HL }
  end, view.legal_pits)
end

---@param pit integer
---@return games.GameAction
local function sow_action(pit)
  return {
    key = tostring(pit),
    label = SOW_KIND,
    action = { kind = SOW_KIND, pit = tostring(pit) },
    available = function(update)
      return vim.tbl_contains(update.view.legal_pits, pit)
    end,
  }
end

local M = {}

---@type string[] keys every mancala asset pack draws: each seed count as a pit, a legal pit and a
---store, and the blank corners
M.asset_keys = vim.list_extend(
  vim
    .iter(vim.fn.range(0, MAX_SEEDS))
    :map(function(seeds)
      return vim.tbl_map(function(kind)
        return key_for(kind, seeds)
      end, { KeyKind.PIT, KeyKind.LEGAL, KeyKind.STORE })
    end)
    :flatten()
    :totable(),
  { BLANK_KEY }
)

M.glyph_width = CELL_WIDTH
M.MAX_SEEDS = MAX_SEEDS
M.KeyKind = KeyKind
M.BLANK_KEY = BLANK_KEY
M.key_for = key_for

---@param update games.MancalaUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local bottom, top = sides(update, ctx.me)
  return board({
    grid = {
      rows = 2,
      cols = COLUMNS,
      shade = "plain",
      cell_width = CELL_WIDTH,
      labels = { cols = COLUMN_LABELS, rows = ROW_LABELS },
      cell = cells(update, ctx.me),
      marks = marks_for(update.view),
    },
    art = assert(ctx.art, "mancala draws from an asset pack"),
    panels = {
      {
        node = function()
          return kv({
            pairs = {
              { players.name_of_seat(update, top), TOP_ROW },
              { players.name_of_seat(update, bottom), BOTTOM_ROW },
            },
            key_hl = KEY_HL,
            value_hl = VALUE_HL,
          })
        end,
      },
    },
  }, ctx)
end

---@type games.GameAction[]
M.actions = vim.tbl_map(sow_action, vim.fn.range(1, PITS))

---@type table<games.EventKind, games.Describer>
M.describers = {
  sowed = function(event, talk)
    ---@cast event games.MancalaSowedEvent
    local captured = event.captured > 0 and (", capturing %d"):format(event.captured) or ""
    local again = event.again and ", and moves again" or ""
    return ("%s sowed pit %d%s%s"):format(talk.name(event.by), event.pit, captured, again)
  end,
  collected = function(event, talk)
    ---@cast event games.MancalaCollectedEvent
    return ("%s collects the %d seeds left on their side"):format(talk.name(event.by), event.seeds)
  end,
}

return M
