---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = 1, lines = { line } }
end

---@type table<games.Glyphs, games.ArtSet>
local GLYPHS = {
  unicode = {
    face1 = { glyph("⚀") },
    face2 = { glyph("⚁") },
    face3 = { glyph("⚂") },
    face4 = { glyph("⚃") },
    face5 = { glyph("⚄") },
    face6 = { glyph("⚅") },
    -- "·" and "▲" are two cells wide under ambiwidth=double, where the ascii ones stand in.
    none = { glyph("·") },
    held = { glyph("▲") },
    free = { glyph(" ") },
  },
  ascii = {
    face1 = { glyph("1") },
    face2 = { glyph("2") },
    face3 = { glyph("3") },
    face4 = { glyph("4") },
    face5 = { glyph("5") },
    face6 = { glyph("6") },
    none = { glyph(".") },
    held = { glyph("^") },
    free = { glyph(" ") },
  },
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
    end, vim.split(row, ""))
    lines[#lines + 1] = "▕" .. table.concat(pips) .. "▏"
  end
  lines[#lines + 1] = "▕" .. ("▁"):rep(inner) .. "▏"
  return { h = #lines, w = inner + 2, lines = lines }
end

---Dice at cell height 5 and up, two columns narrower than the cell so dice side by side stay apart;
---shorter cells show the glyph.
---@type games.ArtSet
local ART = vim.iter(PIPS):fold({}, function(set, key, spots)
  set[key] = { outlined(spots) }
  return set
end)

---@param set games.ArtSet
---@param keys string[]
---@return games.ArtSet
local function only(set, keys)
  local out = {}
  for _, key in ipairs(keys) do
    out[key] = set[key]
  end
  return out
end

---Dice faces with pips, the empty place of a die not rolled, and the marks under a die held or
---not, shared by the dice games.
local M = {}

---The pack holding exactly keys, so each game's pack draws its own keys and no others.
---@param keys string[]
---@return games.AssetPack
function M.pack(keys)
  return {
    name = "default",
    glyphs = { unicode = only(GLYPHS.unicode, keys), ascii = only(GLYPHS.ascii, keys) },
    art = only(ART, keys),
  }
end

return M
