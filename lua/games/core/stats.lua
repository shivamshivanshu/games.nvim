local M = {}

---@param tally games.Tally
---@return string
local function tally_text(tally)
  return ("W %d  L %d  D %d"):format(tally.wins, tally.losses, tally.draws)
end

---@generic T: games.Tally
---@param tallies T[]
---@param label fun(tally: T, index: integer): string
---@return games.KvPair[]
local function rows_of(tallies, label)
  local rows = {}
  for i, tally in ipairs(tallies) do
    rows[i] = { label(tally, i), tally_text(tally) }
  end
  return rows
end

---Per-game record, then head-to-head, of the user stats were requested for, in the server's order.
---@param stats games.StatsResult of one user
---@return games.KvPair[]
function M.stats_pairs(stats)
  return vim.list_extend(
    rows_of(stats.records, function(record)
      return record.game
    end),
    rows_of(stats.head_to_head, function(record)
      return ("vs %s (%s)"):format(record.opponent, record.game)
    end)
  )
end

---The server's ranking: of game's records when game is given, else of the totals over every game.
---@param stats games.StatsResult
---@param game games.GameType|nil
---@return games.KvPair[]
function M.leaderboard_pairs(stats, game)
  ---@type games.LeaderboardEntry[]
  local ranked = game
      and vim.tbl_filter(function(record)
        return record.game == game
      end, stats.records)
    or stats.leaderboard
  return rows_of(ranked, function(entry, rank)
    return ("%d. %s"):format(rank, entry.user)
  end)
end

return M
