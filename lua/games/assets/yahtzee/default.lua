---Yahtzee dice: the shared faces, the place of a die not yet rolled, and the marks under each die.
---@type games.AssetPack
return require("games.assets.shared.dice").pack({
  "face1",
  "face2",
  "face3",
  "face4",
  "face5",
  "face6",
  "none",
  "held",
  "free",
})
