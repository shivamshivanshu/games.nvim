local assets = require("games.ui.assets")

---Builders of art entries and packs, shared by the asset packs.
local M = {}

---@param line string
---@param w integer the game's glyph width
---@return games.ArtEntry
function M.glyph(line, w)
  return { h = 1, w = w, lines = { line } }
end

---@param lines table<string, string> key -> its one-line glyph
---@param w integer the game's glyph width
---@return games.ArtSet
function M.glyph_set(lines, w)
  return vim.tbl_map(function(line)
    return { M.glyph(line, w) }
  end, lines)
end

---Its width counts characters, never display columns, so a pack is the same whatever 'ambiwidth'
---is in effect when it loads.
---@param lines string[] block-character lines, each as many characters as the first
---@return games.ArtEntry
function M.sprite(lines)
  return { h = #lines, w = vim.fn.strchars(lines[1]), lines = lines }
end

---@param set games.ArtSet
---@param keys string[]
---@return games.ArtSet
local function only(set, keys)
  return vim.iter(keys):fold({}, function(out, key)
    out[key] = set[key]
    return out
  end)
end

---The default pack holding exactly keys of glyphs and art, so a game sharing them draws its own
---keys and no others.
---@param keys string[]
---@param glyphs table<games.Glyphs, games.ArtSet>
---@param art games.ArtSet
---@return games.AssetPack
function M.pack(keys, glyphs, art)
  return {
    name = assets.DEFAULT_PACK,
    glyphs = vim.tbl_map(function(set)
      return only(set, keys)
    end, glyphs),
    art = only(art, keys),
  }
end

---pack with each key drawn as its source key.
---@param pack games.AssetPack
---@param source_of table<string, string> key -> the key of pack it draws as
---@return games.AssetPack
function M.renamed(pack, source_of)
  local function rekeyed(set)
    return vim.iter(source_of):fold({}, function(out, key, source)
      out[key] = set[source]
      return out
    end)
  end
  return { name = pack.name, glyphs = vim.tbl_map(rekeyed, pack.glyphs), art = rekeyed(pack.art) }
end

return M
