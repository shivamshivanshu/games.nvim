local build = require("games.assets.shared.build")
local discs = require("games.assets.shared.discs")

---@type table<string, string> each connect4 key -> the shared disc it draws as
local DISC_OF_KEY = { red = "black", yellow = "white", empty = "empty" }

---Connect4 discs: the shared ones, red solid and yellow hollow, so the seats differ by shape as
---well as colour.
---@type games.AssetPack
return build.renamed(discs.pack(vim.tbl_values(DISC_OF_KEY)), DISC_OF_KEY)
