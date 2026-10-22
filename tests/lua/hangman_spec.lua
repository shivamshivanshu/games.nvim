local describe = require("games.core.describe")
local hangman = require("games.games.hangman")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

local BOARD_ROWS = { 4, 9 }

---sam, alone, against "about": b and o found, e and q missed, and the word crane missed.
---@param overrides? table
---@return games.HangmanUpdate
local function update(overrides)
  return helpers.game_update("hangman", {
    seq = 5,
    players = { { user = "sam", seat = "solver" } },
    to_act = { "sam" },
    controls = { "resign" },
    view = {
      pattern = { "", "b", "o", "", "" },
      guessed = {
        { letter = "b", hit = true },
        { letter = "e", hit = false },
        { letter = "o", hit = true },
        { letter = "q", hit = false },
      },
      missed_words = { "crane" },
      misses = 3,
      remaining = 3,
    },
  }, overrides)
end

---Hanged by three more misses, so the word shows.
---@return games.HangmanUpdate
local function hanged()
  local state = update({
    seq = 8,
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = {}, reason = "hanged" },
  })
  vim.list_extend(state.view.guessed, {
    { letter = "z", hit = false },
    { letter = "x", hit = false },
    { letter = "k", hit = false },
  })
  state.view.misses, state.view.remaining, state.view.word = 6, 0, "about"
  return state
end

---@param rendered games.Rendered
---@return string[]
local function board_lines(rendered)
  return helpers.trimmed_lines(rendered, BOARD_ROWS[1], BOARD_ROWS[2])
end

local T = {}

function T.board_shows_the_word_as_found_the_gallows_and_the_guesses()
  for _, case in ipairs({
    {
      glyphs = "unicode",
      board = {
        " _   B   O   _   _      ┌───┐     found       b o",
        "                        │   │     missed      e q",
        "                        │   O     wrong word  crane",
        "                        │  ╱│     misses left 3",
        "                        │",
        "                      ──┴─────",
      },
    },
    {
      glyphs = "ascii",
      board = {
        " _   B   O   _   _      +---+     found       b o",
        "                        |   |     missed      e q",
        "                        |   O     wrong word  crane",
        "                        |  /|     misses left 3",
        "                        |",
        "                      --+-----",
      },
    },
  }) do
    local rendered = helpers.render_game("hangman", update(), { me = "sam", glyphs = case.glyphs })
    assert_eq(case.board, board_lines(rendered))
  end
end

function T.the_gallows_falls_back_to_ascii_where_box_drawing_widens()
  helpers.with_overrides({ { vim.o, "ambiwidth", "double" } }, function()
    local rendered = helpers.render_game("hangman", update(), {
      me = "sam",
      glyphs = "unicode",
      display = { text_area = { w = 120, h = 40 }, ambiwidth = "double" },
    })
    local text = table.concat(rendered.lines, "\n")
    assert(text:find("+---+", 1, true) and not text:find("┌", 1, true), text)
  end)
end

function T.hanged_board_draws_the_whole_figure_and_frames_the_unfound_letters()
  local rendered = helpers.render_game("hangman", hanged(), { me = "sam" })
  local lines = board_lines(rendered)
  assert_eq("(A)  B   O  (U) (T)     +---+     found       b o", lines[1])
  assert_eq("                        |  / \\    word        ABOUT", lines[5])
  assert(lines[4]:find("|  /|\\", 1, true), lines[4])
  local unfound = vim
    .iter(helpers.extmarks(rendered))
    :filter(function(mark)
      return mark.row == BOARD_ROWS[1] - 1 and mark.hl == "GamesLost"
    end)
    :totable()
  assert_eq(3, #unfound)
end

function T.events_name_the_player_the_guess_and_whether_it_hit()
  local events = {
    { kind = "guessed", by = "solver", guess = "e", hit = false },
    { kind = "guessed", by = "solver", guess = "about", hit = true },
  }
  assert_eq(
    "sam guessed E: miss; sam guessed ABOUT: hit",
    describe.events(update({ events = events }))
  )
end

return T
