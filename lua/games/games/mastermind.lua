local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local types = require("games.types")

local Group = highlights.Group

---@alias games.MastermindColour "r"|"g"|"b"|"y"|"o"|"p"

---@class games.MastermindGuess
---@field code string one palette letter per peg
---@field black integer pegs of the right colour in the right place
---@field white integer pegs of the right colour in the wrong place

---@class games.MastermindView
---@field palette games.MastermindColour[] the colours a peg may take
---@field code_length integer pegs per code
---@field guesses games.MastermindGuess[] oldest first
---@field remaining integer guesses left
---@field secret? string set once the game has finished

---@class games.MastermindUpdate: games.GameUpdate
---@field view games.MastermindView

---@class games.MastermindGuessedEvent: games.Event
---@field kind "guessed"
---@field by string
---@field code string
---@field black integer
---@field white integer

---@type games.MastermindColour[] every colour a pack draws a peg for
local COLOURS = { "r", "g", "b", "y", "o", "p" }
local CELL_WIDTH = 3
---Rows above the guesses: the secret, then a blank one.
local SECRET_ROWS = 2
local GAP_COLS = 1
---@enum games.MastermindKey asset keys other than the coloured pegs
local Key = {
  HOLE = "hole",
  HIDDEN = "hidden",
  BLACK = "black",
  WHITE = "white",
  NO_KEY = "none",
  GAP = "gap",
}
local PEG_MARKER = "peg"
local BLACK_MEANING, WHITE_MEANING = "right colour, right place", "right colour, wrong place"
local SOLVED_HL = Group.WIN

---@type table<games.MastermindColour, string>
local PEG_HL = {
  r = Group.PEG_RED,
  g = Group.PEG_GREEN,
  b = Group.PEG_BLUE,
  y = Group.PEG_YELLOW,
  o = Group.PEG_ORANGE,
  p = Group.PEG_PURPLE,
}

---@param colour string
---@return string
local function peg_key(colour)
  return art.marked_key(colour, PEG_MARKER)
end

---@param colour string
---@return games.GridCell
local function peg(colour)
  return { peg_key(colour), PEG_HL[colour] }
end

---The key peg at position i of a guess's feedback: blacks first, then whites, then none.
---@param guess games.MastermindGuess
---@param i integer
---@return games.GridCell
local function key_peg(guess, i)
  if i <= guess.black then
    return { Key.BLACK }
  end
  if i <= guess.black + guess.white then
    return { Key.WHITE }
  end
  return { Key.NO_KEY }
end

---@param view games.MastermindView
---@param col integer
---@return games.GridCell
local function secret_cell(view, col)
  if col > view.code_length then
    return { Key.GAP }
  end
  return view.secret and peg(view.secret:sub(col, col)) or { Key.HIDDEN }
end

---@param view games.MastermindView
---@param row integer
---@param col integer
---@return games.GridCell
local function cell_for(view, row, col)
  if row == 1 then
    return secret_cell(view, col)
  end
  if row <= SECRET_ROWS then
    return { Key.GAP }
  end
  local guess = view.guesses[row - SECRET_ROWS]
  if col <= view.code_length then
    return guess and peg(guess.code:sub(col, col)) or { Key.HOLE }
  end
  local key_col = col - view.code_length - GAP_COLS
  if key_col < 1 then
    return { Key.GAP }
  end
  return guess and key_peg(guess, key_col) or { Key.NO_KEY }
end

---@param update games.MastermindUpdate
---@return games.GridMark[] the solving guess's pegs, once the server says the code is solved
local function marks_for(update)
  local view = update.view
  if not (update.result and update.result.reason == types.SOLVED_REASON) then
    return {}
  end
  return vim
    .iter(vim.fn.range(1, view.code_length))
    :map(function(col)
      return grid.mark(SECRET_ROWS + #view.guesses, col, SOLVED_HL)
    end)
    :totable()
end

---@param view games.MastermindView
---@param ctx games.ArtLayoutCtx
---@return games.Node
local function panel(view, ctx)
  local set = ctx.art
  local function glyph(key)
    return vim.trim(art.glyph(set, key, ctx.ambiwidth))
  end
  local legend = {}
  for i, colour in ipairs(view.palette) do
    legend[#legend + 1] = compose.text(
      (i > 1 and "  " or "") .. glyph(peg_key(colour)) .. " " .. colour,
      PEG_HL[colour]
    )
  end
  return compose.col({
    compose.row(legend),
    compose.text(glyph(Key.BLACK) .. " " .. BLACK_MEANING, Group.LABEL),
    compose.text(glyph(Key.WHITE) .. " " .. WHITE_MEANING, Group.LABEL),
    compose.text(""),
    kv.panel({ { "guesses left", tostring(view.remaining) } }),
  })
end

local M = {}

---@type string[] keys every mastermind asset pack draws: a peg per colour, an empty hole, a hidden
---peg, the black, white and absent key pegs, and the gap before them
M.asset_keys = vim.list_extend(
  vim.tbl_map(peg_key, COLOURS),
  { Key.HOLE, Key.HIDDEN, Key.BLACK, Key.WHITE, Key.NO_KEY, Key.GAP }
)

M.glyph_width = CELL_WIDTH

---@param update games.MastermindUpdate
---@param ctx games.ArtLayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = SECRET_ROWS + #view.guesses + view.remaining,
      cols = view.code_length + GAP_COLS + view.code_length,
      cell_width = CELL_WIDTH,
      cell = function(r, c)
        return cell_for(view, r, c)
      end,
      marks = marks_for(update),
    },
    panels = {
      {
        node = function()
          return panel(view, ctx)
        end,
      },
    },
  }, ctx)
end

---@type games.GameInput
M.input = {
  kind = "guess",
  prompt = "Guess: ",
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  guessed = function(event, talk)
    ---@cast event games.MastermindGuessedEvent
    return ("%s guessed %s: %d black, %d white"):format(
      talk.name(event.by),
      event.code:upper(),
      event.black,
      event.white
    )
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local red = peg(COLOURS[1])
  return {
    grid.legend_entry(ctx, red, "a code peg, by colour, named in the panel"),
    grid.legend_entry(ctx, { Key.BLACK }, "key peg: " .. BLACK_MEANING),
    grid.legend_entry(ctx, { Key.WHITE }, "key peg: " .. WHITE_MEANING),
    grid.legend_entry(ctx, { Key.NO_KEY }, "no key peg"),
    grid.legend_entry(ctx, { Key.HOLE }, "a guess still to make"),
    grid.legend_entry(ctx, { Key.HIDDEN }, "the secret, hidden until the game is over"),
    grid.legend_entry(ctx, red, "the guess that solved the code", SOLVED_HL),
  }
end

return M
