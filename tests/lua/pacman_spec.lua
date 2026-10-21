local Client = require("games.core.client")
local describe = require("games.core.describe")
local helpers = require("helpers")
local pacman = require("games.games.pacman")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUED = helpers.catalogued("pacman")
---Lines above the board at cell height 1: the player, the status row and a blank.
local BOARD_TOP = 3
---Display columns from one cell's start to the next: cells touch.
local CELL_STEP = 2

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("pacman", pacman, glyphs, extra)
end

---A small maze mid-game: Blinky chasing, Pinky frightened, Inky's eyes going home, and Clyde in
---the house under Pac-Man's own cell.
---@param overrides? table fields of the view to replace
---@return games.PacmanUpdate
local function update_with(overrides)
  return helpers.game_update("pacman", {
    seq = 40,
    players = { { user = "alice", seat = "player" } },
    to_act = { "alice" },
    controls = { "resign" },
    view = {
      maze = { "#######", "#.o...#", "#.##-##", "#.....#", "#######" },
      pacman = { row = 4, col = 3, facing = "left" },
      ghosts = {
        { name = "blinky", row = 2, col = 5, facing = "left", mode = "chase" },
        { name = "pinky", row = 4, col = 6, facing = "up", mode = "frightened" },
        { name = "inky", row = 2, col = 2, facing = "down", mode = "eyes" },
        { name = "clyde", row = 4, col = 3, facing = "up", mode = "house" },
      },
      score = 1230,
      lives = 2,
      level = 2,
      levels = 3,
      food_left = 7,
      fright_ticks = 12,
      paused = false,
      can_turn = true,
      can_pause = true,
    },
  }, { view = overrides })
end

---@param update games.PacmanUpdate
---@param glyphs? games.Glyphs
---@return games.Rendered
local function render(update, glyphs)
  return (
    shell.render(update, pacman, view_options(glyphs or "ascii"), "alice", 0, nil, CATALOGUED)
  )
end

---@param rendered games.Rendered
---@param r integer
---@param c integer
---@return games.Hl[] the highlight stack on cell r, c
local function cell_hl(rendered, r, c)
  return helpers.hl_stack_at(rendered, BOARD_TOP + r - 1, (c - 1) * CELL_STEP)
end

