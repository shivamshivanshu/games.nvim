---Reversi discs: the shared ones, with the marker on each cell the viewer may play.
---@type games.AssetPack
return require("games.assets.shared.discs").pack(require("games.games.reversi").asset_keys)
