local helpers = require("helpers")
local wordle = require("games.games.wordle")

local assert_eq = helpers.assert_eq

local BOARD_ROWS = { 4, 9 }
local BOARD_WIDTH = 19

---sam, alone, after crane and route against "rouse": four guesses left.
---@param overrides? table
---@return games.WordleUpdate
local function update(overrides)
  return helpers.game_update("wordle", {
    seq = 2,
    players = { { user = "sam", seat = "solver" } },
    to_act = { "sam" },
    controls = { "resign" },
    view = {
      guesses = {
        { word = "crane", feedback = { "miss", "present", "miss", "miss", "hit" } },
        { word = "route", feedback = { "hit", "hit", "hit", "miss", "hit" } },
      },
      keyboard = {
        c = "miss",
        r = "hit",
        a = "miss",
        n = "miss",
        e = "hit",
        o = "hit",
        u = "hit",
        t = "miss",
      },
      remaining = 4,
    },
  }, overrides)
end

---Out of guesses: four more misses after crane and route.
---@return games.WordleUpdate
local function failed()
  local state = update({
    seq = 6,
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = {}, reason = "out_of_guesses" },
  })
  local miss = { word = "fight", feedback = { "miss", "miss", "miss", "miss", "miss" } }
  vim.list_extend(state.view.guesses, { miss, miss, miss, miss })
  state.view.remaining = 0
  state.view.answer = "rouse"
  return state
end

---The highlights of the board's cells on one line, left to right.
---@param rendered games.Rendered
---@param row integer 0-based
---@return games.Hl[]
local function tile_hls(rendered, row)
  return helpers.hls_on_row(rendered, row, BOARD_WIDTH)
end

local T = {}

function T.board_frames_hits_and_presents_and_lists_the_keyboard_beside_it()
  for _, case in ipairs({
    {
      glyphs = "unicode",
      board = {
        " C  (R)  A   N  [E]    Q  W [E][R][T] Y [U] I [O] P",
        "[R] [O] [U]  T  [E]     A  S  D  F  G  H  J  K  L",
        " ·   ·   ·   ·   ·       Z  X  C  V  B  N  M",
        " ·   ·   ·   ·   ·",
        " ·   ·   ·   ·   ·    guesses left 4",
        " ·   ·   ·   ·   ·",
      },
    },
    {
      glyphs = "ascii",
      board = {
        " C  (R)  A   N  [E]    Q  W [E][R][T] Y [U] I [O] P",
        "[R] [O] [U]  T  [E]     A  S  D  F  G  H  J  K  L",
        " .   .   .   .   .       Z  X  C  V  B  N  M",
        " .   .   .   .   .",
        " .   .   .   .   .    guesses left 4",
        " .   .   .   .   .",
      },
    },
  }) do
    local state = update()
    state.view.keyboard.t = "hit"
    local rendered = helpers.render_game("wordle", state, { me = "sam", glyphs = case.glyphs })
    local board = vim.tbl_map(function(line)
      return (line:gsub("%s+$", ""))
    end, vim.list_slice(rendered.lines, BOARD_ROWS[1], BOARD_ROWS[2]))
    assert_eq(case.board, board)
  end
end

function T.tiles_and_keys_take_their_marks_background()
  local rendered = helpers.render_game("wordle", update(), { me = "sam", glyphs = "unicode" })
  local first_line = BOARD_ROWS[1] - 1
  local miss, present, hit =
    { "GamesMiss", "GamesMissInk" },
    { "GamesPresent", "GamesPresentInk" },
    { "GamesHit", "GamesHitInk" }
  assert_eq({ miss, present, miss, miss, hit }, tile_hls(rendered, first_line))
  assert_eq({}, tile_hls(rendered, first_line + 2))
  local key_hls = vim
    .iter(helpers.extmarks(rendered))
    :filter(function(mark)
      return mark.row == first_line and mark.col >= BOARD_WIDTH
    end)
    :map(function(mark)
      return mark.hl
    end)
    :totable()
  assert_eq({ hit, hit, miss, hit, hit }, key_hls)
end

function T.finished_game_shows_the_answer_in_place_of_the_guesses_left()
  local rendered = helpers.render_game("wordle", failed(), { me = "sam", glyphs = "unicode" })
  local panel = table.concat(rendered.lines, "\n")
  assert(panel:find("answer ROUSE", 1, true), panel)
  assert(not panel:find("guesses left", 1, true), panel)
end

function T.events_name_the_player_and_the_word()
  local guessed = {
    kind = "guessed",
    by = "solver",
    word = "crane",
    feedback = { "miss", "miss", "miss", "miss", "hit" },
  }
  assert_eq("sam guessed CRANE", helpers.described(update(), guessed))
end

function T.the_legend_shows_each_mark_as_the_board_draws_it()
  local legend =
    wordle.legend({ art = helpers.view_options("wordle", "ascii").art, ambiwidth = "single" })
  local marks = vim.tbl_map(function(entry)
    return { entry.sample[1].str, entry.sample[1].hl, entry.meaning }
  end, vim.list_slice(legend, 1, 3))
  assert_eq({
    { "[A]", { "GamesHit", "GamesHitInk" }, "hit: the right letter in the right place" },
    { "(A)", { "GamesPresent", "GamesPresentInk" }, "present: in the word, but elsewhere" },
    { " A ", { "GamesMiss", "GamesMissInk" }, "miss: not in the word" },
  }, marks)
end

return T
