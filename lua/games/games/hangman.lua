local art = require("games.ui.art")
local board = require("games.ui.widgets.board")
local compose = require("games.ui.compose")
local kv = require("games.ui.widgets.kv")

---@class games.HangmanLetterGuess
---@field letter string
---@field hit boolean whether the word holds it

---@class games.HangmanView
---@field pattern string[] one per letter of the word: the letter once found, "" while hidden
---@field guessed games.HangmanLetterGuess[] oldest first
---@field missed_words string[] wrong whole-word guesses, oldest first
---@field misses integer wrong guesses of either kind
---@field remaining integer misses left before the hanging
---@field word? string set once the game has finished

---@class games.HangmanUpdate: games.GameUpdate
---@field view games.HangmanView

---@class games.HangmanGuessedEvent: games.Event
---@field kind "guessed"
---@field by string
---@field guess string a letter or a whole word
---@field hit boolean

local LETTERS = "abcdefghijklmnopqrstuvwxyz"
local CELL_WIDTH = 3
local HIDDEN_KEY = "hidden"
local UNFOUND_PREFIX = "unfound:"
local GALLOWS_PREFIX = "gallows:"
---The gallows drawn after 0 to this many misses.
local GALLOWS_STAGES = 6
local UNFOUND_HL = "GamesLost"
local NONE = "-"
local WRONG_WORD_LABEL = "wrong word"
local MISSES_LEFT_LABEL = "misses left"
---Display columns the status panel reserves: its widest label, a space, and the widest value, a
---letter list as long as the most misses allowed.
local STATUS_WIDTH = #MISSES_LEFT_LABEL + 1 + 2 * GALLOWS_STAGES - 1

---@param letter string
---@return string
local function unfound_key(letter)
  return UNFOUND_PREFIX .. letter
end

---@param misses integer
---@return string
local function gallows_key(misses)
  return GALLOWS_PREFIX .. misses
end

---A found letter as itself; a hidden one blank, or once the game is over the word's letter,
---framed and tinted as unfound.
---@param view games.HangmanView
---@param col integer
---@return games.GridCell
local function cell_for(view, col)
  local found = view.pattern[col]
  if found ~= "" then
    return { found }
  end
  if view.word then
    return { unfound_key(view.word:sub(col, col)), UNFOUND_HL }
  end
  return { HIDDEN_KEY }
end

---@param letters string[]
---@return string
local function spaced(letters)
  return #letters > 0 and table.concat(letters, " ") or NONE
end

---@param view games.HangmanView
---@param hit boolean
---@return string[]
local function guessed_letters(view, hit)
  return vim
    .iter(view.guessed)
    :filter(function(guess)
      return guess.hit == hit
    end)
    :map(function(guess)
      return guess.letter
    end)
    :totable()
end

---@param view games.HangmanView
---@return games.KvPair[]
local function status_pairs(view)
  local pairs = {
    { "found", spaced(guessed_letters(view, true)) },
    { "missed", spaced(guessed_letters(view, false)) },
  }
  for _, word in ipairs(view.missed_words) do
    pairs[#pairs + 1] = { WRONG_WORD_LABEL, word }
  end
  pairs[#pairs + 1] = { MISSES_LEFT_LABEL, tostring(view.remaining) }
  if view.word then
    pairs[#pairs + 1] = { "word", view.word:upper() }
  end
  return pairs
end

---@param view games.HangmanView
---@param ctx games.LayoutCtx
---@return games.Node
local function gallows(view, ctx)
  local set = assert(ctx.art, "hangman draws from an asset pack")
  local lines = art.picture(set, gallows_key(view.misses), ctx.ambiwidth)
  return compose.col(vim.tbl_map(function(line)
    return compose.text(line)
  end, lines))
end

local M = {}

---@type string[] keys every hangman asset pack draws: each letter found, each letter unfound at the
---end, and a hidden letter
M.asset_keys = vim.list_extend(
  vim
    .iter(vim.split(LETTERS, ""))
    :map(function(letter)
      return { letter, unfound_key(letter) }
    end)
    :flatten()
    :totable(),
  { HIDDEN_KEY }
)

---@type string[] the gallows after each number of misses
M.picture_keys = vim.tbl_map(gallows_key, vim.fn.range(0, GALLOWS_STAGES))

M.glyph_width = CELL_WIDTH

---@param update games.HangmanUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = 1,
      cols = #view.pattern,
      shade = "plain",
      cell_width = CELL_WIDTH,
      cell = function(_, c)
        return cell_for(view, c)
      end,
    },
    art = assert(ctx.art, "hangman draws from an asset pack"),
    panels = {
      {
        node = function()
          return gallows(view, ctx)
        end,
      },
      {
        width = STATUS_WIDTH,
        node = function()
          return kv({ pairs = status_pairs(view), key_hl = "GamesKey", value_hl = "GamesLabel" })
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
    ---@cast event games.HangmanGuessedEvent
    return ("%s guessed %s: %s"):format(
      talk.name(event.by),
      event.guess:upper(),
      event.hit and "hit" or "miss"
    )
  end,
}

return M
