---@type games.Tally
local EMPTY_TALLY = { wins = 0, losses = 0, draws = 0 }

local M = {}

---@param tally games.Tally
---@return string
local function tally_text(tally)
  return ("W %d  L %d  D %d"):format(tally.wins, tally.losses, tally.draws)
end

---Per-game record, then head-to-head, for one user.
---@param stats games.StatsResult
---@param user games.UserID
---@return {[1]: string, [2]: string}[]
function M.stats_pairs(stats, user)
  local records = vim.tbl_filter(function(record)
    return record.user == user
  end, stats.records)
  table.sort(records, function(a, b)
    return a.game < b.game
  end)
  local rivals = vim.tbl_filter(function(record)
    return record.user == user
  end, stats.head_to_head)
  table.sort(rivals, function(a, b)
    return a.opponent < b.opponent or (a.opponent == b.opponent and a.game < b.game)
  end)
  return vim.list_extend(
    vim.tbl_map(function(record)
      return { record.game, tally_text(record) }
    end, records),
    vim.tbl_map(function(record)
      return { ("vs %s (%s)"):format(record.opponent, record.game), tally_text(record) }
    end, rivals)
  )
end

---@param a games.Tally
---@param b games.Tally
---@return games.Tally
local function add_tallies(a, b)
  return { wins = a.wins + b.wins, losses = a.losses + b.losses, draws = a.draws + b.draws }
end

---Users ranked by wins, then fewest losses, then most draws; totals span every game when game is nil.
---@param records games.StatsRecord[]
---@param game games.GameType|nil
---@return {[1]: string, [2]: string}[]
function M.leaderboard_pairs(records, game)
  ---@type table<games.UserID, games.Tally>
  local totals = {}
  for _, record in ipairs(records) do
    if game == nil or record.game == game then
      totals[record.user] = add_tallies(totals[record.user] or EMPTY_TALLY, record)
    end
  end
  local ranked = vim.tbl_keys(totals)
  table.sort(ranked, function(a, b)
    local ta, tb = totals[a], totals[b]
    if ta.wins ~= tb.wins then
      return ta.wins > tb.wins
    end
    if ta.losses ~= tb.losses then
      return ta.losses < tb.losses
    end
    if ta.draws ~= tb.draws then
      return ta.draws > tb.draws
    end
    return a < b
  end)
  local rows = {}
  for rank, user in ipairs(ranked) do
    rows[rank] = { ("%d. %s"):format(rank, user), tally_text(totals[user]) }
  end
  return rows
end

return M
