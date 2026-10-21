local assets = require("games.ui.assets")
local describe = require("games.core.describe")
local helpers = require("helpers")
local poker = require("games.games.poker")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUE = { act = { input = { field = "text" } } }

---@param glyphs games.Glyphs
---@return games.ViewOptions
local function poker_view(glyphs)
  local spec = { keys = poker.asset_keys, glyph_width = poker.glyph_width }
  local pack = assets.load("poker", assets.DEFAULT_PACK, spec)
  return { glyphs = glyphs, flipped = false, art = assets.art_set(pack, glyphs) }
end

---Hand 3 on the flop: alice to act facing bob's bet of 40, carol out; view overrides on top.
---@param view_overrides? table
---@return games.PokerUpdate
local function poker_update(view_overrides)
  return helpers.game_update("poker", {
    seq = 4,
    players = {
      { user = "alice", seat = "p1" },
      { user = "bob", seat = "p2" },
      { user = "carol", seat = "p3" },
    },
    to_act = { "alice" },
    controls = { "resign", "abort" },
    view = {
      hand_no = 3,
      button = "p2",
      blinds = { small = 10, big = 20 },
      board = { "As", "Kh", "Td" },
      pots = { { amount = 60, eligible = { "p1", "p2" } }, { amount = 20, eligible = { "p2" } } },
      seats = {
        {
          seat = "p1",
          user = "alice",
          stack = 980,
          bet = 0,
          folded = false,
          allin = false,
          out = false,
        },
        {
          seat = "p2",
          user = "bob",
          stack = 940,
          bet = 40,
          folded = false,
          allin = false,
          out = false,
        },
        {
          seat = "p3",
          user = "carol",
          stack = 0,
          bet = 0,
          folded = true,
          allin = false,
          out = true,
        },
      },
      legal = { "fold", "call", "raise", "allin" },
      to_call = 40,
      min_raise_to = 80,
      max_raise_to = 980,
      my_cards = { "Qs", "Jc" },
      last_hand = {
        hand_no = 2,
        board = { "2c", "7d", "9h", "Js", "Kd" },
        shown = { { seat = "p1", cards = { "Ah", "Ac" }, category = "pair" } },
        awards = { { seat = "p1", amount = 30, pot = 0 }, { seat = "p2", amount = 10, pot = 1 } },
      },
    },
  }, { view = view_overrides })
end

---@param update games.PokerUpdate
---@param glyphs games.Glyphs
---@return string[]
local function render(update, glyphs)
  return shell.render(update, poker, poker_view(glyphs), "alice", 0, nil, CATALOGUE).lines
end

---@param mark games.Extmark
---@param group string
---@return boolean
local function has_hl(mark, group)
  local hl = mark.hl
  return hl == group or (type(hl) == "table" and vim.tbl_contains(hl, group))
end

local T = {}

