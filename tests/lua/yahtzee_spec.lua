local describe = require("games.core.describe")
local helpers = require("helpers")
local shell = require("games.ui.shell")
local yahtzee = require("games.games.yahtzee")

local assert_eq = helpers.assert_eq

local CATALOGUE = { roll = true, hold = true, score = true }
local DICE_COUNT = 5
---Lines above the dice at cell height 1: two players, the status row and a blank.
local DICE_TOP = 5

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("yahtzee", yahtzee, glyphs, extra)
end

---@param seat games.Seat
---@param overrides? table
---@return games.YahtzeeCard
local function card(seat, overrides)
  return vim.tbl_extend("force", {
    seat = seat,
    scores = {},
    upper_total = 0,
    upper_bonus = 0,
    yahtzee_bonus = 0,
    total = 0,
    out = false,
  }, overrides or {})
end

---alice (p1) to act after one roll, holding three threes, against bob (p2).
---@param view? table overrides of the view
---@param overrides? table overrides of the update
---@return games.YahtzeeUpdate
local function rolled(view, overrides)
  return helpers.game_update("yahtzee", {
    seq = 3,
    players = { { user = "alice", seat = "p1" }, { user = "bob", seat = "p2" } },
    to_act = { "alice" },
    controls = { "resign" },
    view = {
      round = 2,
      dice = { 3, 3, 5, 1, 3 },
      held = { true, true, false, false, true },
      rolls_left = 1,
      cards = {
        card("p1", { scores = { chance = 17 }, total = 17 }),
        card("p2", { scores = { sixes = 18 }, upper_total = 18, total = 18 }),
      },
      can_roll = true,
      can_hold = true,
      options = {
        { box = "ones", score = 1 },
        { box = "threes", score = 9 },
        { box = "three_of_a_kind", score = 15 },
      },
    },
  }, vim.tbl_extend("force", { view = view }, overrides or {}))
end

---@param update games.YahtzeeUpdate
---@param me games.UserID
---@return string[]
local function render(update, me)
  return shell.render(update, yahtzee, view_options("ascii"), me, 0, nil, CATALOGUE).lines
end

---The scorecard line whose label is label, without the dice beside it.
---@param lines string[]
---@param label string
---@return string
local function scorecard_line(lines, label)
  local line = vim.iter(lines):find(function(l)
    return l:find(" " .. label .. " ", 1, true) ~= nil
  end)
  assert(line, "no line for " .. label)
  return vim.trim(line:sub(14))
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

function T.held_dice_carry_a_mark_under_them()
  local lines = render(rolled(), "alice")
  assert_eq(" 3 3 5 1 3", lines[DICE_TOP]:sub(1, 10))
  assert_eq(" ^ ^     ^", lines[DICE_TOP + 1]:sub(1, 10))
  assert_eq(" 1 2 3 4 5", lines[DICE_TOP + 2]:sub(1, 10))
end

