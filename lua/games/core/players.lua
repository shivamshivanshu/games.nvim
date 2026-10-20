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

---Position of seat in update.players, which is the order seat colours follow.
---@param update games.GameUpdate
---@param seat games.Seat
---@return integer|nil
function M.seat_index(update, seat)
  for index, player in ipairs(update.players) do
    if player.seat == seat then
      return index
    end
  end
end

---Whether the game is ongoing and waiting on user.
---@param update games.GameUpdate
---@param user games.UserID
---@return boolean
function M.is_to_act(update, user)
  return update.result == nil and vim.tbl_contains(update.to_act, user)
end

return M
