local assets = require("games.ui.assets")
local helpers = require("helpers")
local ludo = require("games.games.ludo")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUE = { roll = true, move = true }

---@param glyphs games.Glyphs
---@return games.ViewOptions
local function ludo_view(glyphs)
  local spec = { keys = ludo.asset_keys, glyph_width = ludo.glyph_width }
  local pack = assets.load("ludo", assets.DEFAULT_PACK, spec)
  return { glyphs = glyphs, flipped = false, art = assets.art_set(pack, glyphs) }
end

---Three players mid-game, alice (blue) to move after a six, with view overrides on top.
---@param view_overrides? table
---@return games.LudoUpdate
local function ludo_update(view_overrides)
  return {
    game_id = "0000",
    game = "ludo",
    seq = 3,
    options = {},
    players = {
      { user = "alice", seat = "blue" },
      { user = "bob", seat = "yellow" },
      { user = "carol", seat = "red" },
    },
    to_act = { "alice" },
    controls = { "resign" },
    events = {
      { kind = "rolled", by = "blue", value = 4 },
      { kind = "moved", by = "blue", token = 2, from = 6, to = 10 },
    },
    view = vim.tbl_extend("force", {
      starts = { blue = 0, red = 13, yellow = 26 },
      progress = { blue = { -1, 10, 52, 56 }, yellow = { 0, 0, -1, 40 }, red = { -1, -1, -1, 21 } },
      last_roll = 6,
      sixes_in_row = 1,
      phase = "move",
      movable = { 1, 2 },
      can_roll = false,
    }, view_overrides or {}),
  }
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
  local rendered = shell.render(ludo_update(), ludo, ludo_view("ascii"), "alice", 0, nil, CATALOGUE)
  local board = {
    "                  B  .  .                      roll  6      ",
    "   B        o     .  :: *     R        R       sixes 1      ",
    "                  *  :: Y                      phase move   ",
    "                  .  :: .                      1     base ← ",
    "   o        o     .  :: .     o        R       2     10/56 ←",
    "                  .  :: .                      3     52/56  ",
    ".  *  .  .  .  .     ^     .  .  .  *  .  .    4     home   ",
    ".  :: B  :: :: :: B     ^  :: :: :: :: :: .                 ",
    ".  .  *  .  .  .     ^     .  .  .  .  Y2 .                 ",
    "                  .  :: .                                   ",
    "   o        o     .  :: .     o        Y                    ",
    "                  .  :: .                                   ",
    "                  .  :: R                                   ",
    "   o        o     *  :: .     o        o                    ",
    "                  .  .  .                                   ",
  }
  local first = line_of(rendered, board[1])
  assert_eq(board, vim.list_slice(rendered.lines, first, first + #board - 1))
end

function T.a_token_takes_its_seats_colour_and_the_last_move_is_marked()
  local rendered = shell.render(ludo_update(), ludo, ludo_view("ascii"), "alice", 0, nil, CATALOGUE)
  local row = line_of(rendered, "                  B  .  .                      roll  6      ") - 1
  local function hl_at(line, col)
    local found = {}
    for _, mark in ipairs(rendered.extmarks) do
      if mark.row == line and mark.col == col then
        vim.list_extend(found, type(mark.hl) == "table" and mark.hl or { mark.hl })
      end
    end
    return found
  end
  local blue_token, red_token = hl_at(row, 18), hl_at(row + 1, 30)
  assert(vim.tbl_contains(blue_token, "GamesSeat1"), vim.inspect(blue_token))
  assert(vim.tbl_contains(blue_token, "GamesLastMove"), vim.inspect(blue_token))
  assert(vim.tbl_contains(red_token, "GamesSeat3"), vim.inspect(red_token))
end

function T.keys_follow_what_the_view_offers_the_viewer()
  local function game_keys(update, me)
    local _, _, bindings = shell.render(update, ludo, ludo_view("ascii"), me, 0, nil, CATALOGUE)
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
    { { kind = "moved", by = "blue", token = 1, from = -1, to = 0 }, "alice brought token 1 out" },
    {
      { kind = "moved", by = "blue", token = 3, from = 52, to = 56 },
      "alice brought token 3 home",
    },
    {
      { kind = "moved", by = "red", token = 4, from = 17, to = 21 },
      "carol moved token 4 to 21/56",
    },
    {
      { kind = "captured", by = "red", victim = "yellow", token = 2 },
      "carol sent bob's token 2 to base",
    },
  }) do
    assert_eq(case[2], ludo.describe_event(case[1], update))
  end
end

return T
