local hearts = require("games.games.hearts")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUE = { play = true, pass = true }

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("hearts", hearts, glyphs, extra)
end

---bob (p2) to play the third card of a trick in hand 2, which passed right; dave left.
---@param view? table overrides of the view
---@return games.HeartsUpdate
local function playing(view)
  return helpers.game_update("hearts", {
    seq = 30,
    players = {
      { user = "alice", seat = "p1" },
      { user = "bob", seat = "p2" },
      { user = "carol", seat = "p3" },
      { user = "dave", seat = "p4" },
    },
    to_act = { "bob" },
    controls = { "resign" },
    view = {
      hand_no = 2,
      pass = "right",
      phase = "play",
      score_limit = 100,
      seats = {
        { seat = "p1", score = 13, taken = 0, pass_chosen = true, out = false },
        { seat = "p2", score = 0, taken = 2, pass_chosen = true, out = false },
        { seat = "p3", score = 8, taken = 0, pass_chosen = true, out = false },
        { seat = "p4", score = 5, taken = 13, pass_chosen = true, out = true },
      },
      trick = { { seat = "p4", card = "Kh" }, { seat = "p1", card = "2h" } },
      last_trick = {
        plays = {
          { seat = "p3", card = "Ac" },
          { seat = "p4", card = "Qs" },
          { seat = "p1", card = "3c" },
          { seat = "p2", card = "9c" },
        },
        winner = "p3",
        points = 13,
      },
      hearts_broken = true,
      my_hand = { "2c", "7c", "Td", "Qd", "4s", "Ks", "3h", "8h", "Ah" },
      legal = { "3h", "8h", "Ah" },
      must_pass = 0,
      pass_to = "p1",
      passed = { "As", "Kd", "Qh" },
      received = { "2c", "7c", "4s" },
      last_hand = {
        hand_no = 1,
        points = {
          { seat = "p1", points = 13 },
          { seat = "p2", points = 0 },
          { seat = "p3", points = 8 },
          { seat = "p4", points = 5 },
        },
      },
    },
  }, { view = view })
end

---The start of hand 3, which passes across: bob has still to choose, alice has chosen.
---@return games.HeartsUpdate
local function passing()
  local update = playing({
    hand_no = 3,
    pass = "across",
    phase = "pass",
    trick = {},
    legal = {},
    must_pass = 3,
    pass_to = "p4",
    passed = {},
    received = {},
  })
  update.to_act = { "bob", "carol" }
  for _, seat in ipairs(update.view.seats) do
    seat.taken, seat.pass_chosen = 0, seat.seat == "p1"
  end
  return update
end

---@param update games.HeartsUpdate
---@param me games.UserID
---@param glyphs? games.Glyphs
---@return string[]
local function render(update, me, glyphs)
  return shell.render(update, hearts, view_options(glyphs or "ascii"), me, 0, nil, CATALOGUE).lines
end

---@param lines string[]
---@param text string
---@return integer|nil the first line showing text
local function line_of(lines, text)
  for i, line in ipairs(lines) do
    if line:find(text, 1, true) then
      return i
    end
  end
  return nil
end

---@param lines string[]
---@param text string
---@return boolean
local function shows(lines, text)
  return line_of(lines, text) ~= nil
end

---The three lines of the trick, each cut to the grid's width.
---@param lines string[]
---@return string[]
local function trick_rows(lines)
  local first = assert(line_of(lines, "Hand "), table.concat(lines, "\n"))
  return vim.tbl_map(function(line)
    return line:sub(1, 8)
  end, vim.list_slice(lines, first, first + 2))
end

local T = {}

function T.the_trick_sits_round_the_viewer_the_seat_on_its_left_on_the_left()
  assert_eq({ "   Kh   ", "..    2h", "   ..   " }, trick_rows(render(playing(), "bob")))
  assert_eq({ "   ..   ", "2h    ..", "   Kh   " }, trick_rows(render(playing(), "dave")))
end

