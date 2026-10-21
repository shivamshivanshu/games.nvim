local g2048 = require("games.games.g2048")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUED = { move = true }
---Lines above the board at cell height 1: the player, the status row and a blank.
local BOARD_TOP = 3
---Display columns from one cell's start to the next: the glyph and the gap.
local CELL_STEP = 7

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("g2048", g2048, glyphs, extra)
end

---alice's board after a few merges, a 2048 among them; three directions still move.
---@param overrides? table
---@return games.G2048Update
local function update_with(overrides)
  return helpers.game_update("g2048", {
    seq = 12,
    players = { { user = "alice", seat = "solver" } },
    to_act = { "alice" },
    controls = { "resign" },
    view = {
      cells = { { 2, 0, 0, 4 }, { 16, 128, 2048, 0 }, { 0, 0, 8, 131072 }, { 1024, 0, 0, 2 } },
      score = 4436,
      reached = true,
      movable = { "down", "left", "right" },
    },
  }, overrides)
end

---@return games.G2048Update stuck without a 2048: lost
local function lost()
  return update_with({
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = {}, reason = "no_moves" },
    view = {
      cells = { { 2, 4, 2, 4 }, { 4, 2, 4, 2 }, { 2, 4, 2, 4 }, { 4, 2, 4, 2 } },
      score = 60,
      reached = false,
      movable = {},
    },
  })
end

---@param update games.G2048Update
---@param glyphs? games.Glyphs
---@param display? games.Display
---@return games.Rendered
local function render(update, glyphs, display)
  return (
    shell.render(update, g2048, view_options(glyphs or "ascii"), "alice", 0, display, CATALOGUED)
  )
end

---@param rendered games.Rendered
---@param r integer
---@param c integer
---@return games.Hl[] the highlight stack on cell r, c at cell height 1
local function cell_hl(rendered, r, c)
  return helpers.hl_stack_at(rendered, BOARD_TOP + r - 1, (c - 1) * CELL_STEP)
end

---@param rendered games.Rendered
---@return string
local function footer(rendered)
  return rendered.lines[#rendered.lines]
end

local T = {}

function T.board_centres_each_tile_beside_the_score_and_the_goal_in_both_glyph_modes()
  for _, case in ipairs({ { glyphs = "unicode", empty = "·" }, { glyphs = "ascii", empty = "." } }) do
    local lines = render(update_with(), case.glyphs).lines
    local e = case.empty
    assert_eq({
      ("  2      %s      %s      4      score 4436   "):format(e, e),
      ("  16    128    2048    %s      2048  reached"):format(e),
      ("  %s      %s      8    131072                "):format(e, e),
      (" 1024    %s      %s      2                   "):format(e, e),
    }, vim.list_slice(lines, BOARD_TOP + 1, BOARD_TOP + 4))
  end
  assert_eq("  2      4      2      4      score 60     ", render(lost()).lines[BOARD_TOP + 1])
  assert_eq("  4      2      4      2      2048  not yet", render(lost()).lines[BOARD_TOP + 2])
end

function T.tiles_of_the_goal_and_above_are_tinted()
  local rendered = render(update_with())
  assert(vim.tbl_contains(cell_hl(rendered, 2, 3), "GamesWin"), "the 2048")
  assert(vim.tbl_contains(cell_hl(rendered, 3, 4), "GamesWin"), "the 131072")
  assert(not vim.tbl_contains(cell_hl(rendered, 4, 1), "GamesWin"), "the 1024 is below the goal")
end

function T.direction_keys_and_arrows_are_bound_only_for_the_movable_directions()
  assert_eq(
    "R give up  h,<Left> left  j,<Down> down  l,<Right> right  ]g next game  q close",
    footer(render(update_with()))
  )
  assert_eq("]g next game  q close", footer(render(lost())))
  assert_eq(
    "R give up  ]g next game  q close",
    footer((shell.render(update_with(), g2048, view_options("ascii"), "alice", 0, nil, {})))
  )
end

function T.footer_pairs_each_letter_with_its_arrow_when_every_direction_moves()
  local open = update_with()
  open.view = vim.tbl_extend("force", open.view, { movable = { "up", "down", "left", "right" } })
  assert_eq(
    "R give up  h,<Left> left  j,<Down> down  k,<Up> up  l,<Right> right  ]g next game  q close",
    footer(render(open))
  )
end

function T.each_key_sends_its_direction_as_text()
  local sent = {}
  for _, entry in ipairs(g2048.actions) do
    sent[entry.key] = entry.action
  end
  local function move(direction)
    return { kind = "move", direction = direction }
  end
  assert_eq({
    h = move("left"),
    ["<Left>"] = move("left"),
    j = move("down"),
    ["<Down>"] = move("down"),
    k = move("up"),
    ["<Up>"] = move("up"),
    l = move("right"),
    ["<Right>"] = move("right"),
  }, sent)
end

function T.a_roomy_window_draws_tiles_of_up_to_four_digits_in_block_digits()
  local lines = render(update_with(), "unicode", { text_area = { w = 200, h = 60 } }).lines
  local text = table.concat(lines, "\n")
  assert(text:find("▀▀▄", 1, true), "block digits for the small tiles")
  assert(text:find("131072", 1, true), "a six-digit tile keeps its glyph")
  assert(not text:find("2048    ", 1, true), "the 2048 is drawn in block digits, not its glyph")
end

function T.describe_event_names_the_move_its_gain_and_the_goal()
  local update = update_with()
  local spawned = { row = 1, col = 2, value = 2 }
  assert_eq(
    "alice moved left +8",
    g2048.describe_event(
      { kind = "moved", by = "solver", direction = "left", gained = 8, spawned = spawned },
      update
    )
  )
  assert_eq(
    "alice moved up",
    g2048.describe_event(
      { kind = "moved", by = "solver", direction = "up", gained = 0, spawned = spawned },
      update
    )
  )
  assert_eq(
    "alice made 2048",
    g2048.describe_event({ kind = "reached", by = "solver", tile = 2048 }, update)
  )
  assert_eq(nil, g2048.describe_event({ kind = "other", by = "solver" }, update))
end

return T