function T.c_reads_check_or_call_and_the_amount_off_the_view()
  local checking =
    render(poker_update({ legal = { "fold", "check", "bet", "allin" }, to_call = 0 }), "ascii")

  assert(checking[#checking]:find("c check  ", 1, true), checking[#checking])
  assert(not checking[#checking]:find("call", 1, true), checking[#checking])
end

function T.c_sends_check_when_check_is_legal_and_call_otherwise()
  local c = vim.iter(poker.actions):find(function(entry)
    return entry.key == "c"
  end)
  local checking = poker_update({ legal = { "fold", "check", "bet", "allin" }, to_call = 0 })

  assert_eq({ kind = "act", text = "check" }, c.act(checking, poker_view("ascii")))
  assert_eq({ kind = "act", text = "call" }, c.act(poker_update(), poker_view("ascii")))
end

function T.hearts_and_diamonds_are_red_on_the_board_and_in_the_last_hand()
  local rendered =
    shell.render(poker_update(), poker, poker_view("unicode"), "alice", 0, nil, CATALOGUE)
  local red = vim.tbl_filter(function(mark)
    return has_hl(mark, "GamesCardRed")
  end, rendered.extmarks)
  local where = vim.tbl_map(function(mark)
    return { mark.row, rendered.lines[mark.row + 1]:sub(mark.col + 1, mark.end_col) }
  end, red)

  assert_eq({ { 5, "K♥" }, { 5, "T♦" }, { 12, "A♥" } }, where)
end

function T.seats_table_board_hand_and_betting_numbers_render_in_either_glyph_mode()
  for _, case in ipairs({
    {
      glyphs = "unicode",
      button = "Ⓓ ",
      board = "A♠ K♥ T♦ ·· ··",
      hand = "Q♠ J♣",
      shown = "A♥ A♣",
    },
    { glyphs = "ascii", button = "D ", board = "As Kh Td .. ..", hand = "Qs Jc", shown = "Ah Ac" },
  }) do
    assert_eq({
      "alice  p1     980             ←  your move ",
      "bob    p2  " .. case.button .. " 940 bet 40                   ",
      "carol  p3       0        out               ",
      "",
      "",
      case.board .. "   Hand     3  blinds 10/20      ",
      case.hand .. "            Pot      60 + side 20         ",
      "                 To call  40                   ",
      "                 Raise to 80-980               ",
      "                 Moves    fold call raise allin",
      "                                               ",
      "                 Last hand 2                   ",
      "                 alice      " .. case.shown .. "  pair        ",
      "                 alice won 30                  ",
      "                 bob won 10 (side 1)           ",
    }, vim.list_slice(render(poker_update(), case.glyphs), 1, 15))
  end
end

function T.only_the_seat_to_act_gets_fold_and_check_call_and_the_betting_lines()
  local acting = render(poker_update(), "ascii")
  local waiting =
    render(poker_update({ legal = {}, to_call = 0, min_raise_to = 0, max_raise_to = 0 }), "ascii")

  assert(acting[#acting]:find("f fold  c call 40", 1, true), acting[#acting])
  assert(not waiting[#waiting]:find("f fold", 1, true), waiting[#waiting])
  assert(not waiting[#waiting]:find("c ", 1, true), waiting[#waiting])
  assert(not table.concat(waiting, "\n"):find("To call", 1, true))
end

function T.the_raise_range_shows_only_when_a_bet_or_raise_is_legal()
  local short = render(
    poker_update({ legal = { "fold", "call", "allin" }, min_raise_to = 0, max_raise_to = 0 }),
    "ascii"
  )
  local opening = render(
    poker_update({ legal = { "fold", "check", "bet", "allin" }, to_call = 0, min_raise_to = 20 }),
    "ascii"
  )

  assert(not table.concat(short, "\n"):find("Raise to", 1, true))
  assert(table.concat(opening, "\n"):find("Bet%s+20%-980"), table.concat(opening, "\n"))
end

function T.events_read_as_table_talk_with_users_for_seats()
  local update = poker_update()
  update.events = {
    {
      kind = "hand_started",
      by = "p1",
      hand_no = 4,
      button = "p3",
      blinds = { small = 20, big = 40 },
    },
    { kind = "posted", by = "p1", seat = "p2", blind = "small", amount = 20 },
    { kind = "acted", by = "p1", move = "raise", amount = 120 },
    { kind = "acted", by = "p2", move = "fold", amount = 0 },
    { kind = "board", by = "p1", street = "flop", cards = { "As", "Kh", "Td" } },
    {
      kind = "shown",
      by = "p1",
      hands = { { seat = "p1", cards = { "Qs", "Jc" }, category = "straight" } },
    },
    { kind = "returned", by = "p1", seat = "p1", amount = 80 },
    { kind = "won", by = "p1", seat = "p2", amount = 50, pot = 1 },
    { kind = "busted", by = "p1", seat = "p3" },
    { kind = "left", by = "p3" },
  }

  assert_eq(
    table.concat({
      "hand 4, blinds 20/40",
      "bob posts 20",
      "alice raises to 120",
      "bob folds",
      "flop A♠ K♥ T♦",
      "showdown: alice Q♠ J♣",
      "80 returned to alice",
      "bob wins 50 (side 1)",
      "carol is out",
      "carol left the table",
    }, "; "),
    describe.events(update)
  )
end

function T.event_cards_take_the_users_glyph_mode()
  local update = poker_update()
  update.events = { { kind = "board", by = "p1", street = "turn", cards = { "Qh" } } }
  local games = require("games")

  assert_eq("turn Q♥", describe.events(update))
  helpers.with_overrides(
    { { games, "config", { glyphs = "ascii", board = {}, assets = {} } } },
    function()
      assert_eq("turn Qh", describe.events(update))
    end
  )
end

return T
