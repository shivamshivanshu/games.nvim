local helpers = require("helpers")
local highlights = require("games.ui.highlights")

local assert_eq = helpers.assert_eq

local REPO_ROOT = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")
---A quoted group name, unless it is the prefix of a concatenation such as "GamesSeat" .. i.
local GROUP_LITERAL = '"(Games%u%w*)"()'
local CONCATENATION = "^%s*%.%."

---Each "Games*" literal in lines that names no group, as "<where>:<line> <name>".
---@param lines string[]
---@param where string
---@return string[]
local function unknown_groups(lines, where)
  local unknown = {}
  for number, line in ipairs(lines) do
    for name, after in line:gmatch(GROUP_LITERAL) do
      if not highlights.ROLES[name] and not line:sub(after):find(CONCATENATION) then
        unknown[#unknown + 1] = ("%s:%d %s"):format(where, number, name)
      end
    end
  end
  return unknown
end

local T = {}

function T.a_misspelt_group_name_is_caught()
  assert_eq(
    { "synthetic:2 GamesKye" },
    unknown_groups(
      { 'hl = "GamesKey"', 'hl = "GamesKye"', 'local hl = "GamesSeat" .. i' },
      "synthetic"
    )
  )
end

function T.every_group_name_the_plugin_spells_out_is_defined()
  local unknown = {}
  for _, file in ipairs(vim.fn.globpath(REPO_ROOT .. "/lua", "**/*.lua", false, true)) do
    vim.list_extend(unknown, unknown_groups(vim.fn.readfile(file), file:sub(#REPO_ROOT + 2)))
  end
  assert_eq({}, unknown)
end

return T
