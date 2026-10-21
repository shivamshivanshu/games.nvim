---Gomoku stones: the shared discs, without a legal-move marker.
---@type games.AssetPack
return require("games.assets.shared.discs").pack(require("games.games.gomoku").asset_keys)
