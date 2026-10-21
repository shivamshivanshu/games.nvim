local actions = require("games.ui.actions")
local helpers = require("helpers")
local keys = require("games.ui.keys")
local notation = require("games.ui.notation")

local assert_eq = helpers.assert_eq

---@param action games.GameAction
---@param view table
---@return boolean
local function available(action, view)
  return action.available({ view = view })
end

local T = {}

function T.per_value_binds_each_value_s_digits_and_offers_it_while_legal()
  local drops = actions.per_value({
    kind = "drop",
    field = "column",
    values = { 1, 2 },
    legal = function(view)
      return view.legal_columns
    end,
  })
  assert_eq(
    { "1", "2" },
    vim.tbl_map(function(action)
      return action.key
    end, drops)
  )
  assert_eq({ kind = "drop", column = "2" }, drops[2].action)
  assert_eq("drop", drops[2].label)
  assert_eq(
    { true, false },
    vim.tbl_map(function(action)
      return available(action, { legal_columns = { 1 } })
    end, drops)
  )
end

function T.directions_bind_hjkl_with_arrows_when_asked_and_offer_what_movable_allows()
  local moves = actions.directions({
    kind = "move",
    arrows = true,
    movable = function(view, direction)
      return vim.tbl_contains(view.movable, direction)
    end,
  })
  assert_eq(
    { "h", "j", "k", "l" },
    vim.tbl_map(function(action)
      return action.key
    end, moves)
  )
  assert_eq({ "<Left>" }, moves[1].aliases)
  assert_eq({ kind = "move", direction = keys.Direction.UP }, moves[3].action)
  assert_eq(true, available(moves[3], { movable = { "up" } }))
  assert_eq(false, available(moves[1], { movable = { "up" } }))

  local plain = actions.directions({
    kind = "move",
    movable = function()
      return true
    end,
  })
  assert_eq(nil, plain[1].aliases)
end

function T.notation_names_files_by_letter_and_ranks_by_number()
  assert_eq("abcdefgh", notation.letters(8))
  assert_eq({ "1", "2", "3" }, notation.numbers(3))
end

return T
