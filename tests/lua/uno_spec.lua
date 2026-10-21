local helpers = require("helpers")
local shell = require("games.ui.shell")
local uno = require("games.games.uno")

local assert_eq = helpers.assert_eq

local CATALOGUE =
  { play = true, draw = true, pass = true, challenge = true, accept = true, color = true }

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("uno", uno, glyphs, extra)
end

---alice (p1) to play on a wild that named green; bob has one card left and carol left.
---@param view? table overrides of the view
---@return games.UnoUpdate
local function playing(view)
  return {
    game_id = "0000",
    game = "uno",
    seq = 3,
    options = {},
    players = {
      { user = "alice", seat = "p1" },
      { user = "bob", seat = "p2" },
      { user = "carol", seat = "p3" },
    },
    to_act = { "alice" },
    controls = { "resign" },
    events = {},
    view = vim.tbl_extend("force", {
      top = "W",
      color = "G",
      direction = "counterclockwise",
      phase = "play",
      draw_pile = 61,
      seats = {
        { seat = "p1", cards = 5, out = false },
        { seat = "p2", cards = 1, out = false },
        { seat = "p3", cards = 0, out = true },
      },
      my_hand = { "R5", "RS", "G+2", "BR", "W+4" },
      legal = { "play", "draw" },
      playable = { "G+2", "W+4" },
    }, view or {}),
  }
end

---@param update games.UnoUpdate
---@param me games.UserID
---@param glyphs? games.Glyphs
---@return games.Rendered
local function rendered(update, me, glyphs)
  return shell.render(update, uno, view_options(glyphs or "ascii"), me, 0, nil, CATALOGUE)
end

---@param update games.UnoUpdate
---@param me games.UserID
---@param glyphs? games.Glyphs
---@return string[]
local function render(update, me, glyphs)
  return rendered(update, me, glyphs).lines
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

local T = {}

function T.the_table_shows_the_colour_in_play_by_name_and_letter()
  local lines = render(playing(), "alice")
  assert(shows(lines, "Colour    green (G)"), table.concat(lines, "\n"))
  assert(shows(lines, "Turns     counterclockwise"))
  assert(shows(lines, "Draw pile 61 cards"))
  assert(shows(lines, "Now       play a card or draw"))
end

function T.the_hand_marks_the_cards_that_may_be_played()
  local lines = render(playing(), "alice")
  local hand = assert(line_of(lines, "R5  RS  G+2 BR  W+4"), table.concat(lines, "\n"))
  assert_eq("^       ^", vim.trim(lines[hand + 1]))
  assert_eq(lines[hand]:find("G+2", 1, true), lines[hand + 1]:find("^", 1, true))
end

function T.a_long_hand_wraps_to_the_panel()
  local hand = { "R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9", "Y0" }
  local lines = render(playing({ my_hand = hand, playable = { "Y0" } }), "alice")
  local first = assert(line_of(lines, "R0  R1  R2  R3  R4  R5  R6  R7  R8"))
  assert_eq("", vim.trim(lines[first + 1]))
  assert(lines[first + 2]:find("R9  Y0", 1, true), lines[first + 2])
  assert_eq("^", vim.trim(lines[first + 3]))
end

