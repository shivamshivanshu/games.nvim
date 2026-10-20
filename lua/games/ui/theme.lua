local M = {}

local SEAT_COLOURS = 4

---Seat colours link to Diagnostic* because every colourscheme keeps their conventional hues.
---Seats 1 and 2 are Info (blue) and Warn (yellow): the blue-yellow axis survives the common
---red-green colour blindness, and the two differ in lightness too.
---@type table<string, string> semantic group -> default link target
local DEFAULT_LINKS = {
  GamesSquareLight = "CursorLine",
  GamesSquareDark = "ColorColumn",
  GamesLastMove = "Search",
  GamesCheck = "DiagnosticError",
  GamesYourTurn = "DiagnosticOk",
  GamesMuted = "Comment",
  GamesWin = "IncSearch",
  GamesSeat1 = "DiagnosticInfo",
  GamesSeat2 = "DiagnosticWarn",
  GamesSeat3 = "DiagnosticError",
  GamesSeat4 = "DiagnosticOk",
  GamesHit = "DiffAdd",
  GamesPresent = "DiffText",
  GamesMiss = "DiffChange",
}

function M.setup()
  vim.iter(DEFAULT_LINKS):each(function(name, target)
    vim.api.nvim_set_hl(0, name, { link = target, default = true })
  end)
end

---Highlight group for the seat at position i of update.players; colours repeat after four seats.
---@param i integer
---@return string
function M.seat_hl(i)
  return "GamesSeat" .. ((i - 1) % SEAT_COLOURS + 1)
end

return M
