---Yahtzee dice: the shared faces, the place of a die not yet rolled, and the marks under each die.
---@type games.AssetPack
return require("games.assets.shared.dice").pack(require("games.games.yahtzee").asset_keys)