function T.the_seat_to_act_sees_what_to_do_in_each_phase()
  for _, case in ipairs({
    {
      view = { legal = { "draw" }, playable = {} },
      now = "nothing to play: draw",
      keys = "d draw",
    },
    {
      view = { phase = "drawn", legal = { "play", "pass" }, playable = { "BR" } },
      now = "play the card drawn or keep it",
      keys = "k keep",
    },
    {
      view = { phase = "drawn", legal = { "pass" }, playable = {} },
      now = "it does not match: keep it",
      keys = "k keep",
    },
    {
      view = { phase = "challenge", legal = { "challenge", "accept" }, playable = {} },
      now = "challenge the Wild Draw Four or accept it",
      keys = "c challenge  a accept",
    },
    {
      view = { phase = "color", color = "", legal = { "color" }, playable = {} },
      now = "name the colour",
      keys = "r red  y yellow  g green  b blue",
    },
  }) do
    local lines = render(playing(case.view), "alice")
    assert(shows(lines, "Now       " .. case.now), table.concat(lines, "\n"))
    assert_eq(
      ("R resign  m play, e.g. r5 or w g  %s  ]g next game  q close"):format(case.keys),
      lines[#lines]
    )
  end
end

function T.before_a_wild_turned_up_is_named_there_is_no_colour()
  local lines = render(playing({ phase = "color", color = "", legal = {}, playable = {} }), "bob")
  assert(shows(lines, "Colour    not named yet"), table.concat(lines, "\n"))
end

function T.a_waiting_seat_sees_no_hint_marks_or_moves()
  local lines = render(playing({ legal = {}, playable = {}, my_hand = { "Y1", "B2" } }), "bob")
  assert(not shows(lines, "Now"))
  local hand = assert(line_of(lines, "Y1  B2"))
  assert_eq("", vim.trim(lines[hand + 1]))
  assert_eq("R resign  m play, e.g. r5 or w g  ]g next game  q close", lines[#lines])
end

function T.colour_keys_name_their_colour()
  local named = vim.tbl_map(
    function(entry)
      return { entry.key, entry.action }
    end,
    vim.tbl_filter(function(entry)
      return entry.action.kind == "color"
    end, uno.actions)
  )
  assert_eq({
    { "r", { kind = "color", color = "R" } },
    { "y", { kind = "color", color = "Y" } },
    { "g", { kind = "color", color = "G" } },
    { "b", { kind = "color", color = "B" } },
  }, named)
  assert_eq({ kind = "play", card = "w g" }, uno.input.parse("w g"))
end

function T.each_header_line_shows_the_seats_cards_uno_on_the_last_or_out()
  local lines = render(playing(), "alice")
  assert(lines[1]:find("alice  p1  5 cards", 1, true), lines[1])
  assert(lines[2]:find("bob    p2  1 card UNO", 1, true), lines[2])
  assert(lines[3]:find("carol  p3  out", 1, true), lines[3])
end

function T.cards_are_drawn_in_their_colour_and_the_top_in_the_colour_in_play()
  local out = rendered(playing(), "alice", "unicode")
  local coloured = {}
  for _, mark in ipairs(out.extmarks) do
    local hl = type(mark.hl) == "table" and mark.hl[#mark.hl] or mark.hl
    if type(hl) == "string" and hl:find("^GamesCard") then
      coloured[#coloured + 1] = { out.lines[mark.row + 1]:sub(mark.col + 1, mark.end_col), hl }
    end
  end
  assert_eq({
    { "W  ", "GamesCardGreen" },
    { "R5 ", "GamesCardRed" },
    { "R⊘ ", "GamesCardRed" },
    { "G+2", "GamesCardGreen" },
    { "B⇄ ", "GamesCardBlue" },
  }, coloured)
end

function T.the_pile_and_the_draw_pile_draw_on_a_card_background()
  local out = rendered(playing(), "alice")
  local backed = {}
  for _, mark in ipairs(out.extmarks) do
    if type(mark.hl) == "table" and vim.tbl_contains(mark.hl, "GamesSquareLight") then
      backed[#backed + 1] = out.lines[mark.row + 1]:sub(mark.col + 1, mark.end_col)
    end
  end
  assert_eq({ "W  ", "###" }, backed)
end

function T.describe_event_tells_each_step()
  local ctx = { art = view_options("unicode").art, ambiwidth = "single" }
  local update = playing()
  for _, case in ipairs({
    { { kind = "started", by = "p2", dealer = "p2", top = "Y7" }, "bob deals and turns up Y7" },
    { { kind = "played", by = "p1", card = "RS", color = "R" }, "alice plays R⊘" },
    {
      { kind = "played", by = "p1", card = "W+4", color = "B" },
      "alice plays W+4 and names blue",
    },
    { { kind = "drew", by = "p1", seat = "p2", count = 4 }, "bob draws 4 cards" },
    { { kind = "passed", by = "p2" }, "bob keeps the card drawn" },
    { { kind = "skipped", by = "p1", seat = "p2" }, "bob is skipped" },
    { { kind = "reversed", by = "p1", direction = "clockwise" }, "play turns clockwise" },
    { { kind = "color", by = "p1", color = "Y" }, "alice names yellow" },
    {
      { kind = "challenged", by = "p2", seat = "p1", guilty = true },
      "bob challenges alice's Wild Draw Four: guilty",
    },
    {
      { kind = "reshuffled", by = "p1", cards = 40 },
      "the discards are shuffled into a new draw pile of 40 cards",
    },
    { { kind = "left", by = "p3" }, "carol left the game" },
    { { kind = "other" }, nil },
  }) do
    assert_eq(case[2], uno.describe_event(case[1], update, ctx))
  end
end

return T
