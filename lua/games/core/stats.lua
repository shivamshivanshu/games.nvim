---@class (private) games.UserTally: games.Tally
---@field user games.UserID

local M = {}

---@param tally games.Tally
---@return string
local function tally_text(tally)
  return ("W %d  L %d  D %d"):format(tally.wins, tally.losses, tally.draws)
end

---@alias games.StatsRow {[1]: string, [2]: string}

---@generic T: games.Tally
---@param tallies T[]
---@param label fun(tally: T, index: integer): string
---@return games.StatsRow[]
local function rows_of(tallies, label)
  local rows = {}
  for i, tally in ipairs(tallies) do
    rows[i] = { label(tally, i), tally_text(tally) }
  end
  return rows
end

---@generic T: {user: games.UserID}
---@param records T[]
---@param user games.UserID
---@return T[]
local function of_user(records, user)
  return vim.tbl_filter(function(record)
    return record.user == user
  end, records)
end

---Per-game record, then head-to-head, for one user.
---@param stats games.StatsResult
---@param user games.UserID
---@return games.StatsRow[]
function M.stats_pairs(stats, user)
  local records = of_user(stats.records, user)
  table.sort(records, function(a, b)
    return a.game < b.game
  end)
  local rivals = of_user(stats.head_to_head, user)
  table.sort(rivals, function(a, b)
    return a.opponent < b.opponent or (a.opponent == b.opponent and a.game < b.game)
  end)
  return vim.list_extend(
    rows_of(records, function(record)
      return record.game
    end),
    rows_of(rivals, function(record)
      return ("vs %s (%s)"):format(record.opponent, record.game)
    end)
  )
end

---Users ranked by wins, then fewest losses, then most draws; totals span every game when game is nil.
---@param records games.StatsRecord[]
---@param game games.GameType|nil
---@return games.StatsRow[]
function M.leaderboard_pairs(records, game)
  ---@type table<games.UserID, games.UserTally>
  local totals = {}
  for _, record in ipairs(records) do
    if game == nil or record.game == game then
      local total = totals[record.user] or { user = record.user, wins = 0, losses = 0, draws = 0 }
      totals[record.user] = {
        user = record.user,
        wins = total.wins + record.wins,
        losses = total.losses + record.losses,
        draws = total.draws + record.draws,
      }
    end
  end
  local ranked = vim.tbl_values(totals)
  table.sort(ranked, function(a, b)
    if a.wins ~= b.wins then
      return a.wins > b.wins
    end
    if a.losses ~= b.losses then
      return a.losses < b.losses
    end
    if a.draws ~= b.draws then
      return a.draws > b.draws
    end
    return a.user < b.user
  end)
  return rows_of(ranked, function(total, rank)
    return ("%d. %s"):format(rank, total.user)
  end)
end

return M
