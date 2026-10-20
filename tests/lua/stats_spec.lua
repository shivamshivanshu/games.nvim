local stats = require("games.core.stats")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

local function tally(fields, wins, losses, draws)
  return vim.tbl_extend("error", fields, { wins = wins, losses = losses, draws = draws })
end

---@type games.StatsResult
local RANKED = {
  records = {
    tally({ user = "bob", game = "chess" }, 2, 1, 0),
    tally({ user = "alice", game = "chess" }, 2, 3, 0),
    tally({ user = "carol", game = "ttt" }, 3, 0, 0),
  },
  head_to_head = {},
  leaderboard = {
    tally({ user = "carol" }, 3, 0, 0),
    tally({ user = "bob" }, 2, 1, 0),
    tally({ user = "alice" }, 2, 3, 0),
  },
}

local T = {}

function T.leaderboard_numbers_the_servers_ranking_of_one_game_or_all()
  assert_eq({
    { "1. bob", "W 2  L 1  D 0" },
    { "2. alice", "W 2  L 3  D 0" },
  }, stats.leaderboard_pairs(RANKED, "chess"))
  assert_eq({
    { "1. carol", "W 3  L 0  D 0" },
    { "2. bob", "W 2  L 1  D 0" },
    { "3. alice", "W 2  L 3  D 0" },
  }, stats.leaderboard_pairs(RANKED, nil))
end

return T
