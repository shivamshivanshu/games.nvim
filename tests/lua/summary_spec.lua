local helpers = require("helpers")
local summary = require("games.core.summary")

local assert_eq = helpers.assert_eq

local AS_OF_MS = 1792584000000
local A_SECOND_LATER_MS = AS_OF_MS + 1000

---@param clock_option string the option the catalog marks as the clock
---@return games.GameInfo
local function chess_info(clock_option)
  return {
    id = "chess",
    name = "Chess",
    players = { min = 2, max = 2 },
    capabilities = {},
    options = {
      [clock_option] = { type = "enum", values = { "10+5" }, default = "10+5", role = "clock" },
      color = { type = "enum", values = { "white" }, default = "white" },
    },
    actions = {},
  }
end

local function ravi_played_nf3(overrides)
  return vim.tbl_extend("force", {
    game_id = "7f3a91c2",
    game = "chess",
    seq = 6,
    options = { time_control = "10+5", color = "white" },
    players = { { user = "alice", seat = "white" }, { user = "ravi", seat = "black" } },
    to_act = { "alice" },
    clock = {
      remaining_ms = { alice = 292000, ravi = 288000 },
      running = "alice",
      as_of_ms = AS_OF_MS,
    },
    controls = { "resign", "offer_draw" },
    events = { { kind = "moved", by = "black", san = "Nf3" } },
  }, overrides or {})
end

local T = {}

function T.format_game_line()
  assert_eq(
    "● chess  vs ravi    10+5  your move  4:51  last: ravi played Nf3  #7f3a",
    summary.format_game(ravi_played_nf3(), "alice", chess_info("time_control"), A_SECOND_LATER_MS)
  )
  assert_eq(
    "  chess  vs ravi    10+5  waiting    4:51  last: ravi played Nf3  #7f3a",
    summary.format_game(
      ravi_played_nf3({ to_act = { "ravi" } }),
      "alice",
      chess_info("time_control"),
      A_SECOND_LATER_MS
    )
  )
end

function T.time_control_is_the_option_the_catalog_marks_as_the_clock()
  local update = ravi_played_nf3({ options = { pace = "3+2", time_control = "10+5" } })
  local line = summary.format_game(update, "alice", chess_info("pace"), AS_OF_MS)
  assert(line:find("vs ravi    3+2 ", 1, true), line)
  assert_eq("chess", summary.game_summary("chess", nil, update.options))
  assert_eq("Chess 3+2", summary.game_summary("chess", chess_info("pace"), update.options))
end

function T.format_finished_game_line()
  local finished = ravi_played_nf3({
    to_act = {},
    clock = { remaining_ms = { alice = 605000, ravi = 288000 }, as_of_ms = AS_OF_MS },
    result = { kind = "decisive", winners = { "ravi" }, reason = "checkmate" },
  })
  assert_eq(
    "  chess  vs ravi    10+5  finished  10:05  last: ravi played Nf3  #7f3a",
    summary.format_game(finished, "alice", chess_info("time_control"), A_SECOND_LATER_MS)
  )
end

return T
