local assets = require("games.ui.assets")
local compose = require("games.ui.compose")
local help = require("games.ui.shell.help")
local helpers = require("helpers")
local registry = require("games.core.registry")
local types = require("games.types")

local assert_eq = helpers.assert_eq

---@param game games.GameType
---@param entries games.LegendEntry[]
---@return string[]
local function entry_problems(game, entries)
  if #entries == 0 then
    return { ("%s: an empty legend"):format(game) }
  end
  local problems = {}
  for i, entry in ipairs(entries) do
    local all_text = vim.iter(entry.sample):all(function(node)
      return node.kind == "text"
    end)
    local shows_a_glyph = vim.iter(entry.sample):any(function(node)
      return node.str:find("%S") ~= nil
    end)
    if not (all_text and shows_a_glyph) or entry.meaning == "" then
      problems[#problems + 1] = ("%s: entry %d is %s"):format(game, i, vim.inspect(entry))
    end
  end
  return problems
end

local T = {}

function T.every_game_draws_a_legend_of_samples_and_meanings_in_both_glyph_modes()
  local problems = {}
  for _, info in ipairs(helpers.catalog()) do
    local module = assert(registry.game_module(info.id), info.id)
    for _, glyphs in ipairs({ types.Glyphs.ASCII, types.Glyphs.UNICODE }) do
      local ctx = {
        art = assets.module_art(info.id, module, { glyphs = glyphs }),
        ambiwidth = types.DEFAULT_AMBIWIDTH,
      }
      local entries = module.legend(ctx)
      vim.list_extend(problems, entry_problems(info.id, entries))
      compose.render(help.node(entries, {}))
    end
  end
  assert_eq({}, problems)
end

return T
