local compose = require("games.ui.compose")
local float = require("games.ui.float")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

local T = {}

function T.a_float_taller_or_wider_than_the_editor_is_cut_to_it_less_the_margin()
  local rows = {}
  for i = 1, 100 do
    rows[i] = compose.text(("%d %s"):format(i, ("x"):rep(200)))
  end
  local win = float.open("stats", compose.col(rows))
  local config = vim.api.nvim_win_get_config(win)
  vim.api.nvim_win_close(win, true)
  assert_eq({ vim.o.columns - 4, vim.o.lines - 4 }, { config.width, config.height })
end

function T.a_small_float_fits_its_lines_and_title()
  local win = float.open("a long title here", compose.text("ab"))
  local config = vim.api.nvim_win_get_config(win)
  vim.api.nvim_win_close(win, true)
  assert_eq({ #"a long title here", 1 }, { config.width, config.height })
end

return T
