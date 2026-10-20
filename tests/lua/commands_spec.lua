local commands = require("games.core.commands")

local function assert_eq(expected, actual)
  assert(vim.deep_equal(expected, actual), vim.inspect({ expected = expected, actual = actual }))
end

local function record(user, game, wins, losses, draws)
  return { user = user, game = game, wins = wins, losses = losses, draws = draws }
end

local T = {}

function T.leaderboard_ranks_wins_then_fewest_losses()
  local records = {
    record("alice", "chess", 2, 3, 0),
    record("bob", "chess", 2, 1, 0),
    record("carol", "chess", 1, 0, 4),
    record("carol", "ttt", 3, 0, 0),
  }

  assert_eq({
    { "1. bob", "W 2  L 1  D 0" },
    { "2. alice", "W 2  L 3  D 0" },
    { "3. carol", "W 1  L 0  D 4" },
  }, commands.leaderboard_pairs(records, "chess"))
  assert_eq({ "1. carol", "W 4  L 0  D 4" }, commands.leaderboard_pairs(records, nil)[1])
end

return T
