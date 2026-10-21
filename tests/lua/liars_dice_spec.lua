local describe = require("games.core.describe")
local helpers = require("helpers")
local liars_dice = require("games.games.liars_dice")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUE = helpers.catalogued("liars_dice")

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("liars_dice", liars_dice, glyphs, extra)
end

---alice (p1) to raise bob's bid in round 2; carol is out and round 1 was revealed.
---@param view? table overrides of the view
---@return games.LiarsDiceUpdate
local function bidding(view)
  return helpers.game_update("liars_dice", {
    seq = 3,
    players = {
      { user = "alice", seat = "p1" },
      { user = "bob", seat = "p2" },
      { user = "carol", seat = "p3" },
    },
    to_act = { "alice" },
    controls = { "resign" },
    view = {
      round = 2,
      seats = {
        { seat = "p1", dice = 4, out = false },
        { seat = "p2", dice = 5, out = false },
        { seat = "p3", dice = 0, out = true },
      },
      bid = { by = "p2", quantity = 3, face = 4 },
      total_dice = 9,
      my_dice = { 1, 2, 4, 6 },
      min_bids = {
        { face = 1, quantity = 2 },
        { face = 2, quantity = 4 },
        { face = 3, quantity = 4 },
        { face = 4, quantity = 4 },
        { face = 5, quantity = 3 },
        { face = 6, quantity = 3 },
      },
      can_dudo = true,
      last_reveal = {
        round = 1,
        bid = { by = "p1", quantity = 4, face = 5 },
        challenger = "p2",
        counted = 3,
        loser = "p1",
        cups = {
          { seat = "p1", dice = { 1, 3, 5, 5, 6 } },
          { seat = "p2", dice = { 2, 2, 3, 4, 6 } },
        },
      },
    },
  }, { view = view })
end

---@param update games.LiarsDiceUpdate
---@param me games.UserID
---@param glyphs? games.Glyphs
---@return string[]
local function render(update, me, glyphs)
  return shell.render(update, liars_dice, view_options(glyphs or "ascii"), me, 0, nil, CATALOGUE).lines
end

---@param lines string[]
---@param text string
---@return boolean
local function shows(lines, text)
  return vim.iter(lines):any(function(l)
    return l:find(text, 1, true) ~= nil
  end)
end

local T = {}

function T.a_seat_sees_its_own_dice_with_empty_places_for_those_it_lost()
  local lines = render(bidding(), "alice")
  assert(shows(lines, "1 2 4 6 ."), table.concat(lines, "\n"))
end

function T.the_table_shows_the_standing_bid_and_the_least_raise_on_each_face()
  local lines = render(bidding(), "alice")
  assert(shows(lines, "Round 2, 9 dice in play"))
  assert(shows(lines, "Bid   3 x 4 by bob"))
  assert(shows(lines, "Raise 2x1  4x2  4x3  4x4  3x5  3x6"))
end

function T.before_any_bid_the_seat_to_act_opens()
  local update = bidding({
    min_bids = { { face = 2, quantity = 1 }, { face = 6, quantity = 1 } },
    can_dudo = false,
  })
  update.view.bid = nil
  local lines = render(update, "alice")
  assert(shows(lines, "Bid   none yet"))
  assert(shows(lines, "Open  1x2  1x6"))
  assert_eq("R resign  m bid, e.g. 3x5  ]g next game  q close", lines[#lines])
end

function T.the_last_reveal_shows_every_cup()
  local lines = render(bidding(), "bob")
  assert(shows(lines, "Round 1: bob called dudo on 4 x 5"))
  assert(shows(lines, "3 counted, alice lost a die"))
  assert(shows(lines, "alice      1 3 5 5 6"))
  assert(shows(lines, "bob        2 2 3 4 6"))
end

function T.a_waiting_seat_has_no_raise_line_and_cannot_call_dudo()
  local lines = render(bidding({ min_bids = {}, can_dudo = false }), "bob")
  assert(not shows(lines, "Raise"))
  assert_eq("R resign  ]g next game  q close", lines[#lines])
end

function T.the_seat_to_act_may_bid_or_call_dudo()
  local lines = render(bidding(), "alice")
  assert_eq("R resign  m bid, e.g. 3x5  d dudo  ]g next game  q close", lines[#lines])
  local dudo = vim.iter(liars_dice.actions):find(function(entry)
    return entry.key == "d"
  end)
  assert_eq({ kind = "dudo" }, dudo.action)
end

function T.each_header_line_shows_the_seats_dice_or_that_it_is_out()
  local lines = render(bidding(), "alice")
  assert(lines[1]:find("alice  p1  4 dice", 1, true), lines[1])
  assert(lines[3]:find("carol  p3  out", 1, true), lines[3])
end

function T.one_die_is_not_dice()
  local view = {
    seats = { { seat = "p1", dice = 1, out = false }, { seat = "p2", dice = 0, out = true } },
    total_dice = 1,
    my_dice = { 4 },
  }
  local lines = render(bidding(view), "alice")
  assert(lines[1]:find("alice  p1  1 die", 1, true), lines[1])
  assert(shows(lines, "Round 2, 1 die in play"), table.concat(lines, "\n"))
end

function T.events_tell_the_bid_the_challenge_and_who_is_out()
  local update = bidding()
  update.events = {
    { kind = "bid", by = "p1", quantity = 3, face = 5 },
    {
      kind = "challenged",
      by = "p2",
      bid = { by = "p1", quantity = 3, face = 5 },
      counted = 2,
      loser = "p1",
      cups = {},
    },
    { kind = "out", seat = "p3" },
  }
  assert_eq(
    table.concat({
      "alice bids 3 x ⚄",
      "bob calls dudo on 3 x ⚄: 2 counted, alice loses a die",
      "carol has no dice left",
    }, "; "),
    describe.events(update)
  )
end

function T.unicode_dice_draw_their_faces()
  local lines = render(bidding(), "alice", "unicode")
  assert(shows(lines, "⚀ ⚁ ⚃ ⚅ ·"), table.concat(lines, "\n"))
end

return T
