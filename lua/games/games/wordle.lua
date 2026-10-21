local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local players = require("games.core.players")

---@alias games.WordleMark "hit"|"present"|"miss"

---@class games.WordleGuess
---@field word string
---@field feedback games.WordleMark[] one per letter

---@class games.WordleView
---@field guesses games.WordleGuess[] oldest first
---@field keyboard table<string, games.WordleMark> each guessed letter's best mark
---@field remaining integer guesses left
---@field answer? string set once the game has finished

---@class games.WordleUpdate: games.GameUpdate
---@field view games.WordleView

---@class games.WordleGuessedEvent: games.Event
---@field kind "guessed"
---@field by string
---@field word string
---@field feedback games.WordleMark[]

local LETTERS = "abcdefghijklmnopqrstuvwxyz"
local WORD_LENGTH = 5
local CELL_WIDTH = 3
local EMPTY_KEY = "empty"
local KEYBOARD_ROWS = { "qwertyuiop", "asdfghjkl", "zxcvbnm" }
---Display columns the side panel reserves: its widest keyboard row, indent included; row i is
---indented i - 1 columns.
local PANEL_WIDTH = vim.iter(ipairs(KEYBOARD_ROWS)):fold(0, function(widest, i, letters)
  return math.max(widest, i - 1 + CELL_WIDTH * #letters)
end)

---@type table<games.WordleMark, string>
local MARK_HL = { hit = "GamesHit", present = "GamesPresent", miss = "GamesMiss" }

---@type games.WordleMark[] marks the pack frames; a miss draws the plain letter
local FRAMED_MARKS = { "hit", "present" }

---The asset key that draws letter under mark: a framed key for a hit or present, else the letter.
---@param letter string
---@param mark? games.WordleMark
---@return string
local function key_for(letter, mark)
  if mark and vim.tbl_contains(FRAMED_MARKS, mark) then
    return ("%s:%s"):format(mark, letter)
  end
  return letter
end

---@param view games.WordleView
---@return games.GridMark[]
local function marks_for(view)
  local marks = {}
  for row, guess in ipairs(view.guesses) do
    for col, mark in ipairs(guess.feedback) do
      marks[#marks + 1] = { row = row, col = col, hl = MARK_HL[mark] }
    end
  end
  return marks
end

---@param view games.WordleView
---@param row integer
---@param col integer
---@return games.GridCell
local function cell_for(view, row, col)
  local guess = view.guesses[row]
  if not guess then
    return { EMPTY_KEY }
  end
  return { key_for(guess.word:sub(col, col), guess.feedback[col]) }
end

---@param view games.WordleView
---@param ctx games.LayoutCtx
---@return games.Node
local function keyboard(view, ctx)
  local set = assert(ctx.art, "wordle draws from an asset pack")
  return compose.col(vim
    .iter(ipairs(KEYBOARD_ROWS))
    :map(function(i, letters)
      local keys = vim.tbl_map(function(letter)
        local mark = view.keyboard[letter]
        local glyph = art.glyph(set, key_for(letter, mark), ctx.ambiwidth)
        return compose.text(glyph, highlights.with_ink(MARK_HL[mark]))
      end, vim.split(letters, ""))
      return compose.pad(i - 1, compose.row(keys))
    end)
    :totable())
end

---@param view games.WordleView
---@return games.KvPair[]
local function status_pairs(view)
  if view.answer then
    return { { "answer", view.answer:upper() } }
  end
  return { { "guesses left", tostring(view.remaining) } }
end

local M = {}

---@type string[] keys every wordle asset pack draws: each letter plain, framed per mark, and an empty tile
M.asset_keys = vim.list_extend(
  vim
    .iter(vim.split(LETTERS, ""))
    :map(function(letter)
      return { letter, key_for(letter, "hit"), key_for(letter, "present") }
    end)
    :flatten()
    :totable(),
  { EMPTY_KEY }
)

M.glyph_width = CELL_WIDTH

---@param update games.WordleUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = #view.guesses + view.remaining,
      cols = WORD_LENGTH,
      shade = "plain",
      cell_width = CELL_WIDTH,
      cell = function(r, c)
        return cell_for(view, r, c)
      end,
      marks = marks_for(view),
    },
    art = assert(ctx.art, "wordle draws from an asset pack"),
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return compose.col({
            keyboard(view, ctx),
            compose.text(""),
            kv({ pairs = status_pairs(view), key_hl = "GamesKey", value_hl = "GamesLabel" }),
          })
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
    return { kind = "guess", text = text }
  end,
}

---@param event games.Event
---@param update games.WordleUpdate
---@return string|nil
function M.describe_event(event, update)
  if event.kind ~= "guessed" then
    return nil
  end
  ---@cast event games.WordleGuessedEvent
  return ("%s guessed %s"):format(
    players.user_of_seat(update, event.by) or event.by,
    event.word:upper()
  )
end

return M
