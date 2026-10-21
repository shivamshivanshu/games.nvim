local M = {}

---The first of update's players pred holds for, with its position in update.players.
---@param update games.GameUpdate
---@param pred fun(player: games.PlayerSeat): boolean
---@return games.PlayerSeat|nil
---@return integer|nil
local function seated(update, pred)
  for i, player in ipairs(update.players) do
    if pred(player) then
      return player, i
    end
  end
  return nil, nil
end

---@param update games.GameUpdate
---@param seat games.Seat
---@return games.UserID|nil
function M.user_of_seat(update, seat)
  local player = seated(update, function(p)
    return p.seat == seat
  end)
  return player and player.user
end

---The name a seat shows as: its player's, or the seat's own while no one holds it.
---@param update games.GameUpdate
---@param seat games.Seat
---@return string
function M.name_of_seat(update, seat)
  return M.user_of_seat(update, seat) or seat
end

---@param update games.GameUpdate
---@param user games.UserID
---@return games.Seat|nil
function M.seat_of_user(update, user)
  local player = seated(update, function(p)
    return p.user == user
  end)
  return player and player.seat
end

---Position of seat in update.players, which orders the header and the seat colours.
---@param update games.GameUpdate
---@param seat games.Seat
---@return integer|nil
function M.seat_index(update, seat)
  local _, index = seated(update, function(p)
    return p.seat == seat
  end)
  return index
end

---@enum games.GameStage where a game stands for one player, in the order they need that player
M.GameStage = { YOUR_MOVE = 1, WAITING = 2, FINISHED = 3 }

---Where update's game stands for me.
---@param update games.GameUpdate
---@param me games.UserID
---@return games.GameStage
function M.stage(update, me)
  if update.result then
    return M.GameStage.FINISHED
  end
  return M.is_to_act(update, me) and M.GameStage.YOUR_MOVE or M.GameStage.WAITING
end

---The entry of list that is seat's, such as a view's per-seat rows.
---@generic T: {seat: games.Seat}
---@param list T[]
---@param seat games.Seat
---@return T|nil
function M.entry_of_seat(list, seat)
  return vim.iter(list):find(function(entry)
    return entry.seat == seat
  end)
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
