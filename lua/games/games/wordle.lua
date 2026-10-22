local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local highlights = require("games.ui.highlights")
local kv = require("games.ui.widgets.kv")
local notation = require("games.ui.notation")
local text = require("games.ui.text")

local Group = highlights.Group

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

---@enum games.WordleMark
local Mark = { HIT = "hit", PRESENT = "present", MISS = "miss" }

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
local MARK_HL = {
  [Mark.HIT] = Group.HIT,
  [Mark.PRESENT] = Group.PRESENT,
  [Mark.MISS] = Group.MISS,
}

---@type games.WordleMark[] marks the pack frames; a miss draws the plain letter
local FRAMED_MARKS = { Mark.HIT, Mark.PRESENT }

---The asset key that draws letter under mark: a framed key for a hit or present, else the letter.
---@param letter string
---@param mark? games.WordleMark
---@return string
local function key_for(letter, mark)
  local framed = mark and vim.tbl_contains(FRAMED_MARKS, mark)
  return art.marked_key(letter, framed and mark or nil)
end

---@param view games.WordleView
---@return games.GridMark[]
local function marks_for(view)
  local feedback = vim.tbl_map(function(guess)
    return guess.feedback
  end, view.guesses)
  return grid.marks_where(feedback, function(mark)
    return MARK_HL[mark]
  end)
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
---@param ctx games.ArtLayoutCtx
---@return games.Node
local function keyboard(view, ctx)
  local set = ctx.art
  return compose.col(vim
    .iter(ipairs(KEYBOARD_ROWS))
    :map(function(i, letters)
      local keys = vim.tbl_map(function(letter)
        local mark = view.keyboard[letter]
        local glyph = art.glyph(set, key_for(letter, mark), ctx.ambiwidth)
        return compose.text(glyph, highlights.with_ink(MARK_HL[mark]))
      end, text.chars(letters))
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

M.Mark = Mark

---@type string[] keys every wordle asset pack draws: each letter plain, framed per mark, and an empty tile
M.asset_keys = vim.list_extend(
  vim
    .iter(notation.LETTERS)
    :map(function(letter)
      return { letter, key_for(letter, Mark.HIT), key_for(letter, Mark.PRESENT) }
    end)
    :flatten()
    :totable(),
  { EMPTY_KEY }
)

M.glyph_width = CELL_WIDTH

---@param update games.WordleUpdate
---@param ctx games.ArtLayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = #view.guesses + view.remaining,
      cols = WORD_LENGTH,
      cell_width = CELL_WIDTH,
      cell = function(r, c)
        return cell_for(view, r, c)
      end,
      marks = marks_for(view),
    },
    panels = {
      {
        width = PANEL_WIDTH,
        node = function()
          return compose.col({
            keyboard(view, ctx),
            compose.text(""),
            kv.panel(status_pairs(view)),
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
}

---@type table<games.EventKind, games.Describer>
M.describers = {
  guessed = function(event, talk)
    ---@cast event games.WordleGuessedEvent
    return ("%s guessed %s"):format(talk.name(event.by), event.word:upper())
  end,
}

---@param ctx games.DescribeCtx
---@return games.LegendEntry[]
function M.legend(ctx)
  local function marked(letter, mark)
    return grid.sample(ctx, { key_for(letter, mark) }, MARK_HL[mark])
  end
  local function tile(mark)
    return { marked("a", mark) }
  end
  local first_keys = text.chars(KEYBOARD_ROWS[1])
  local keyboard_sample = vim
    .iter(ipairs({ Mark.HIT, Mark.PRESENT, Mark.MISS }))
    :map(function(i, mark)
      return marked(first_keys[i], mark)
    end)
    :totable()
  return {
    { sample = tile(Mark.HIT), meaning = "hit: the right letter in the right place" },
    { sample = tile(Mark.PRESENT), meaning = "present: in the word, but elsewhere" },
    { sample = tile(Mark.MISS), meaning = "miss: not in the word" },
    grid.legend_entry(ctx, { EMPTY_KEY }, "a guess still to make"),
    { sample = keyboard_sample, meaning = "keyboard: each letter in its best mark so far" },
  }
end

return M
