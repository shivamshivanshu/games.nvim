local board = require("games.ui.widgets.board")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")

---@alias games.MinesweeperSeat "solver"

---"0" to "8" is a revealed cell's count; mine, flagged_mine, exploded and wrong_flag appear once
---the game is over.
---@alias games.MinesweeperCell "hidden"|"flag"|"mine"|"flagged_mine"|"exploded"|"wrong_flag"|"0"|"1"|"2"|"3"|"4"|"5"|"6"|"7"|"8"

---@class games.MinesweeperSquare
---@field row integer 1 at the top
---@field col integer 1 at the left

---@class games.MinesweeperView
---@field cells games.MinesweeperCell[][] rows from the top
---@field flags integer
---@field remaining integer mines less flags; negative with more flags than mines
---@field chordable games.MinesweeperSquare[] revealed numbers a chord acts on now; empty once over

---@class games.MinesweeperUpdate: games.GameUpdate
---@field view games.MinesweeperView

---@class games.MinesweeperRevealedEvent: games.Event
---@field kind "revealed"
---@field by games.MinesweeperSeat
---@field row integer
---@field col integer
---@field opened integer cells the reveal opened

---@class games.MinesweeperFlaggedEvent: games.Event
---@field kind "flagged"
---@field by games.MinesweeperSeat
---@field row integer
---@field col integer
---@field flagged boolean false when the flag was taken off

---@class games.MinesweeperChordedEvent: games.Event
---@field kind "chorded"
---@field by games.MinesweeperSeat
---@field row integer
---@field col integer
---@field opened integer cells the chord opened

local CELL_WIDTH = 2
local ROW_LETTERS = "abcdefghijklmnop"
local MAX_COLUMNS = 30
local REVEAL_KIND, FLAG_KIND, CHORD_KIND = "reveal", "flag", "chord"
local EXPLODED_KEY = "exploded"
local HIDDEN_HL = "GamesMuted"
local FLAG_HL = "GamesCardRed"
local EXPLODED_HL = "GamesCheck"
local KEY_HL, VALUE_HL = "GamesKey", "GamesLabel"
local MINES_LEFT_LABEL = "mines left"
---Display columns the status panel reserves: its widest label, a space, and a count such as -99.
local PANEL_WIDTH = #MINES_LEFT_LABEL + 1 + 3

---@type table<games.MinesweeperCell, games.Hl>
local CELL_HL = { hidden = HIDDEN_HL, flag = FLAG_HL, flagged_mine = FLAG_HL, wrong_flag = FLAG_HL }

---@type string[]
local COLUMN_LABELS = vim.tbl_map(tostring, vim.fn.range(1, MAX_COLUMNS))

---@param row integer
---@param col integer
---@return string the cell as typed, e.g. "c12"
local function address(row, col)
  return ROW_LETTERS:sub(row, row) .. col
end

---@param view games.MinesweeperView
---@return games.GridMark[] the exploded mine, tinted on top of its own glyph
local function marks_for(view)
  local marks = {}
  for row, cells in ipairs(view.cells) do
    for col, cell in ipairs(cells) do
      if cell == EXPLODED_KEY then
        marks[#marks + 1] = { row = row, col = col, hl = EXPLODED_HL }
      end
    end
  end
  return marks
end

---@param kind games.ActionKind
---@param prompt string
---@return games.GameInput
local function cell_input(kind, prompt)
  return {
    kind = kind,
    prompt = prompt,
    parse = function(text)
      return { kind = kind, cell = text }
    end,
  }
end

---@param event games.MinesweeperRevealedEvent|games.MinesweeperChordedEvent
---@return string "" when it opened none, which only a mine does
local function opened_text(event)
  if event.opened == 0 then
    return ""
  end
  return event.opened == 1 and ", opening 1 cell" or (", opening %d cells"):format(event.opened)
end

local M = {}

---@type string[] keys every minesweeper asset pack draws: each cell the view can send
M.asset_keys = vim.list_extend(
  { "hidden", "flag", "mine", "flagged_mine", EXPLODED_KEY, "wrong_flag" },
  vim.tbl_map(tostring, vim.fn.range(0, 8))
)

M.glyph_width = CELL_WIDTH

---@param update games.MinesweeperUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  local rows, cols = #view.cells, #view.cells[1]
  return board({
    grid = {
      rows = rows,
      cols = cols,
      shade = "plain",
      cell_width = CELL_WIDTH,
      labels = { rows = ROW_LETTERS:sub(1, rows), cols = vim.list_slice(COLUMN_LABELS, 1, cols) },
      cell = function(r, c)
        local cell = view.cells[r][c]
        return { cell, CELL_HL[cell] }
      end,
      marks = marks_for(view),
    },
    art = assert(ctx.art, "minesweeper draws from an asset pack"),
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv({
            pairs = {
              { "flags", tostring(view.flags) },
              { MINES_LEFT_LABEL, tostring(view.remaining) },
            },
            key_hl = KEY_HL,
            value_hl = VALUE_HL,
          })
        end,
      },
    },
  }, ctx)
end

---@type games.GameInput
M.input = cell_input(REVEAL_KIND, "Reveal, e.g. c5: ")

---@type games.GameAction[]
M.actions = {
  { key = "f", label = "flag", input = cell_input(FLAG_KIND, "Flag or unflag, e.g. c5: ") },
  {
    key = "c",
    label = "chord",
    input = cell_input(CHORD_KIND, "Chord, e.g. c5: "),
    available = function(update)
      return #update.view.chordable > 0
    end,
  },
}

---@param event games.Event
---@param update games.MinesweeperUpdate
---@return string|nil
function M.describe_event(event, update)
  local who = players.user_of_seat(update, event.by) or event.by
  local at = event.row and address(event.row, event.col)
  if event.kind == "revealed" then
    ---@cast event games.MinesweeperRevealedEvent
    return ("%s revealed %s%s"):format(who, at, opened_text(event))
  elseif event.kind == "chorded" then
    ---@cast event games.MinesweeperChordedEvent
    return ("%s chorded %s%s"):format(who, at, opened_text(event))
  elseif event.kind == "flagged" then
    ---@cast event games.MinesweeperFlaggedEvent
    return ("%s %s %s"):format(who, event.flagged and "flagged" or "unflagged", at)
  end
  return nil
end

return M
