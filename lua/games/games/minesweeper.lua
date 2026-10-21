local board = require("games.ui.widgets.board")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local notation = require("games.ui.notation")
local text = require("games.ui.text")

local Group = highlights.Group

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
local REVEAL_KIND, FLAG_KIND, CHORD_KIND = "reveal", "flag", "chord"
local EXPLODED_KEY = "exploded"
local HIDDEN_HL = Group.MUTED
local FLAG_HL = Group.CARD_RED
local EXPLODED_HL = Group.CHECK
local MINES_LEFT_LABEL = "mines left"
---Display columns the status panel reserves: its widest label, a space, and a count such as -99.
local PANEL_WIDTH = #MINES_LEFT_LABEL + 1 + 3

---@type table<games.MinesweeperCell, games.Hl>
local CELL_HL = { hidden = HIDDEN_HL, flag = FLAG_HL, flagged_mine = FLAG_HL, wrong_flag = FLAG_HL }

---@param row integer
---@param col integer
---@return string the cell as typed, e.g. "c12"
local function address(row, col)
  return text.ALPHABET:sub(row, row) .. col
end

---@param view games.MinesweeperView
---@return games.GridMark[] the exploded mine, tinted on top of its own glyph
local function marks_for(view)
  return grid.marks_where(view.cells, function(cell)
    return cell == EXPLODED_KEY and EXPLODED_HL or nil
  end)
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
      cell_width = CELL_WIDTH,
      labels = { rows = notation.letters(rows), cols = notation.numbers(cols) },
      cell = function(r, c)
        local cell = view.cells[r][c]
        return { cell, CELL_HL[cell] }
      end,
      marks = marks_for(view),
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return kv.panel({
            { "flags", tostring(view.flags) },
            { MINES_LEFT_LABEL, tostring(view.remaining) },
          })
        end,
      },
    },
  }, ctx)
end

---@type games.GameInput
M.input = { kind = REVEAL_KIND, prompt = "Reveal, e.g. c5: " }

---@type games.GameAction[]
M.actions = {
  { key = "f", label = "flag", input = { kind = FLAG_KIND, prompt = "Flag or unflag, e.g. c5: " } },
  {
    key = "c",
    label = "chord",
    input = { kind = CHORD_KIND, prompt = "Chord, e.g. c5: " },
    available = function(update)
      return #update.view.chordable > 0
    end,
  },
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  revealed = function(event, talk)
    ---@cast event games.MinesweeperRevealedEvent
    return ("%s revealed %s%s"):format(
      talk.name(event.by),
      address(event.row, event.col),
      opened_text(event)
    )
  end,
  chorded = function(event, talk)
    ---@cast event games.MinesweeperChordedEvent
    return ("%s chorded %s%s"):format(
      talk.name(event.by),
      address(event.row, event.col),
      opened_text(event)
    )
  end,
  flagged = function(event, talk)
    ---@cast event games.MinesweeperFlaggedEvent
    return ("%s %s %s"):format(
      talk.name(event.by),
      event.flagged and "flagged" or "unflagged",
      address(event.row, event.col)
    )
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local function cell(key, meaning)
    return { sample = { grid.sample(ctx, { key, CELL_HL[key] }) }, meaning = meaning }
  end
  return {
    cell("hidden", "a hidden cell"),
    cell("flag", "a flag"),
    cell("3", "a revealed cell: the mines next to it"),
    cell("mine", "once the game is over: a mine"),
    cell("flagged_mine", "once the game is over: a flagged mine"),
    cell("wrong_flag", "once the game is over: a flag on no mine"),
    {
      sample = { grid.sample(ctx, { EXPLODED_KEY }, EXPLODED_HL) },
      meaning = "the mine that went off",
    },
  }
end

return M
