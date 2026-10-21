local fifteen = require("games.games.fifteen")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUED = { slide = true, move = true }
local FIRST_BOARD_ROW = 3

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("fifteen", fifteen, glyphs, extra)
end

---alice's 3x3 puzzle with the blank in the bottom right corner's left neighbour.
---@param overrides? table
---@return games.FifteenUpdate
local function update(overrides)
  return vim.tbl_extend("force", {
    game_id = "0000",
    game = "fifteen",
    seq = 4,
    options = { size = "3x3" },
    players = { { user = "alice", seat = "solver" } },
    to_act = { "alice" },
    controls = { "resign" },
    events = {},
    view = {
      tiles = { { 1, 2, 3 }, { 4, 5, 6 }, { 7, 0, 8 } },
      moves = 4,
      movable = {
        { tile = 5, direction = "down" },
        { tile = 7, direction = "right" },
        { tile = 8, direction = "left" },
      },
    },
  }, overrides or {})
end

---@return games.FifteenUpdate solved by sliding 8 left
local function solved()
  return update({
    seq = 5,
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = { "alice" }, reason = "solved" },
    view = { tiles = { { 1, 2, 3 }, { 4, 5, 6 }, { 7, 8, 0 } }, moves = 5, movable = {} },
  })
end

local T = {}

function T.board_brackets_and_tints_the_movable_tiles_beside_the_move_count()
  for _, glyphs in ipairs({ "unicode", "ascii" }) do
    local rendered =
      shell.render(update(), fifteen, view_options(glyphs), "alice", 0, nil, CATALOGUED)
    assert_eq({
      "  1    2    3    moves 4",
      "  4  [ 5]   6           ",
      "[ 7]      [ 8]          ",
    }, vim.list_slice(rendered.lines, FIRST_BOARD_ROW + 1, FIRST_BOARD_ROW + 3))
  end
  local rendered =
    shell.render(update(), fifteen, view_options("unicode"), "alice", 0, nil, CATALOGUED)
  local movable = { "GamesMovable", "GamesMovableInk" }
  assert_eq({ movable }, helpers.hls_on_row(rendered, FIRST_BOARD_ROW + 1))
  assert_eq({ movable, movable }, helpers.hls_on_row(rendered, FIRST_BOARD_ROW + 2))
end

function T.a_solved_board_tints_every_tile_and_offers_no_move()
  local rendered =
    shell.render(solved(), fifteen, view_options("ascii"), "alice", 0, nil, CATALOGUED)
  assert_eq("  1    2    3    moves 5", rendered.lines[FIRST_BOARD_ROW + 1])
  local won = { "GamesWin", "GamesWinInk" }
  assert_eq({ won, won }, helpers.hls_on_row(rendered, FIRST_BOARD_ROW + 2))
  assert_eq("]g next game  q close", rendered.lines[#rendered.lines])
end

function T.direction_keys_are_bound_only_where_a_tile_can_move_that_way()
  local function footer(state, catalogued)
    local rendered =
      shell.render(state, fifteen, view_options("ascii"), "alice", 0, nil, catalogued)
    return rendered.lines[#rendered.lines]
  end
  assert_eq(
    "R give up  m slide tile, e.g. 7  h left  j down  l right  ]g next game  q close",
    footer(update(), CATALOGUED)
  )
  assert_eq(
    "R give up  m slide tile, e.g. 7  ]g next game  q close",
    footer(update(), { slide = true })
  )
end

function T.keys_send_the_slide_by_number_and_the_move_by_direction()
  assert_eq({ kind = "slide", tile = "7" }, fifteen.input.parse("7"))
  local sent = {}
  for _, entry in ipairs(fifteen.actions) do
    sent[entry.key] = entry.action
  end
  assert_eq({
    h = { kind = "move", direction = "left" },
    j = { kind = "move", direction = "down" },
    k = { kind = "move", direction = "up" },
    l = { kind = "move", direction = "right" },
  }, sent)
end

function T.a_roomy_window_draws_the_tiles_in_block_digits_with_movable_ones_underlined()
  local rendered = shell.render(update(), fifteen, view_options("unicode"), "alice", 0, {
    text_area = { w = 120, h = 40 },
  }, CATALOGUED)
  local drawn = table.concat(rendered.lines, "\n")
  assert(drawn:find("▄█", 1, true) and drawn:find("▀▀▀▀▀▀▀", 1, true), drawn)
end

function T.describe_event_names_the_tile_and_its_way()
  assert_eq(
    "alice slid 8 left",
    fifteen.describe_event({ kind = "slid", by = "solver", tile = 8, direction = "left" }, update())
  )
  assert_eq(nil, fifteen.describe_event({ kind = "other", by = "solver" }, update()))
end

return T
