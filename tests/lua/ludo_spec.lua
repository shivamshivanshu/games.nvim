local helpers = require("helpers")
local ludo = require("games.games.ludo")

local assert_eq = helpers.assert_eq

local BASE = { kind = "base" }
local HOME = { kind = "home" }

---@param square integer
---@return games.LudoLocation
local function track(square)
  return { kind = "track", square = square }
end

---@param step integer
---@return games.LudoLocation
local function lane(step)
  return { kind = "home_column", step = step }
end

---Three players mid-game, alice (blue) to move after a six, with view overrides on top.
---@param view_overrides? table
---@return games.LudoUpdate
local function ludo_update(view_overrides)
  return helpers.game_update("ludo", {
    seq = 3,
    players = {
      { user = "alice", seat = "blue" },
      { user = "bob", seat = "yellow" },
      { user = "carol", seat = "red" },
    },
    to_act = { "alice" },
    controls = { "resign" },
    events = { { kind = "rolled", by = "blue", value = 6 } },
    view = {
      starts = { blue = 0, red = 13, yellow = 26 },
      progress = { blue = { -1, 10, 52, 56 }, yellow = { 0, 0, -1, 40 }, red = { -1, -1, -1, 21 } },
      safe = { 0, 8, 13, 21, 26, 34, 39, 47 },
      locations = {
        blue = { BASE, track(10), lane(2), HOME },
        yellow = { track(26), track(26), BASE, track(14) },
        red = { BASE, BASE, BASE, track(34) },
      },
      last_roll = 6,
      sixes_in_row = 1,
      phase = "move",
      movable = { 1, 2 },
      can_roll = false,
    },
  }, { view = view_overrides })
end

---@param rendered games.Rendered
---@param first string the board's first line
---@return integer index of first in rendered.lines
local function line_of(rendered, first)
  for i, line in ipairs(rendered.lines) do
    if line == first then
      return i
    end
  end
  error("board not found: " .. first)
end

local T = {}

