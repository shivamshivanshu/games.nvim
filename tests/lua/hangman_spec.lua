local hangman = require("games.games.hangman")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUED = { guess = true }
local BOARD_ROWS = { 4, 9 }
local FOOTER_ROW = 11

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("hangman", hangman, glyphs, extra)
end

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
  return vim.tbl_map(function(line)
    return (line:gsub("%s+$", ""))
  end, vim.list_slice(rendered.lines, BOARD_ROWS[1], BOARD_ROWS[2]))
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
    local rendered =
      shell.render(update(), hangman, view_options(case.glyphs), "sam", 0, nil, CATALOGUED)
    assert_eq(case.board, board_lines(rendered))
  end
end

function T.the_gallows_falls_back_to_ascii_where_box_drawing_widens()
  helpers.with_overrides({ { vim.o, "ambiwidth", "double" } }, function()
    local rendered = shell.render(
      update(),
      hangman,
      view_options("unicode"),
      "sam",
      0,
      { text_area = { w = 120, h = 40 }, ambiwidth = "double" },
      CATALOGUED
    )
    local text = table.concat(rendered.lines, "\n")
    assert(text:find("+---+", 1, true) and not text:find("┌", 1, true), text)
  end)
end

function T.hanged_board_draws_the_whole_figure_and_frames_the_unfound_letters()
  local rendered = shell.render(hanged(), hangman, view_options("ascii"), "sam", 0, nil, CATALOGUED)
  local lines = board_lines(rendered)
  assert_eq("(A)  B   O  (U) (T)     +---+     found       b o", lines[1])
  assert_eq("                        |  / \\    word        ABOUT", lines[5])
  assert(lines[4]:find("|  /|\\", 1, true), lines[4])
  local unfound = vim
    .iter(rendered.extmarks)
    :filter(function(mark)
      return mark.row == BOARD_ROWS[1] - 1 and mark.hl == "GamesLost"
    end)
    :totable()
  assert_eq(3, #unfound)
end

function T.guess_key_needs_an_ongoing_game_and_the_catalogue()
  for _, case in ipairs({
    {
      state = update(),
      catalogued = CATALOGUED,
      footer = "R give up  m guess  ]g next game  q close",
    },
    { state = update(), catalogued = nil, footer = "R give up  ]g next game  q close" },
    { state = hanged(), catalogued = CATALOGUED, footer = "]g next game  q close" },
  }) do
    local rendered =
      shell.render(case.state, hangman, view_options("unicode"), "sam", 0, nil, case.catalogued)
    assert_eq(case.footer, rendered.lines[FOOTER_ROW])
  end
end

function T.a_roomy_window_draws_block_letters_and_a_bar_for_a_hidden_one()
  local rendered = shell.render(update(), hangman, view_options("unicode"), "sam", 0, {
    text_area = { w = 120, h = 40 },
  }, CATALOGUED)
  local text = table.concat(rendered.lines, "\n")
  assert(
    text:find("████████", 1, true) and text:find("▀▀▀▀▀▀", 1, true),
    text
  )
end

function T.input_sends_the_text_raw_for_the_server_to_parse()
  assert_eq("guess", hangman.input.kind)
  assert_eq({ kind = "guess", text = " E" }, hangman.input.parse(" E"))
end

function T.describe_event_names_the_player_the_guess_and_whether_it_hit()
  assert_eq(
    "sam guessed E: miss",
    hangman.describe_event({ kind = "guessed", by = "solver", guess = "e", hit = false }, update())
  )
  assert_eq(
    "sam guessed ABOUT: hit",
    hangman.describe_event(
      { kind = "guessed", by = "solver", guess = "about", hit = true },
      update()
    )
  )
  assert_eq(nil, hangman.describe_event({ kind = "other", by = "solver" }, update()))
end

return T
