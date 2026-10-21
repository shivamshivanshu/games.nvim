local assets = require("games.ui.assets")
local helpers = require("helpers")
local shell = require("games.ui.shell")
local wordle = require("games.games.wordle")

local assert_eq = helpers.assert_eq

local CATALOGUED = { guess = true }
local BOARD_ROWS = { 4, 9 }
local BOARD_WIDTH = 19
local FOOTER_ROW = 11

---@param glyphs games.Glyphs
---@return games.ViewOptions
local function view_options(glyphs)
  local spec = { keys = wordle.asset_keys, glyph_width = wordle.glyph_width }
  local pack = assets.load("wordle", assets.DEFAULT_PACK, spec)
  return { glyphs = glyphs, flipped = false, art = assets.art_set(pack, glyphs) }
end

---sam, alone, after crane and route against "rouse": four guesses left.
---@param overrides? table
---@return games.WordleUpdate
local function update(overrides)
  return vim.tbl_extend("force", {
    game_id = "0000",
    game = "wordle",
    seq = 2,
    options = {},
    players = { { user = "sam", seat = "solver" } },
    to_act = { "sam" },
    controls = { "resign" },
    events = {},
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
  }, overrides or {})
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
  return vim
    .iter(rendered.extmarks)
    :filter(function(mark)
      return mark.row == row and mark.col < BOARD_WIDTH
    end)
    :map(function(mark)
      return mark.hl
    end)
    :totable()
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
    local rendered =
      shell.render(state, wordle, view_options(case.glyphs), "sam", 0, nil, CATALOGUED)
    local board = vim.tbl_map(function(line)
      return (line:gsub("%s+$", ""))
    end, vim.list_slice(rendered.lines, BOARD_ROWS[1], BOARD_ROWS[2]))
    assert_eq(case.board, board)
  end
end

function T.tiles_and_keys_take_their_marks_background()
  local rendered =
    shell.render(update(), wordle, view_options("unicode"), "sam", 0, nil, CATALOGUED)
  local first_line = BOARD_ROWS[1] - 1
  local miss, present, hit =
    { "GamesMiss", "GamesMissInk" },
    { "GamesPresent", "GamesPresentInk" },
    { "GamesHit", "GamesHitInk" }
  assert_eq({ miss, present, miss, miss, hit }, tile_hls(rendered, first_line))
  assert_eq({}, tile_hls(rendered, first_line + 2))
  local key_hls = vim
    .iter(rendered.extmarks)
    :filter(function(mark)
      return mark.row == first_line and mark.col >= BOARD_WIDTH
    end)
    :map(function(mark)
      return mark.hl
    end)
    :totable()
  assert_eq({ hit, hit, miss, hit, hit }, key_hls)
end

function T.guess_key_needs_an_ongoing_game_and_the_catalogue()
  for _, case in ipairs({
    {
      state = update(),
      catalogued = CATALOGUED,
      footer = "R give up  m guess  ]g next game  q close",
    },
    { state = update(), catalogued = nil, footer = "R give up  ]g next game  q close" },
    { state = failed(), catalogued = CATALOGUED, footer = "]g next game  q close" },
  }) do
    local rendered =
      shell.render(case.state, wordle, view_options("unicode"), "sam", 0, nil, case.catalogued)
    assert_eq(case.footer, rendered.lines[FOOTER_ROW])
  end
end

function T.finished_game_shows_the_answer_in_place_of_the_guesses_left()
  local rendered =
    shell.render(failed(), wordle, view_options("unicode"), "sam", 0, nil, CATALOGUED)
  local panel = table.concat(rendered.lines, "\n")
  assert(panel:find("answer ROUSE", 1, true), panel)
  assert(not panel:find("guesses left", 1, true), panel)
end

function T.input_sends_the_text_raw_for_the_server_to_parse()
  assert_eq("guess", wordle.input.kind)
  assert_eq({ kind = "guess", text = " Crane" }, wordle.input.parse(" Crane"))
end

function T.a_roomy_window_draws_block_letters_with_a_bar_on_hits()
  local rendered = shell.render(update(), wordle, view_options("unicode"), "sam", 0, {
    text_area = { w = 120, h = 40 },
  }, CATALOGUED)
  assert(
    vim.iter(rendered.lines):any(function(line)
      return line:find("█▀▀▀▀▐", 1, true) ~= nil
    end),
    table.concat(rendered.lines, "\n")
  )
end

function T.describe_event_names_the_player_and_the_word()
  assert_eq(
    "sam guessed CRANE",
    wordle.describe_event({
      kind = "guessed",
      by = "solver",
      word = "crane",
      feedback = { "miss", "miss", "miss", "miss", "hit" },
    }, update())
  )
  assert_eq(nil, wordle.describe_event({ kind = "other", by = "solver" }, update()))
end

return T