function T.tokens_stand_on_their_seats_squares_counted_when_stacked()
  local rendered = helpers.render_game("ludo", ludo_update())
  local board = {
    "                  B  .  .                      roll  6         ",
    "   B        o     .  :: *     R        R       sixes 1         ",
    "                  *  :: Y                      phase move      ",
    "                  .  :: .                      1     base <    ",
    "   o        o     .  :: .     o        R       2     track 10 <",
    "                  .  :: .                      3     lane 2    ",
    ".  *  .  .  .  .     ^     .  .  .  *  .  .    4     home      ",
    ".  :: B  :: :: :: B     ^  :: :: :: :: :: .                    ",
    ".  .  *  .  .  .     ^     .  .  .  .  Y2 .                    ",
    "                  .  :: .                                      ",
    "   o        o     .  :: .     o        Y                       ",
    "                  .  :: .                                      ",
    "                  .  :: R                                      ",
    "   o        o     *  :: .     o        o                       ",
    "                  .  .  .                                      ",
  }
  local first = line_of(rendered, board[1])
  assert_eq(board, vim.list_slice(rendered.lines, first, first + #board - 1))
end

---The highlights layered on the cell at display line and column.
---@param rendered games.Rendered
---@param line integer
---@param col integer
---@return games.Hl[]
local function hl_at(rendered, line, col)
  local found = {}
  for _, mark in ipairs(helpers.extmarks(rendered)) do
    if mark.row == line and mark.col == col then
      vim.list_extend(found, type(mark.hl) == "table" and mark.hl or { mark.hl })
    end
  end
  return found
end

---The 0-based line of the board's top row, which holds blue's token 2.
---@param rendered games.Rendered
---@return integer
local function blue_token_line(rendered)
  for i, line in ipairs(rendered.lines) do
    if vim.startswith(line, "                  B  .  .                      roll  6") then
      return i - 1
    end
  end
  error("board not found")
end

function T.a_token_takes_its_seats_colour_and_the_last_move_is_marked_from_a_snapshot()
  local update = ludo_update({
    last = { seat = "blue", token = 2, to = track(10) },
    phase = "roll",
    movable = {},
    can_roll = true,
  })
  update.events = {}
  local rendered = helpers.render_game("ludo", update)
  local row = blue_token_line(rendered)
  local blue_token, red_token = hl_at(rendered, row, 18), hl_at(rendered, row + 1, 30)
  assert(vim.tbl_contains(blue_token, "GamesSeat1"), vim.inspect(blue_token))
  assert(vim.tbl_contains(blue_token, "GamesLastMove"), vim.inspect(blue_token))
  assert(vim.tbl_contains(red_token, "GamesSeat3"), vim.inspect(red_token))
end

function T.no_square_is_marked_without_a_last_move()
  local rendered = helpers.render_game("ludo", ludo_update())
  local blue_token = hl_at(rendered, blue_token_line(rendered), 18)
  assert(not vim.tbl_contains(blue_token, "GamesLastMove"), vim.inspect(blue_token))
end

function T.keys_follow_what_the_view_offers_the_viewer()
  local function game_keys(update, me)
    local _, _, bindings = helpers.render_game("ludo", update, { me = me })
    return vim
      .iter(bindings)
      :map(function(binding)
        return binding.key
      end)
      :filter(function(key)
        return key:match("^[r1-4]$") ~= nil
      end)
      :totable()
  end
  assert_eq({ "1", "2" }, game_keys(ludo_update(), "alice"))
  assert_eq(
    { "r" },
    game_keys(ludo_update({ phase = "roll", movable = {}, can_roll = true }), "alice")
  )
  assert_eq({}, game_keys(ludo_update({ movable = {} }), "bob"))
end

function T.move_keys_send_the_token_number()
  local move_two = vim.iter(ludo.actions):find(function(entry)
    return entry.key == "2"
  end)
  assert_eq({ kind = "move", token = "2" }, move_two.action)
end

function T.events_read_as_sentences()
  local update = ludo_update()
  for _, case in ipairs({
    { { kind = "rolled", by = "yellow", value = 6 }, "bob rolled 6" },
    { { kind = "passed", by = "red" }, "carol cannot move" },
    { { kind = "forfeited", by = "blue" }, "alice rolled a third six and loses the turn" },
    {
      { kind = "moved", by = "blue", token = 1, from = BASE, to = track(0) },
      "alice brought token 1 out",
    },
    {
      { kind = "moved", by = "blue", token = 3, from = lane(2), to = HOME },
      "alice brought token 3 home",
    },
    {
      { kind = "moved", by = "red", token = 4, from = track(30), to = track(34) },
      "carol moved token 4 to square 34",
    },
    {
      { kind = "moved", by = "red", token = 2, from = track(10), to = lane(1) },
      "carol moved token 2 to lane 1",
    },
    {
      { kind = "captured", by = "red", victim = "yellow", token = 2 },
      "carol sent bob's token 2 to base",
    },
  }) do
    assert_eq(case[2], helpers.described(update, case[1]))
  end
end

---The board's 15 rows, without the panel beside it.
---@param rendered games.Rendered
---@param first string the board's first row, as drawn
---@return string[]
local function board_rows(rendered, first)
  local row = vim.iter(ipairs(rendered.lines)):find(function(_, line)
    return vim.startswith(line, first)
  end)
  return vim.tbl_map(function(line)
    return vim.fn.strcharpart(line, 0, 44)
  end, vim.list_slice(rendered.lines, row, row + 14))
end

function T.several_colours_on_a_safe_square_show_their_count()
  local update = ludo_update()
  update.view.locations.yellow = { track(8), track(8), BASE, track(14) }
  local rendered = helpers.render_game("ludo", update)
  update.view.locations.blue = { track(8), track(10), lane(2), HOME }
  local mixed = helpers.render_game("ludo", update)
  assert_eq("                  Y2 :: Y", board_rows(rendered, "                  B")[3]:sub(1, 25))
  assert_eq("                  x3 :: Y", board_rows(mixed, "                  B")[3]:sub(1, 25))
end

function T.the_unicode_pack_draws_each_seat_by_shape()
  local rendered = helpers.render_game("ludo", ludo_update(), { glyphs = "unicode" })
  local rows = board_rows(rendered, "                  ●")
  assert_eq("   ●        ○     ·  ░░ ✶     ▲        ▲    ", rows[2])
  assert_eq(
    "·  ░░ ●  ░░ ░░ ░░ ●     ⌂  ░░ ░░ ░░ ░░ ░░ · ",
    rows[8]
  )
  assert(
    rendered.lines[vim.iter(ipairs(rendered.lines)):find(function(_, line)
      return line:find("track 10", 1, true) ~= nil
    end)]:find("track 10 ←", 1, true),
    "movable mark from the unicode pack"
  )
end

function T.a_resigned_seat_leaves_its_corner_empty_and_uncoloured()
  local update = ludo_update()
  for _, field in ipairs({ "starts", "progress", "locations" }) do
    update.view[field].red = nil
  end
  local rendered = helpers.render_game("ludo", update)
  local rows = board_rows(rendered, "                  B")
  assert_eq("   B        o     .  :: *     o        o    ", rows[2])
  assert_eq("                  .  :: *                   ", rows[13])
  local row = vim.iter(ipairs(rendered.lines)):find(function(_, line)
    return vim.startswith(line, "   B        o     .  :: *     o")
  end) - 1
  local coloured = vim.tbl_filter(function(mark)
    local hl = type(mark.hl) == "table" and mark.hl or { mark.hl }
    return mark.row == row and mark.col == 30 and vim.tbl_contains(hl, "GamesSeat3")
  end, helpers.extmarks(rendered))
  assert_eq({}, coloured)
end

return T
