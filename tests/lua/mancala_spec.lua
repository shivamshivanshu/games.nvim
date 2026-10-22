local helpers = require("helpers")
local mancala = require("games.games.mancala")

local assert_eq = helpers.assert_eq

---Lines above the board at cell height 1: two players, the status row and a blank.
local BOARD_TOP = 4
---Display columns from one cell's start to the next: the glyph and the gap.
local CELL_STEP = 5
local BOARD = { top = BOARD_TOP, left = 1, step = CELL_STEP }

---alice (south) sowed pit 1, then bob (north) captured; bob is to move again after a store landing.
---@param overrides? table
---@return games.MancalaUpdate
local function update_with(overrides)
  return helpers.game_update("mancala", {
    seq = 3,
    players = { { user = "alice", seat = "south" }, { user = "bob", seat = "north" } },
    to_act = { "bob" },
    controls = { "resign", "offer_draw" },
    view = {
      south = { pits = { 0, 5, 5, 5, 5, 4 }, store = 1 },
      north = { pits = { 4, 0, 5, 5, 12, 4 }, store = 7 },
      legal_pits = { 1, 3, 4, 5, 6 },
    },
  }, overrides)
end

---@param update games.MancalaUpdate
---@param me games.UserID
---@param glyphs? games.Glyphs
---@return games.Rendered
local function render(update, me, glyphs)
  return (helpers.render_game("mancala", update, { me = me, glyphs = glyphs }))
end

---@param rendered games.Rendered
---@param r integer
---@param c integer
---@return games.Hl[] the highlight stack on cell r, c at cell height 1
local function cell_hl(rendered, r, c)
  return helpers.cell_hl_at(rendered, BOARD, r, c)
end

---@param rendered games.Rendered
---@return string
local function footer(rendered)
  return rendered.lines[#rendered.lines]
end

local T = {}

function T.the_viewer_sits_at_the_bottom_sowing_left_to_right_into_the_right_store()
  for _, glyphs in ipairs({ "unicode", "ascii" }) do
    assert_eq({
      " ( 1)   4    5    5    5    5    0         alice top row, store on the left    ",
      "      [ 4]   0  [ 5] [ 5] [12] [ 4] ( 7)   bob   bottom row, store on the right",
      "       1    2    3    4    5    6                                              ",
    }, vim.list_slice(render(update_with(), "bob", glyphs).lines, BOARD_TOP + 1, BOARD_TOP + 3))
  end
  local waiting = update_with({
    view = { legal_pits = {} },
  })
  assert_eq({
    " ( 7)   4   12    5    5    0    4         bob   top row, store on the left    ",
    "        0    5    5    5    5    4  ( 1)   alice bottom row, store on the right",
  }, vim.list_slice(render(waiting, "alice").lines, BOARD_TOP + 1, BOARD_TOP + 2))
end

function T.each_side_takes_its_players_colour_and_legal_pits_are_tinted()
  local rendered = render(update_with(), "bob")
  assert(
    vim.tbl_contains(cell_hl(rendered, 1, 1), "GamesSeat1"),
    "alice's store is the first seat's"
  )
  assert(vim.tbl_contains(cell_hl(rendered, 2, 8), "GamesSeat2"), "bob's store")
  assert(vim.tbl_contains(cell_hl(rendered, 2, 2), "GamesMovable"), "bob's pit 1 may be sown")
  assert(not vim.tbl_contains(cell_hl(rendered, 2, 3), "GamesMovable"), "bob's empty pit 2")
  assert(not vim.tbl_contains(cell_hl(rendered, 1, 2), "GamesMovable"), "alice's pits")
end

function T.sow_keys_are_bound_for_the_movers_legal_pits_only()
  assert_eq(
    "R resign  D offer draw  1,3-6 sow  ]g next game  ? help  q close",
    footer(render(update_with(), "bob"))
  )
  local waiting = update_with({
    view = { legal_pits = {} },
  })
  assert_eq(
    "R resign  D offer draw  ]g next game  ? help  q close",
    footer(render(waiting, "alice"))
  )
end

function T.a_sow_key_sends_its_pit_as_text()
  local three = vim.iter(mancala.actions):find(function(entry)
    return entry.key == "3"
  end)
  assert_eq({ kind = "sow", pit = "3" }, three.action)
  assert_eq(6, #mancala.actions)
end

function T.events_tell_captures_extra_turns_and_the_final_collection()
  local update = update_with()
  local function sowed(fields)
    return vim.tbl_extend(
      "force",
      { kind = "sowed", by = "north", pit = 2, captured = 0, again = false },
      fields
    )
  end
  assert_eq("bob sowed pit 2", helpers.described(update, sowed({})))
  assert_eq("bob sowed pit 2, capturing 8", helpers.described(update, sowed({ captured = 8 })))
  assert_eq("bob sowed pit 2, and moves again", helpers.described(update, sowed({ again = true })))
  assert_eq(
    "alice collects the 6 seeds left on their side",
    helpers.described(update, { kind = "collected", by = "south", seeds = 6 })
  )
end

return T
