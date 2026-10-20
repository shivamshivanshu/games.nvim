local helpers = require("helpers")
local theme = require("games.ui.theme")

local assert_eq = helpers.assert_eq

return {
  seat_highlights_link_to_distinct_groups_and_repeat_after_four = function()
    theme.setup()
    local targets = {}
    for i = 1, 4 do
      local target = vim.api.nvim_get_hl(0, { name = theme.seat_hl(i), link = true }).link
      assert(target and not targets[target], ("seat %d links to %s"):format(i, vim.inspect(target)))
      targets[target] = true
    end
    assert_eq(theme.seat_hl(1), theme.seat_hl(5))
  end,
}
