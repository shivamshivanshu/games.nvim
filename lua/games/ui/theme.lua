local M = {}

---@type table<string, string> semantic group -> default link target
local DEFAULT_LINKS = {
  GamesSquareLight = "CursorLine",
  GamesSquareDark = "ColorColumn",
  GamesLastMove = "Search",
  GamesCheck = "DiagnosticError",
  GamesYourTurn = "DiagnosticOk",
  GamesMuted = "Comment",
}

---@return nil
function M.setup()
  for name, target in pairs(DEFAULT_LINKS) do
    vim.api.nvim_set_hl(0, name, { link = target, default = true })
  end
end

return M