function T.cards_played_draw_on_a_card_background_and_empty_places_do_not()
  local out = shell.render(playing(), hearts, view_options("ascii"), "bob", 0, nil, CATALOGUE)
  local backed = {}
  for _, mark in ipairs(out.extmarks) do
    if type(mark.hl) == "table" and vim.tbl_contains(mark.hl, "GamesSquareLight") then
      backed[#backed + 1] = out.lines[mark.row + 1]:sub(mark.col + 1, mark.end_col)
    end
  end
  assert_eq({ "Kh", "2h" }, backed)
end

function T.a_spectator_sees_the_first_seat_at_the_bottom()
  assert_eq({ "   ..   ", "..    Kh", "   2h   " }, trick_rows(render(playing(), "erin")))
end

function T.each_header_line_shows_the_score_points_taken_and_where_the_seat_sits()
  local lines = render(playing(), "bob")
  assert(lines[1]:find("alice  p1  13 pts     right", 1, true), lines[1])
  assert(lines[2]:find("bob    p2   0 pts +2", 1, true), lines[2])
  assert(lines[3]:find("carol  p3   8 pts     left", 1, true), lines[3])
  assert(lines[4]:find("dave   p4   5 pts +13 across out", 1, true), lines[4])
end

function T.the_table_shows_the_hand_hearts_the_last_trick_and_the_passes()
  local lines = render(playing(), "bob")
  assert(shows(lines, "Hand       2, passing right"), table.concat(lines, "\n"))
  assert(shows(lines, "Hearts     broken"))
  assert(shows(lines, "Last trick Ac Qs 3c 9c  carol took 13 points"))
  assert(shows(lines, "Passed     As Kd Qh to alice"))
  assert(shows(lines, "Received   2c 7c 4s from carol"))
  assert(shows(lines, "Now        play a card"))
end

function T.the_hand_marks_the_cards_that_may_be_played()
  local lines = render(playing(), "bob")
  local hand = assert(line_of(lines, "2c 7c Td Qd 4s Ks 3h 8h Ah"))
  assert_eq("^  ^  ^", vim.trim(lines[hand + 1]))
  assert_eq(lines[hand]:find("3h", 1, true), lines[hand + 1]:find("^", 1, true))
end

function T.the_last_hand_shows_each_seats_points_and_a_moon()
  local lines = render(playing(), "bob")
  assert(shows(lines, "Last hand 1"))
  assert(shows(lines, "alice      +13"))
  assert(shows(lines, "bob        +0"))
  local moon = playing({
    last_hand = {
      hand_no = 1,
      points = {
        { seat = "p1", points = 26 },
        { seat = "p2", points = 0 },
        { seat = "p3", points = 26 },
        { seat = "p4", points = 26 },
      },
      moon = "p2",
    },
  })
  assert(shows(render(moon, "bob"), "bob shot the moon"))
end

function T.a_seat_yet_to_pass_is_told_whom_to_and_may_press_p()
  local lines = render(passing(), "bob")
  assert(shows(lines, "Hand       3, passing across"), table.concat(lines, "\n"))
  assert(shows(lines, "Now        pass 3 cards to dave"))
  assert(lines[1]:find("alice  p1  13 pts  right  passed", 1, true), lines[1])
  local hand = assert(line_of(lines, "2c 7c Td Qd 4s Ks 3h 8h Ah"))
  assert_eq("", vim.trim(lines[hand + 1]))
  assert_eq("R resign  m play, e.g. qs  p pass cards  ]g next game  q close", lines[#lines])
  local pass = vim.iter(hearts.actions):find(function(entry)
    return entry.key == "p"
  end)
  assert_eq({ kind = "pass", cards = "2c qs ah" }, pass.input.parse("2c qs ah"))
  assert_eq({ kind = "play", card = "qs" }, hearts.input.parse("qs"))
end

function T.a_seat_that_has_passed_waits_for_the_others()
  local update = passing()
  update.view.must_pass, update.view.passed = 0, { "As", "Kd", "Qh" }
  local lines = render(update, "bob")
  assert(shows(lines, "Now        wait for the others to pass"), table.concat(lines, "\n"))
  assert(shows(lines, "Passed     As Kd Qh to dave"))
  assert_eq("R resign  m play, e.g. qs  ]g next game  q close", lines[#lines])
end

function T.a_hand_without_passing_says_so()
  local update = playing({ pass = "hold", passed = {}, received = {} })
  update.view.pass_to = nil
  local lines = render(update, "bob")
  assert(shows(lines, "Hand       2, no passing"), table.concat(lines, "\n"))
  assert(not shows(lines, "Passed"))
end

function T.unicode_cards_draw_their_suits()
  local lines = render(playing(), "bob", "unicode")
  assert(shows(lines, "2♣ 7♣ T♦ Q♦ 4♠ K♠ 3♥ 8♥ A♥"), table.concat(lines, "\n"))
end

function T.describe_event_tells_each_step()
  local ctx = { art = view_options("unicode").art, ambiwidth = "single" }
  local update = playing()
  for _, case in ipairs({
    { { kind = "hand_started", by = "p1", hand_no = 4, pass = "hold" }, "hand 4, no passing" },
    { { kind = "pass_chosen", by = "p2", seat = "p2" }, "bob has chosen the cards to pass" },
    { { kind = "passed", by = "p3", pass = "left" }, "the cards are passed left" },
    { { kind = "played", by = "p1", seat = "p4", card = "Qs" }, "dave plays Q♠" },
    {
      { kind = "trick_won", by = "p1", seat = "p3", points = 1 },
      "carol takes the trick, 1 point",
    },
    {
      {
        kind = "hand_scored",
        by = "p1",
        score = {
          hand_no = 2,
          points = {
            { seat = "p1", points = 3 },
            { seat = "p2", points = 0 },
            { seat = "p3", points = 13 },
            { seat = "p4", points = 10 },
          },
        },
      },
      "hand 2: alice +3, bob +0, carol +13, dave +10",
    },
    {
      { kind = "hand_scored", by = "p1", score = { hand_no = 3, points = {}, moon = "p2" } },
      "hand 3: bob shoots the moon",
    },
    { { kind = "left", by = "p4" }, "dave left the game; its cards are played for it" },
    { { kind = "other" }, nil },
  }) do
    assert_eq(case[2], hearts.describe_event(case[1], update, ctx))
  end
end

return T
