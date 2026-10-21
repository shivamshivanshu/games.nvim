local M = {}

---@param update games.GameUpdate
---@param seat games.Seat
---@return games.UserID|nil
function M.user_of_seat(update, seat)
  local player = vim.iter(update.players):find(function(p)
    return p.seat == seat
  end)
  return player and player.user
end

---@param update games.GameUpdate
---@param user games.UserID
---@return games.Seat|nil
function M.seat_of_user(update, user)
  local player = vim.iter(update.players):find(function(p)
    return p.user == user
  end)
  return player and player.seat
end

---Position of seat in update.players, which orders the header and the seat colours.
---@param update games.GameUpdate
---@param seat games.Seat
---@return integer|nil
function M.seat_index(update, seat)
  for i, player in ipairs(update.players) do
    if player.seat == seat then
      return i
    end
  end
  return nil
end

---Whether the game is ongoing and waiting on user.
---@param update games.GameUpdate
---@param user games.UserID
---@return boolean
function M.is_to_act(update, user)
  return update.result == nil and vim.tbl_contains(update.to_act, user)
end

---Whether update's game is played alone.
---@param update games.GameUpdate
---@return boolean
function M.is_solo(update)
  return #update.players == 1
end

---Whether info's game seats its player alone, so it starts without a challenge.
---@param info games.GameInfo
---@return boolean
function M.is_solo_game(info)
  return info.players.max == 1
end

---Whether info's game may seat one player alone, though it may also take opponents.
---@param info games.GameInfo
---@return boolean
function M.can_start_solo(info)
  return info.players.min == 1
end

return M
