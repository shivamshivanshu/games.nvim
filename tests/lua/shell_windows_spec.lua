local helpers = require("helpers")
local hub = require("fixtures.hub")
local shell = require("games.ui.shell")
local windows = require("games.ui.windows")

local assert_eq = helpers.assert_eq

---@return integer
local function open_float()
  local buf = vim.api.nvim_create_buf(false, true)
  vim.bo[buf].bufhidden = "wipe"
  return vim.api.nvim_open_win(
    buf,
    true,
    { relative = "editor", width = 10, height = 2, row = 0, col = 0 }
  )
end

---Runs fn from a single tiled window, then closes every other window and wipes the boards.
---@param fn fun(client: games.Client, tiled: integer)
local function with_one_tiled_window(fn)
  vim.cmd("silent! only!")
  local client = hub.client()
  local ok, err = xpcall(fn, debug.traceback, client, vim.api.nvim_get_current_win())
  for _, win in ipairs(vim.api.nvim_list_wins()) do
    if not windows.is_tiled(win) then
      vim.api.nvim_win_close(win, true)
    end
  end
  vim.cmd("silent! only!")
  vim.cmd("enew")
  for _, buf in ipairs(vim.api.nvim_list_bufs()) do
    if vim.startswith(vim.api.nvim_buf_get_name(buf), "games://") then
      vim.api.nvim_buf_delete(buf, { force = true })
    end
  end
  assert(ok, err)
end

---@param win integer
local function assert_board_in(win)
  assert_eq(win, vim.api.nvim_get_current_win())
  assert(vim.startswith(vim.api.nvim_buf_get_name(0), "games://"), vim.api.nvim_buf_get_name(0))
end

local T = {}

function T.a_board_shown_only_in_a_float_reopens_in_a_tiled_window()
  with_one_tiled_window(function(client, tiled)
    local board = assert(shell.open(client, hub.YOUR_MOVE.game_id))
    vim.cmd("enew")
    local float = vim.api.nvim_open_win(
      board,
      false,
      { relative = "editor", width = 30, height = 10, row = 0, col = 0 }
    )
    shell.open(client, hub.YOUR_MOVE.game_id)
    assert_board_in(tiled)
    assert(vim.api.nvim_win_is_valid(float))
  end)
end

function T.from_a_float_over_a_float_a_board_opens_in_a_tiled_window()
  with_one_tiled_window(function(client, tiled)
    open_float()
    open_float()
    assert(
      not windows.is_tiled(vim.fn.win_getid(vim.fn.winnr("#"))),
      "the previous window is not a float"
    )
    shell.open(client, hub.YOUR_MOVE.game_id)
    assert_board_in(tiled)
  end)
end

function T.from_a_float_whose_previous_window_closed_a_board_opens_in_a_tiled_window()
  with_one_tiled_window(function(client, tiled)
    vim.cmd("split")
    local closing = vim.api.nvim_get_current_win()
    open_float()
    vim.api.nvim_win_close(closing, true)
    assert_eq(0, vim.fn.win_getid(vim.fn.winnr("#")))
    shell.open(client, hub.YOUR_MOVE.game_id)
    assert_board_in(tiled)
  end)
end

return T
