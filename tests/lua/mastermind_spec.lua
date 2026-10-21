local helpers = require("helpers")
local mastermind = require("games.games.mastermind")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local REPO_ROOT = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")
local CATALOGUED = helpers.catalogued("mastermind")
local BOARD_ROWS = { 4, 15 }
local BOARD_WIDTH = 34

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("mastermind", mastermind, glyphs, extra)
end

---sam, alone, after rgby and oopp: eight guesses left.
---@param overrides? table
---@return games.MastermindUpdate
local function update(overrides)
  return helpers.game_update("mastermind", {
    seq = 2,
    players = { { user = "sam", seat = "solver" } },
    to_act = { "sam" },
    controls = { "resign" },
    view = {
      palette = { "r", "g", "b", "y", "o", "p" },
      code_length = 4,
      guesses = {
        { code = "rgby", black = 1, white = 2 },
        { code = "oopp", black = 0, white = 0 },
      },
      remaining = 8,
    },
  }, overrides)
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

---@param rendered games.Rendered
---@return integer how many cells carry the solved mark
local function solved_marks(rendered)
  return #vim.tbl_filter(function(mark)
    return type(mark.hl) == "table" and mark.hl[1] == "GamesWin"
  end, helpers.extmarks(rendered))
end

---The highlights of the board's cells on one line, left to right.
---@param rendered games.Rendered
---@param row integer 0-based
---@return games.Hl[]
local function cell_hls(rendered, row)
  return helpers.hls_on_row(rendered, row, BOARD_WIDTH)
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

function T.only_a_solved_result_marks_the_last_row()
  local not_solved = solved()
  not_solved.result = { kind = "decisive", winners = {}, reason = "resign" }
  for _, case in ipairs({ { solved(), 4 }, { not_solved, 0 }, { update(), 0 } }) do
    local rendered =
      shell.render(case[1], mastermind, view_options("unicode"), "sam", 0, nil, CATALOGUED)
    assert_eq(case[2], solved_marks(rendered))
  end
end

---The palette the server sent at the start of the conformance transcript.
---@return string[]
local function transcript_palette()
  local path = REPO_ROOT .. "/docs/conformance/mastermind.ndjson"
  for line in io.lines(path) do
    local entry = vim.json.decode(line)
    local body = entry.recv and entry.recv.body
    if type(body) == "table" and type(body.view) == "table" and body.view.palette then
      return body.view.palette
    end
  end
  error("no palette in " .. path)
end

function T.pack_draws_a_peg_for_exactly_the_servers_palette()
  local pegs = vim
    .iter(mastermind.asset_keys)
    :filter(function(key)
      return vim.startswith(key, "peg:")
    end)
    :map(function(key)
      return key:sub(#"peg:" + 1)
    end)
    :totable()
  assert_eq(transcript_palette(), pegs)
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

function T.events_name_the_player_the_code_and_the_pegs()
  local guessed = { kind = "guessed", by = "solver", code = "rgby", black = 1, white = 2 }
  assert_eq("sam guessed RGBY: 1 black, 2 white", helpers.described(update(), guessed))
end

return T
