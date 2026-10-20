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

return M
