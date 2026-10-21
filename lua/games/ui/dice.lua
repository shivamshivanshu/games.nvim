local art = require("games.ui.art")

---@alias games.DieFace integer 1 to 6, as on the wire

local FACES = 6
local DIE_GAP = " "

---Six-sided dice as games and their asset packs name and draw them: one key per face.
local M = {}

---The asset key of face.
---@param face games.DieFace
---@return string
function M.key(face)
  return "face" .. face
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
  return table.concat(
    vim.tbl_map(function(face)
      return art.glyph(set, M.key(face), ambiwidth)
    end, faces),
    DIE_GAP
  )
end

return M
