local assets = require("games.ui.assets")
local helpers = require("helpers")
local mastermind = require("games.games.mastermind")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUED = { guess = true }
local BOARD_ROWS = { 4, 15 }
local BOARD_WIDTH = 34
local FOOTER_ROW = 17

---@param glyphs games.Glyphs
---@return games.ViewOptions
local function view_options(glyphs)
  local spec = { keys = mastermind.asset_keys, glyph_width = mastermind.glyph_width }
  local pack = assets.load("mastermind", assets.DEFAULT_PACK, spec)
  return { glyphs = glyphs, flipped = false, art = assets.art_set(pack, glyphs) }
end

---sam, alone, after rgby and oopp: eight guesses left.
---@param overrides? table
---@return games.MastermindUpdate
local function update(overrides)
  return vim.tbl_extend("force", {
    game_id = "0000",
    game = "mastermind",
    seq = 2,
    options = {},
    players = { { user = "sam", seat = "solver" } },
    to_act = { "sam" },
    controls = { "resign" },
    events = {},
    view = {
      palette = { "r", "g", "b", "y", "o", "p" },
      code_length = 4,
      guesses = {
        { code = "rgby", black = 1, white = 2 },
        { code = "oopp", black = 0, white = 0 },
      },
      remaining = 8,
    },
  }, overrides or {})
end

---Solved on the third guess, so the secret shows.
---@return games.MastermindUpdate
local function solved()
  local state = update({
    seq = 3,
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = { "solver" }, reason = "solved" },
  })
  table.insert(state.view.guesses, { code = "rbgg", black = 4, white = 0 })
  state.view.remaining = 7
  state.view.secret = "rbgg"
  return state
end

---@param rendered games.Rendered
---@return string[] the board's lines, trailing space trimmed
local function board_lines(rendered)
  return vim.tbl_map(function(line)
    return (vim.fn.strcharpart(line, 0, BOARD_WIDTH):gsub("%s+$", ""))
  end, vim.list_slice(rendered.lines, BOARD_ROWS[1], BOARD_ROWS[2]))
end

---The highlights of the board's cells on one line, left to right.
---@param rendered games.Rendered
---@param row integer 0-based
---@return games.Hl[]
local function cell_hls(rendered, row)
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

function T.board_shows_the_hidden_secret_each_guess_and_its_key_pegs()
  local empty = {
    unicode = " ◦   ◦   ◦   ◦       ·   ·   ·   ·",
    ascii = " .   .   .   .       .   .   .   .",
  }
  for _, case in ipairs({
    {
      glyphs = "unicode",
      head = {
        " ?   ?   ?   ?",
        "",
        " Ⓡ   Ⓖ   Ⓑ   Ⓨ       ●   ○   ○   ·",
        " Ⓞ   Ⓞ   Ⓟ   Ⓟ       ·   ·   ·   ·",
      },
    },
    {
      glyphs = "ascii",
      head = {
        " ?   ?   ?   ?",
        "",
        " R   G   B   Y       *   +   +   .",
        " O   O   P   P       .   .   .   .",
      },
    },
  }) do
    local rendered =
      shell.render(update(), mastermind, view_options(case.glyphs), "sam", 0, nil, CATALOGUED)
    local want =
      vim.list_extend(vim.deepcopy(case.head), vim.fn["repeat"]({ empty[case.glyphs] }, 8))
    assert_eq(want, board_lines(rendered))
  end
end

function T.pegs_take_their_colour_and_the_solving_row_is_marked()
  local rendered =
    shell.render(update(), mastermind, view_options("unicode"), "sam", 0, nil, CATALOGUED)
  local first_guess = BOARD_ROWS[1] + 1
  assert_eq(
    { "GamesPegRed", "GamesPegGreen", "GamesPegBlue", "GamesPegYellow" },
    cell_hls(rendered, first_guess)
  )
  assert_eq({}, cell_hls(rendered, BOARD_ROWS[1] - 1))

  local finished =
    shell.render(solved(), mastermind, view_options("unicode"), "sam", 0, nil, CATALOGUED)
  assert_eq(" Ⓡ   Ⓑ   Ⓖ   Ⓖ", board_lines(finished)[1])
  local solving = cell_hls(finished, first_guess + 2)
  assert_eq(4, #solving)
  for _, hls in ipairs(solving) do
    assert_eq("GamesWin", hls[1])
  end
  assert_eq(
    {},
    vim.tbl_filter(function(hls)
      return type(hls) == "table" and hls[1] == "GamesWin"
    end, cell_hls(rendered, first_guess))
  )
end

function T.panel_lists_the_palette_the_key_pegs_and_guesses_left()
  local rendered =
    shell.render(update(), mastermind, view_options("ascii"), "sam", 0, nil, CATALOGUED)
  local panel = table.concat(rendered.lines, "\n")
  for _, wanted in ipairs({
    "R r  G g  B b  Y y  O o  P p",
    "* right colour, right place",
    "+ right colour, wrong place",
    "guesses left 8",
  }) do
    assert(panel:find(wanted, 1, true), panel)
  end
end

function T.guess_key_needs_an_ongoing_game_and_the_catalogue()
  for _, case in ipairs({
    {
      state = update(),
      catalogued = CATALOGUED,
      footer = "R give up  m guess  ]g next game  q close",
    },
    { state = update(), catalogued = nil, footer = "R give up  ]g next game  q close" },
    { state = solved(), catalogued = CATALOGUED, footer = "]g next game  q close" },
  }) do
    local rendered =
      shell.render(case.state, mastermind, view_options("unicode"), "sam", 0, nil, case.catalogued)
    assert_eq(case.footer, rendered.lines[FOOTER_ROW])
  end
end

function T.input_sends_the_code_raw_for_the_server_to_parse()
  assert_eq("guess", mastermind.input.kind)
  assert_eq({ kind = "guess", code = " RGby" }, mastermind.input.parse(" RGby"))
end

function T.describe_event_names_the_player_the_code_and_the_pegs()
  assert_eq(
    "sam guessed RGBY: 1 black, 2 white",
    mastermind.describe_event(
      { kind = "guessed", by = "solver", code = "rgby", black = 1, white = 2 },
      update()
    )
  )
  assert_eq(nil, mastermind.describe_event({ kind = "other", by = "solver" }, update()))
end

return T
