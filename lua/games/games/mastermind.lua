local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")

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
local SOLVED_HL = "GamesWin"
local SOLVED_REASON = "solved"

---@type table<games.MastermindColour, string>
local PEG_HL = {
  r = "GamesPegRed",
  g = "GamesPegGreen",
  b = "GamesPegBlue",
  y = "GamesPegYellow",
  o = "GamesPegOrange",
  p = "GamesPegPurple",
}

---@param colour string
---@return string
local function peg_key(colour)
  return "peg:" .. colour
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
  if not (update.result and update.result.reason == SOLVED_REASON) then
    return {}
  end
  return vim
    .iter(vim.fn.range(1, view.code_length))
    :map(function(col)
      return { row = SECRET_ROWS + #view.guesses, col = col, hl = SOLVED_HL }
    end)
    :totable()
end

---@param view games.MastermindView
---@param ctx games.LayoutCtx
---@return games.Node
local function panel(view, ctx)
  local set = assert(ctx.art, "mastermind draws from an asset pack")
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
    compose.text(glyph(Key.BLACK) .. " right colour, right place", "GamesLabel"),
    compose.text(glyph(Key.WHITE) .. " right colour, wrong place", "GamesLabel"),
    compose.text(""),
    kv({
      pairs = { { "guesses left", tostring(view.remaining) } },
      key_hl = "GamesKey",
      value_hl = "GamesLabel",
    }),
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
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = SECRET_ROWS + #view.guesses + view.remaining,
      cols = view.code_length + GAP_COLS + view.code_length,
      shade = "plain",
      cell_width = CELL_WIDTH,
      cell = function(r, c)
        return cell_for(view, r, c)
      end,
      marks = marks_for(update),
    },
    art = assert(ctx.art, "mastermind draws from an asset pack"),
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
  parse = function(text)
    return { kind = "guess", code = text }
  end,
}

---@param event games.Event
---@param update games.MastermindUpdate
---@return string|nil
function M.describe_event(event, update)
  if event.kind ~= "guessed" then
    return nil
  end
  ---@cast event games.MastermindGuessedEvent
  return ("%s guessed %s: %d black, %d white"):format(
    players.user_of_seat(update, event.by) or event.by,
    event.code:upper(),
    event.black,
    event.white
  )
end

return M
