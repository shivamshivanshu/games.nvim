local art = require("games.ui.art")

---@alias games.DieFace integer 1 to 6, as on the wire

local FACES = 6
local DIE_GAP = " "

---Six-sided dice as games and their asset packs name and draw them: one key per face.
local M = {}

---The asset key of a die not yet rolled.
M.NONE_KEY = "none"

---The asset key of face.
---@param face games.DieFace
---@return string
function M.key(face)
  return "face" .. face
end

---The grid cell of a die showing face, or of one not yet rolled.
---@param face games.DieFace|nil
---@return games.GridCell
function M.cell(face)
  return { face and M.key(face) or M.NONE_KEY }
end

---The six face keys, from one to six.
---@return string[]
function M.keys()
  return vim.tbl_map(M.key, vim.fn.range(1, FACES))
end

---Faces as their glyphs in set, a space apart, for text such as a panel line or a notification.
---@param set games.ArtSet
---@param faces games.DieFace[]
---@param ambiwidth games.Ambiwidth
---@return string
function M.text(set, faces, ambiwidth)
  return art.glyph_line(set, vim.tbl_map(M.key, faces), DIE_GAP, ambiwidth)
end

return M
