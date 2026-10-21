local describe = require("games.core.describe")
local g2048 = require("games.games.g2048")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUED = helpers.catalogued("g2048")
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
      goal = 2048,
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
      goal = 2048,
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

function T.direction_keys_are_bound_only_for_the_movable_directions()
  assert_eq(
    "R give up  h left  j down  l right  ]g next game  ? help  q close",
    footer(render(update_with()))
  )
  assert_eq("]g next game  ? help  q close", footer(render(lost())))
  assert_eq(
    "R give up  ]g next game  ? help  q close",
    footer((shell.render(update_with(), g2048, view_options("ascii"), "alice", 0, nil, {})))
  )
end

function T.every_direction_is_bound_when_every_direction_moves()
  local open = update_with()
  open.view = vim.tbl_extend("force", open.view, { movable = { "up", "down", "left", "right" } })
  assert_eq(
    "R give up  h left  j down  k up  l right  ]g next game  ? help  q close",
    footer(render(open))
  )
end

function T.each_key_and_arrow_sends_its_direction()
  local sent = {}
  for _, entry in ipairs(g2048.actions) do
    for _, key in ipairs({ entry.key, unpack(entry.aliases or {}) }) do
      sent[key] = entry.action
    end
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

function T.events_name_the_move_its_gain_and_the_goal()
  local spawned = { row = 1, col = 2, value = 2 }
  local events = {
    { kind = "moved", by = "solver", direction = "left", gained = 8, spawned = spawned },
    { kind = "moved", by = "solver", direction = "up", gained = 0, spawned = spawned },
    { kind = "reached", by = "solver", tile = 2048 },
  }
  assert_eq(
    "alice moved left +8; alice moved up; alice made 2048",
    describe.events(update_with({ events = events }))
  )
end

return T
