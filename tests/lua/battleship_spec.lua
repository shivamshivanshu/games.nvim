local battleship = require("games.games.battleship")
local helpers = require("helpers")
local shell = require("games.ui.shell")
local text_util = require("games.ui.text")

local assert_eq = helpers.assert_eq

local CATALOGUED = { place_across = true, place_down = true, random = true, fire = true }
local FIRST_BOARD_ROW = 6
local TILE_CODES = { ["."] = "empty", ["#"] = "ship", o = "miss", x = "hit", ["*"] = "sunk" }
local WATER = ("."):rep(10)

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("battleship", battleship, glyphs, extra)
end

---@param rows string[] "." water, "#" ship, "o" miss, "x" hit, "*" sunk; water fills out each row
---and the missing rows
---@return games.BattleshipTile[][]
local function tiles(rows)
  local out = {}
  for r = 1, 10 do
    out[r] = vim.tbl_map(function(code)
      return TILE_CODES[code]
    end, vim.split(((rows[r] or "") .. WATER):sub(1, #WATER), ""))
  end
  return out
end

---@param states games.BattleshipShipState[]
---@return games.BattleshipShipStatus[]
local function fleet(states)
  local ships = {
    { "carrier", 5 },
    { "battleship", 4 },
    { "cruiser", 3 },
    { "submarine", 3 },
    { "destroyer", 2 },
  }
  return vim
    .iter(ipairs(ships))
    :map(function(i, ship)
      return { name = ship[1], size = ship[2], state = states[i] }
    end)
    :totable()
end

---Both seats have acted once both fleets are placed, so abort is over.
local BATTLE_CONTROLS = { "resign" }

local AFLOAT = fleet({ "afloat", "afloat", "afloat", "afloat", "afloat" })

---@param view table fields over an empty board
---@param overrides? table fields over the update
---@return games.BattleshipUpdate
local function update(view, overrides)
  return vim.tbl_extend("force", {
    game_id = "0000",
    game = "battleship",
    seq = 3,
    options = { first = "me" },
    players = { { user = "alice", seat = "p1" }, { user = "bob", seat = "p2" } },
    to_act = { "alice", "bob" },
    controls = { "resign", "abort" },
    events = {},
    view = vim.tbl_extend("force", {
      phase = "setup",
      own = tiles({}),
      target = tiles({}),
      fleet = AFLOAT,
      enemy = AFLOAT,
      anchors = { across = {}, down = {} },
      targets = {},
    }, view),
  }, overrides or {})
end

---alice has placed her carrier on a1-e1 and places the battleship; a2 fits it either way.
---@return games.BattleshipUpdate
local function placing()
  return update({
    own = tiles({ "#####....." }),
    fleet = fleet({ "afloat", "unplaced", "unplaced", "unplaced", "unplaced" }),
    next = { name = "battleship", size = 4 },
    anchors = { across = { { row = 2, col = 1 } }, down = { { row = 2, col = 1 } } },
  })
end

---Mid battle, alice to fire: bob sank her destroyer and missed j1; she hit his i1, missed j10 and
---last sank his destroyer on i1-i2.
---@return games.BattleshipUpdate
local function firing()
  local target = tiles({ "........*.", "........*.", "", "", "", "", "", "", "", ".........o" })
  local targets = {}
  for r = 1, 10 do
    for c = 1, 10 do
      if target[r][c] == "empty" then
        targets[#targets + 1] = { row = r, col = c }
      end
    end
  end
  return update({
    phase = "battle",
    own = tiles({ "#####....o", "", "####", "", "###", "", "###", "", "**" }),
    target = target,
    fleet = fleet({ "afloat", "afloat", "afloat", "afloat", "sunk" }),
    enemy = fleet({ "afloat", "afloat", "afloat", "afloat", "sunk" }),
    targets = targets,
    last = { by = "p1", cell = { row = 2, col = 9 }, outcome = "sunk", ship = "destroyer" },
  }, { to_act = { "alice" }, controls = BATTLE_CONTROLS })
end

---@param rendered games.Rendered
---@param from integer first line
---@param count integer
---@return string[] those lines without trailing space
local function lines_of(rendered, from, count)
  return vim.tbl_map(function(line)
    return (line:gsub("%s+$", ""))
  end, vim.list_slice(rendered.lines, from, from + count - 1))
end

---@param state games.BattleshipUpdate
---@param view games.ViewOptions
---@param catalogued? table<string, true>
---@return games.Rendered
---@return games.KeyBinding[]
local function render(state, view, catalogued)
  local rendered, _, bindings = shell.render(state, battleship, view, "alice", 0, nil, catalogued)
  return rendered, bindings
end

local T = {}

function T.placing_shows_the_next_ship_where_it_fits_or_where_it_is_blocked()
  for _, case in ipairs({
    {
      glyphs = "unicode",
      at = { row = 2, col = 1, orientation = "across" },
      board = {
        "      your fleet                 target           place battleship 4",
        "1  ■ ■ ■ ■ ■ · · · · ·   1  · · · · · · · · · ·   across at a2",
        "2  □ □ □ □ · · · · · ·   2  · · · · · · · · · ·   ship         yours    theirs",
      },
    },
    {
      glyphs = "ascii",
      at = { row = 1, col = 8, orientation = "across" },
      board = {
        "      your fleet                 target           place battleship 4",
        "1  # # # # # . . ! ! !   1  . . . . . . . . . .   across at h1: blocked",
        "2  . . . . . . . . . .   2  . . . . . . . . . .   ship         yours    theirs",
      },
    },
    {
      glyphs = "ascii",
      at = { row = 2, col = 1, orientation = "down" },
      board = {
        "      your fleet                 target           place battleship 4",
        "1  # # # # # . . . . .   1  . . . . . . . . . .   down at a2",
        "2  + . . . . . . . . .   2  . . . . . . . . . .   ship         yours    theirs",
      },
    },
  }) do
    local rendered = render(placing(), view_options(case.glyphs, { cursor = case.at }), CATALOGUED)
    assert_eq(case.board, lines_of(rendered, FIRST_BOARD_ROW - 1, 3))
  end
  local rendered = render(
    placing(),
    view_options("ascii", { cursor = { row = 1, col = 8, orientation = "down" } })
  )
  assert_eq("GamesCheck", helpers.hl_at(rendered, FIRST_BOARD_ROW - 1, 3 + 7 * 2)[1])
  assert_eq(
    "battleship 4 unplaced afloat",
    vim.trim(lines_of(rendered, FIRST_BOARD_ROW + 3, 1)[1]:sub(51))
  )
end

function T.the_battle_marks_hits_misses_sunk_ships_the_aim_and_the_latest_shot_by_glyph()
  local at = { row = 3, col = 2, orientation = "across" }
  for glyphs, want in pairs({
    unicode = {
      "      your fleet                 target           your shot",
      "1  ■ ■ ■ ■ ■ · · · · ○   1  · · · · · · · · ⊠ ·   at b3",
      "2  · · · · · · · · · ·   2  · · · · · · · · ⊠ ·   ship         yours    theirs",
      "3  ■ ■ ■ ■ · · · · · ·   3  · ◎ · · · · · · · ·   carrier 5    afloat   afloat",
    },
    ascii = {
      "      your fleet                 target           your shot",
      "1  # # # # # . . . . o   1  . . . . . . . . * .   at b3",
      "2  . . . . . . . . . .   2  . . . . . . . . * .   ship         yours    theirs",
      "3  # # # # . . . . . .   3  . @ . . . . . . . .   carrier 5    afloat   afloat",
    },
  }) do
    local rendered = render(firing(), view_options(glyphs, { cursor = at }), CATALOGUED)
    assert_eq(want, lines_of(rendered, FIRST_BOARD_ROW - 1, 4))
    local tail = lines_of(rendered, FIRST_BOARD_ROW + 7, 2)
    assert_eq(
      { "last: your shot at i2", "sank the destroyer" },
      vim.tbl_map(function(line)
        return vim.trim(vim.fn.strcharpart(line, 50))
      end, tail)
    )
  end
  local rendered = render(firing(), view_options("ascii", { cursor = at }), CATALOGUED)
  local grid_width, grid_gap, labels_width = 22, 3, 3
  local target_col = grid_width + grid_gap + labels_width
  assert_eq(
    { "GamesMovable", "GamesMovableInk" },
    helpers.hl_at(rendered, FIRST_BOARD_ROW + 1, target_col + 2)
  )
  assert_eq(
    { "GamesLastMove", "GamesCardRed" },
    helpers.hl_at(rendered, FIRST_BOARD_ROW, target_col + 16)
  )
  assert_eq("GamesCardRed", helpers.hl_at(rendered, FIRST_BOARD_ROW + 7, 3))
end

function T.a_finished_board_shows_the_opponents_ships_and_offers_nothing()
  local over = update({
    phase = "over",
    target = tiles({ "#####", "", "*" }),
  }, {
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = { "bob" }, reason = "resign" },
  })
  local rendered = render(over, view_options("ascii"), CATALOGUED)
  assert_eq({
    "      your fleet                 target           game over",
    "1  . . . . . . . . . .   1  # # # # # . . . . .",
  }, lines_of(rendered, FIRST_BOARD_ROW - 1, 2))
  assert_eq("]g next game  q close", rendered.lines[#rendered.lines])
end

function T.keys_follow_the_phase_and_the_catalogue()
  local waiting = update({ phase = "battle" }, { to_act = { "bob" }, controls = BATTLE_CONTROLS })
  local cases = {
    {
      placing(),
      CATALOGUED,
      "R resign  A abort  h,j-l move  r turn  <CR> place  f fill  ]g next game  q close",
    },
    {
      firing(),
      CATALOGUED,
      "R resign  m fire at, e.g. c5  h,j-l move  <CR> fire  ]g next game  q close",
    },
    { waiting, CATALOGUED, "R resign  ]g next game  q close" },
    { placing(), nil, "R resign  A abort  h,j-l move  r turn  ]g next game  q close" },
  }
  for _, case in ipairs(cases) do
    local rendered = render(case[1], view_options("ascii"), case[2])
    local footer = rendered.lines[#rendered.lines]
    assert_eq(case[3], footer)
    assert(vim.fn.strdisplaywidth(footer) <= 80, footer)
  end
end

function T.enter_sends_the_cursors_cell_as_a_placement_lying_its_way_or_a_shot()
  local function sent(state, at, key)
    local _, bindings = render(state, view_options("ascii", { cursor = at }), CATALOGUED)
    local binding = vim.iter(bindings):find(function(b)
      return b.key == key
    end)
    local acted
    binding.run({
      update = state,
      client = {
        game_act = function(_, request)
          acted = request.action
        end,
      },
    })
    return acted
  end
  assert_eq(
    { kind = "place_across", cell = "c7" },
    sent(placing(), { row = 7, col = 3, orientation = "across" }, "<CR>")
  )
  assert_eq(
    { kind = "place_down", cell = "j10" },
    sent(placing(), { row = 10, col = 10, orientation = "down" }, "<CR>")
  )
  assert_eq(
    { kind = "fire", cell = "b3" },
    sent(firing(), { row = 3, col = 2, orientation = "down" }, "<CR>")
  )
  assert_eq({ kind = "random" }, sent(placing(), nil, "f"))
  assert_eq({ kind = "fire", cell = " d4" }, battleship.input.parse(" d4"))
end

function T.aiming_where_the_view_offers_no_shot_marks_it_blocked_and_enter_sends_nothing()
  local grid_width, grid_gap, labels_width = 22, 3, 3
  local target_col = grid_width + grid_gap + labels_width
  local fired = { row = 1, col = 9, orientation = "across" }
  local not_offered = firing()
  not_offered.view.targets = vim.tbl_filter(function(cell)
    return not (cell.row == 4 and cell.col == 3)
  end, not_offered.view.targets)
  for _, case in ipairs({
    { firing(), fired, "i1" },
    { not_offered, { row = 4, col = 3, orientation = "across" }, "c4" },
  }) do
    local state, at, name = case[1], case[2], case[3]
    local rendered, bindings = render(state, view_options("ascii", { cursor = at }), CATALOGUED)
    local line = rendered.lines[FIRST_BOARD_ROW + at.row - 1]
    assert_eq("!", line:sub(target_col + 1 + 2 * (at.col - 1), target_col + 1 + 2 * (at.col - 1)))
    assert_eq(
      "GamesCheck",
      helpers.hl_at(rendered, FIRST_BOARD_ROW + at.row - 2, target_col + 2 * (at.col - 1))[1]
    )
    assert_eq(
      "at " .. name .. ": fired",
      vim.trim(lines_of(rendered, FIRST_BOARD_ROW, 1)[1]:sub(51))
    )
    assert_eq(
      nil,
      vim.iter(bindings):find(function(b)
        return b.key == "<CR>"
      end)
    )
  end
end

function T.the_board_and_panel_fit_eighty_columns_at_one_line_a_cell()
  local rendered = render(
    placing(),
    view_options("unicode", { cursor = { row = 10, col = 10, orientation = "down" } })
  )
  local body = vim.list_slice(rendered.lines, FIRST_BOARD_ROW - 1, #rendered.lines - 2)
  assert(text_util.max_display_width(body) <= 80, table.concat(body, "\n"))
end

function T.describe_event_names_the_shooter_the_cell_and_what_it_found()
  for _, case in ipairs({
    { { kind = "ready", by = "p2" }, "bob's fleet is ready" },
    {
      { kind = "shot", by = "p1", cell = { row = 10, col = 10 }, outcome = "miss" },
      "alice fired at j10: miss",
    },
    {
      { kind = "shot", by = "p2", cell = { row = 1, col = 3 }, outcome = "hit" },
      "bob fired at c1: hit",
    },
    {
      {
        kind = "shot",
        by = "p2",
        cell = { row = 9, col = 2 },
        outcome = "sunk",
        ship = "destroyer",
      },
      "bob fired at b9: sank the destroyer",
    },
    { { kind = "other", by = "p1" }, nil },
  }) do
    assert_eq(case[2], battleship.describe_event(case[1], placing()))
  end
end

return T
