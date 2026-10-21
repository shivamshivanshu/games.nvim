local actions = require("games.ui.actions")
local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local notation = require("games.ui.notation")
local players = require("games.core.players")
local tables = require("games.core.tables")
local theme = require("games.ui.theme")

local Group = highlights.Group

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
local SAMPLE_SEEDS = 4
---Columns: the top seat's store, six pits, then the bottom seat's store.
local COLUMNS = PITS + 2
local CELL_WIDTH = 4
local BLANK_KEY = "blank"
local SOW_KIND = "sow"
local LEGAL_HL = Group.MOVABLE
local TOP_ROW, BOTTOM_ROW = "top row, store on the left", "bottom row, store on the right"

---@enum games.MancalaKeyKind
local KeyKind = { PIT = "pit", LEGAL = "legal", STORE = "store" }

---Under the bottom row, the pits of the seat that sits there; the stores are unlabelled.
---@type string[]
local COLUMN_LABELS = vim.list_extend(vim.list_extend({ "" }, notation.numbers(PITS)), { "" })
local ROW_LABELS = { "", "" }

---@param kind games.MancalaKeyKind
---@param seeds integer
---@return string the asset key, e.g. "legal:4"
local function key_for(kind, seeds)
  return art.marked_key(seeds, kind)
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

---Counter-clockwise from the bottom left: the bottom seat's pits left to right into its store on
---the right, then the top seat's right to left into its store on the left.
---@param update games.MancalaUpdate
---@param me games.UserID
---@return fun(r: integer, c: integer): games.GridCell
local function cells(update, me)
  local view = update.view
  local bottom, top = sides(update, me)
  local legal = tables.set_of(view.legal_pits)
  return function(r, c)
    if (r == 1 and c == COLUMNS) or (r == 2 and c == 1) then
      return { BLANK_KEY }
    end
    local seat = r == 1 and top or bottom
    local pit = r == 1 and COLUMNS - c or c - 1
    local side = view[seat]
    if pit > PITS or pit < 1 then
      return { key_for(KeyKind.STORE, side.store), theme.seat_hl_of(update, seat) }
    end
    local kind = (seat == bottom and legal[pit]) and KeyKind.LEGAL or KeyKind.PIT
    return { key_for(kind, side.pits[pit]), theme.seat_hl_of(update, seat) }
  end
end

---@param view games.MancalaView
---@return games.GridMark[] the viewer's legal pits, on the bottom row
local function marks_for(view)
  return vim.tbl_map(function(pit)
    return grid.mark(2, pit + 1, LEGAL_HL)
  end, view.legal_pits)
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

---@param update games.MancalaUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local bottom, top = sides(update, ctx.me)
  return board({
    grid = {
      rows = 2,
      cols = COLUMNS,
      cell_width = CELL_WIDTH,
      labels = { cols = COLUMN_LABELS, rows = ROW_LABELS },
      cell = cells(update, ctx.me),
      marks = marks_for(update.view),
    },
    panels = {
      {
        node = function()
          return kv.panel({
            { players.name_of_seat(update, top), TOP_ROW },
            { players.name_of_seat(update, bottom), BOTTOM_ROW },
          })
        end,
      },
    },
  }, ctx)
end

---@type games.GameAction[]
M.actions = actions.per_value({
  kind = SOW_KIND,
  field = "pit",
  values = vim.fn.range(1, PITS),
  legal = function(view)
    return view.legal_pits
  end,
})

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

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local function pit(kind)
    return { key_for(kind, SAMPLE_SEEDS), theme.seat_hl(1) }
  end
  return {
    {
      sample = { grid.sample(ctx, pit(KeyKind.PIT)) },
      meaning = "a pit and its seeds, in its owner's colour",
    },
    { sample = { grid.sample(ctx, pit(KeyKind.LEGAL), LEGAL_HL) }, meaning = "a pit you may sow" },
    {
      sample = { grid.sample(ctx, pit(KeyKind.STORE)) },
      meaning = "a store: the seeds its owner has won",
    },
  }
end

return M
