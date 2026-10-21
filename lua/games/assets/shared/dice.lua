local build = require("games.assets.shared.build")
local text = require("games.ui.text")

local GLYPH_WIDTH = 1

---@type table<games.Glyphs, games.ArtSet>
local GLYPHS = {
  unicode = build.glyph_set({
    face1 = "⚀",
    face2 = "⚁",
    face3 = "⚂",
    face4 = "⚃",
    face5 = "⚄",
    face6 = "⚅",
    -- "·" and "▲" are two cells wide under ambiwidth=double, where the ascii ones stand in.
    none = "·",
    held = "▲",
    free = " ",
  }, GLYPH_WIDTH),
  ascii = build.glyph_set({
    face1 = "1",
    face2 = "2",
    face3 = "3",
    face4 = "4",
    face5 = "5",
    face6 = "6",
    none = ".",
    held = "^",
    free = " ",
  }, GLYPH_WIDTH),
}

---Which of a die's 3x3 pip spots each face fills, row by row from the top.
---@type table<string, string[]>
local PIPS = {
  face1 = { "...", ".o.", "..." },
  face2 = { "o..", "...", "..o" },
  face3 = { "o..", ".o.", "..o" },
  face4 = { "o.o", "...", "o.o" },
  face5 = { "o.o", ".o.", "o.o" },
  face6 = { "o.o", "o.o", "o.o" },
}

---A face as a die outlined by thin block edges, its pips small squares, a pip row to a line.
---@param spots string[]
---@return games.ArtEntry
local function outlined(spots)
  local inner = 2 * #spots[1]
  local lines = { " " .. ("▁"):rep(inner) .. " " }
  for _, row in ipairs(spots) do
    local pips = vim.tbl_map(function(spot)
      return spot == "o" and "▗▖" or "  "
    end, text.chars(row))
    lines[#lines + 1] = "▕" .. table.concat(pips) .. "▏"
  end
  lines[#lines + 1] = "▕" .. ("▁"):rep(inner) .. "▏"
  return build.sprite(lines)
end

---Dice at cell height 5 and up, two columns narrower than the cell so dice side by side stay apart;
---shorter cells show the glyph.
---@type games.ArtSet
local ART = vim.iter(PIPS):fold({}, function(set, key, spots)
  set[key] = { outlined(spots) }
  return set
end)

---Dice faces with pips, the empty place of a die not rolled, and the marks under a die held or
---not, shared by the dice games.
local M = {}

---The pack holding exactly keys, so each game's pack draws its own keys and no others.
---@param keys string[]
---@return games.AssetPack
function M.pack(keys)
  return build.pack(keys, GLYPHS, ART)
end

return M