---@param rendered games.Rendered
---@return string
local function footer(rendered)
  return rendered.lines[#rendered.lines]
end

local T = {}

function T.board_draws_the_maze_pacman_and_each_ghost_beside_the_status_in_both_glyph_modes()
  assert_eq({
    "##############   score 1230   ",
    '##"Io . MB. ##   lives 2      ',
    "##. ####--####   level 2/3    ",
    "##. > . . Mp##   food  7      ",
    "##############   game  running",
  }, vim.list_slice(render(update_with()).lines, BOARD_TOP + 1, BOARD_TOP + 5))
  assert_eq(
    {
      "██████████████",
      '██"I● · ᗝB· ██',
      "██· ████──████",
      "██· ᗤ · · ᗝp██",
      "██████████████",
    },
    vim.tbl_map(function(line)
      return vim.fn.strcharpart(line, 0, 14)
    end, vim.list_slice(render(update_with(), "unicode").lines, BOARD_TOP + 1, BOARD_TOP + 5))
  )
end

function T.pacman_faces_its_way()
  for facing, glyph in pairs({ up = "V ", down = "^ ", left = "> ", right = "< " }) do
    local line =
      render(update_with({ pacman = { row = 4, col = 2, facing = facing } })).lines[BOARD_TOP + 4]
    assert_eq(glyph, line:sub(3, 4))
  end
end

function T.each_ghost_has_its_colour_and_fright_and_eyes_their_own()
  local rendered = render(update_with())
  assert(vim.tbl_contains(cell_hl(rendered, 2, 5), "GamesGhostBlinky"), "Blinky")
  assert(vim.tbl_contains(cell_hl(rendered, 4, 6), "GamesGhostFrightened"), "frightened Pinky")
  assert(vim.tbl_contains(cell_hl(rendered, 2, 2), "GamesMuted"), "Inky's eyes")
  assert(vim.tbl_contains(cell_hl(rendered, 4, 3), "GamesPacman"), "Pac-Man over Clyde")
  assert(vim.tbl_contains(cell_hl(rendered, 1, 1), "GamesPacmanWall"), "a wall")
end

function T.a_board_redrawing_from_its_kept_rows_draws_each_update_as_afresh()
  local view = view_options("unicode", { grid_rows = {} })
  for _, overrides in ipairs({
    {},
    { pacman = { row = 4, col = 2, facing = "left" } },
    { pacman = { row = 4, col = 2, facing = "up" } },
    {
      ghosts = {
        { name = "blinky", row = 2, col = 5, facing = "left", mode = "frightened" },
        { name = "pinky", row = 4, col = 6, facing = "up", mode = "frightened" },
      },
    },
    { maze = { "#######", "#     #", "#######" }, pacman = { row = 2, col = 2, facing = "up" } },
  }) do
    local update = update_with(overrides)
    local kept = shell.render(update, pacman, view, "alice", 0, nil, CATALOGUED)
    local afresh = render(update, "unicode")
    assert_eq(afresh.lines, kept.lines)
    assert_eq(helpers.extmarks(afresh), helpers.extmarks(kept))
  end
end

function T.the_status_says_paused_or_over()
  assert_eq(
    "##############   game  paused: turn to go",
    render(update_with({ paused = true, can_pause = false })).lines[BOARD_TOP + 5]
  )
  local over = update_with({ can_turn = false, can_pause = false })
  over.result = { kind = "decisive", winners = {}, reason = "game_over" }
  assert_eq("##############   game  over", render(over).lines[BOARD_TOP + 5])
end

function T.a_reconnect_takes_the_listed_game_even_behind_the_seq_the_board_shows()
  local conn = helpers.stub_conn()
  local client = Client.new(conn --[[@as games.Conn]])
  conn:push("connected", { user = "alice", heartbeat_ms = 15000 })
  conn:push("game.update", vim.tbl_extend("force", update_with({}), { seq = 10 }))
  local buf = assert(shell.open(client, "0000"))

  conn:push("disconnected", { code = "disconnected", message = "lost", retrying = true })
  conn:push("connected", { user = "alice", heartbeat_ms = 15000 })
  local restored = update_with({ paused = true, can_pause = false })
  restored.seq = 8
  conn:requests_of("game.list")[2].cb(nil, { games = { restored } })

  assert_eq(8, client:game("0000").seq)
  helpers.wait_for(function()
    local text = table.concat(vim.api.nvim_buf_get_lines(buf, 0, -1, false), "\n")
    return text:find("paused: turn to go", 1, true) ~= nil
  end, "the board to show the restored game paused")
  vim.api.nvim_buf_delete(buf, { force = true })
end

function T.turn_keys_while_the_game_goes_on_and_pause_only_while_it_runs()
  assert_eq(
    "R give up  h left  j down  k up  l right  p pause  ]g next game  ? help  q close",
    footer(render(update_with()))
  )
  assert_eq(
    "R give up  h left  j down  k up  l right  ]g next game  ? help  q close",
    footer(render(update_with({ paused = true, can_pause = false })))
  )
  local over = update_with({ can_turn = false, can_pause = false })
  over.controls, over.to_act = {}, {}
  assert_eq("]g next game  ? help  q close", footer(render(over)))
end

function T.each_key_and_arrow_sends_its_action()
  local sent = {}
  for _, entry in ipairs(pacman.actions) do
    for _, key in ipairs({ entry.key, unpack(entry.aliases or {}) }) do
      sent[key] = entry.action
    end
  end
  local function turn(direction)
    return { kind = "turn", direction = direction }
  end
  assert_eq({
    h = turn("left"),
    ["<Left>"] = turn("left"),
    j = turn("down"),
    ["<Down>"] = turn("down"),
    k = turn("up"),
    ["<Up>"] = turn("up"),
    l = turn("right"),
    ["<Right>"] = turn("right"),
    p = { kind = "pause" },
  }, sent)
end

function T.taller_cells_keep_the_walls_joined()
  local update = update_with()
  local rendered = shell.render(
    update,
    pacman,
    view_options("unicode"),
    "alice",
    0,
    { text_area = { w = 120, h = 16 } },
    CATALOGUED
  )
  for line = BOARD_TOP + 1, BOARD_TOP + 2 do
    assert_eq(("█"):rep(28), vim.fn.strcharpart(rendered.lines[line], 0, 28))
  end
end

function T.events_name_the_ghost_the_points_and_the_lives_left()
  local update = update_with()
  update.events = {
    { kind = "ghost_eaten", by = "player", ghost = "pinky", points = 400 },
    { kind = "caught", by = "player", ghost = "blinky", lives_left = 2 },
    { kind = "caught", by = "player", ghost = "clyde", lives_left = 1 },
    { kind = "level_cleared", by = "player", level = 1 },
  }
  assert_eq(
    table.concat({
      "alice ate pinky +400",
      "blinky caught alice, 2 lives left",
      "clyde caught alice, 1 life left",
      "alice cleared level 1",
    }, "; "),
    describe.events(update)
  )
end

return T
