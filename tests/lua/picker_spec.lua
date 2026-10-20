local picker = require("games.ui.picker")

local function assert_eq(expected, actual)
  assert(vim.deep_equal(expected, actual), vim.inspect({ expected = expected, actual = actual }))
end

local function ravi_played_nf3(overrides)
  return vim.tbl_extend("force", {
    game_id = "7f3a91c2",
    game = "chess",
    seq = 6,
    options = { time_control = "10+5", color = "white" },
    players = { { user = "alice", seat = "white" }, { user = "ravi", seat = "black" } },
    to_act = { "alice" },
    clock = { remaining_ms = { alice = 291000, ravi = 288000 }, running = "alice" },
    controls = { "resign", "offer_draw" },
    events = { { kind = "moved", by = "black", san = "Nf3" } },
  }, overrides or {})
end

local T = {}

function T.format_game_line()
  assert_eq(
    "● chess  vs ravi    10+5  your move  4:51  last: ravi played Nf3  #7f3a",
    picker.format_game(ravi_played_nf3(), "alice")
  )
  assert_eq(
    "  chess  vs ravi    10+5  waiting    4:51  last: ravi played Nf3  #7f3a",
    picker.format_game(ravi_played_nf3({ to_act = { "ravi" } }), "alice")
  )
end

return T
