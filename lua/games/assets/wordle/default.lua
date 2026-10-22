local build = require("games.assets.shared.build")
local letters = require("games.assets.shared.letters")
local wordle = require("games.games.wordle")

---@type table<string, games.LetterMarker> each feedback key's marker -> how its letters draw
local FEEDBACK_MARKERS = {
  [wordle.Mark.HIT] = letters.Marker.HIT,
  [wordle.Mark.PRESENT] = letters.Marker.PRESENT,
}

---@param empty string
---@return games.ArtSet
local function glyph_set(empty)
  local lines = letters.by_letter(letters.glyph, FEEDBACK_MARKERS)
  lines.empty = empty
  return build.glyph_set(lines, wordle.glyph_width)
end

---Wordle tiles: the letter, framed [A] for a hit and (A) when present so feedback never rests on
---colour alone; block letters 3 and 6 lines tall, with a bar or a dot on the right for the same.
---@type games.AssetPack
return build.default_pack({
  glyphs = {
    unicode = glyph_set(" · "),
    ascii = glyph_set(" . "),
  },
  art = letters.by_letter(letters.art, FEEDBACK_MARKERS),
})