function T.before_the_first_roll_the_dice_are_empty_places()
  local view = { dice = {}, held = { false, false, false, false, false }, rolls_left = 3 }
  local lines =
    render(rolled(vim.tbl_extend("force", view, { can_hold = false, options = {} })), "alice")
  assert_eq(" . . . . .", lines[DICE_TOP]:sub(1, 10))
  assert_eq("R resign  r roll  ]g next game  q close", lines[#lines])
end

function T.only_the_offered_boxes_show_their_key_and_what_they_would_score()
  local lines = render(rolled(), "alice")
  assert_eq("a Ones                  -      -    1", scorecard_line(lines, "Ones"))
  assert_eq("Twos                  -      -", scorecard_line(lines, "Twos"))
  assert_eq("c Threes                -      -    9", scorecard_line(lines, "Threes"))
  assert_eq("Sixes                 -     18", scorecard_line(lines, "Sixes"))
  assert_eq("Chance               17      -", scorecard_line(lines, "Chance"))
  assert_eq("Total                17     18", scorecard_line(lines, "Total"))
  assert_eq("R resign  r roll  1-5 hold  acg score  ]g next game  q close", lines[#lines])
end

function T.the_waiting_seat_sees_the_cards_but_no_keys()
  local view = { can_roll = false, can_hold = false, options = {} }
  local lines = render(rolled(view), "bob")
  assert_eq("Ones                  -      -", scorecard_line(lines, "Ones"))
  assert_eq("R resign  ]g next game  q close", lines[#lines])
end

function T.each_key_sends_its_action()
  local sent = vim.iter(yahtzee.actions):fold({}, function(acc, entry)
    acc[entry.key] = entry.action
    return acc
  end)
  assert_eq({ kind = "roll" }, sent.r)
  assert_eq({ kind = "hold", die = "4" }, sent["4"])
  assert_eq({ kind = "score", box = "ones" }, sent.a)
  assert_eq({ kind = "score", box = "sixes" }, sent.f)
  assert_eq({ kind = "score", box = "yahtzee" }, sent.l)
  assert_eq({ kind = "score", box = "chance" }, sent.n)
  assert_eq(nil, sent.m)
end

function T.each_header_line_shows_the_seats_total_and_whether_it_left()
  local cards = { card("p1", { total = 17 }), card("p2", { total = 18, out = true }) }
  local lines = render(rolled({ cards = cards }), "alice")
  assert(lines[1]:find("alice  p1  total 17", 1, true), lines[1])
  assert(lines[2]:find("bob    p2  total 18 out", 1, true), lines[2])
end

function T.events_name_the_dice_and_the_box()
  local scored = { kind = "scored", by = "p2", box = "full_house", score = 25, yahtzee_bonus = 0 }
  local update = rolled()
  update.events = {
    { kind = "rolled", by = "p1", dice = { 3, 3, 5, 1, 3 } },
    scored,
    vim.tbl_extend("force", scored, { box = "fours", score = 20, yahtzee_bonus = 100 }),
  }
  assert_eq(
    table.concat({
      "alice rolled ⚂ ⚂ ⚄ ⚀ ⚂",
      "bob scored 25 in full house",
      "bob scored 20 in fours and a 100 Yahtzee bonus",
    }, "; "),
    describe.events(update)
  )
end

function T.unicode_dice_draw_their_faces()
  local lines =
    shell.render(rolled(), yahtzee, view_options("unicode"), "alice", 0, nil, CATALOGUE).lines
  assert(lines[DICE_TOP]:find("⚂ ⚂ ⚄ ⚀ ⚂", 1, true), lines[DICE_TOP])
  assert(lines[DICE_TOP + 1]:find("▲ ▲     ▲", 1, true), lines[DICE_TOP + 1])
end

function T.a_finished_game_shows_the_round_without_rolls()
  local finished =
    { result = { kind = "decisive", winners = { "alice" }, reason = "scorecards_full" } }
  local lines = render(rolled({ round = 13, rolls_left = 3 }, finished), "alice")
  assert(shows(lines, "Round 13 of 13"), table.concat(lines, "\n"))
  assert(not shows(lines, "rolls left"), table.concat(lines, "\n"))
end

function T.on_a_big_window_each_die_is_outlined_apart_from_the_next()
  local display = { text_area = { w = 120, h = 40 }, seats = 2 }
  local lines =
    shell.render(rolled(), yahtzee, view_options("unicode"), "alice", 0, display, CATALOGUE).lines
  local tops = vim.iter(lines):find(function(l)
    return l:find("▁▁▁▁▁▁", 1, true) ~= nil
  end)
  assert(tops, table.concat(lines, "\n"))
  assert_eq(DICE_COUNT, select(2, tops:gsub(" ▁▁▁▁▁▁ ", "")))
  assert(shows(lines, "▕  ▗▖  ▏"), table.concat(lines, "\n"))
end

return T
