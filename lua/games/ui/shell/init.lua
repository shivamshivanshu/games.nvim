local assets = require("games.ui.assets")
local compose = require("games.ui.compose")
local describe = require("games.core.describe")
local games = require("games")
local header = require("games.ui.shell.header")
local key_hints = require("games.ui.widgets.key_hints")
local notify = require("games.core.notify")
local paint = require("games.ui.paint")
local players = require("games.core.players")
local registry = require("games.core.registry")
local shell_bindings = require("games.ui.shell.bindings")
local shell_windows = require("games.ui.shell.windows")
local timer = require("games.core.timer")
local types = require("games.types")

---@class games.Board
---@field buf integer
---@field client games.Client
---@field module games.GameModule
---@field me games.UserID
---@field view games.ViewOptions
---@field update games.GameUpdate
---@field options games.OpenOptions it was opened with
---@field clocks? games.ClockColumn where its header draws the clocks; nil without a clock
---@field bindings table<string, games.KeyBinding> by key; each is mapped in the buffer
---@field text_area? games.Size the window space it was last fitted to, nil until shown
---@field dirty boolean holds an update it has not drawn
---@field painted? games.Painted what its last paint drew; nil while one is underway or after one raised
---@field request_refit fun() refits it on the next tick, once however often it is asked
---@field request_render fun() draws it on the next tick while it is dirty and shown, once however often it is asked
---@field seats integer players in its first update, which size the header for its whole life

---The header always has a status line, blank without a banner, so a banner never moves the board.
local STATUS_LINES = 1
local BODY_BLANK_LINES = 2

local NAMESPACE = vim.api.nvim_create_namespace("games.shell")
local AUGROUP = vim.api.nvim_create_augroup("games.shell", { clear = true })

local M = {}

---@type table<integer, games.Board> buffer -> board
local boards = {}
---@type table<games.Client, true>
local subscribed_clients = setmetatable({}, { __mode = "k" })

---@type fun(board: games.Board)
local render_board

---@type games.ShellOps
local SHELL_OPS = {
  redraw = function(board)
    render_board(board)
  end,
  open_next = function(board)
    M.open_next(board.client, board.update.game_id, board.options)
  end,
}

---What the game layout may fill of a text area, after the header, blank lines and footer.
---@param seats integer player lines in the header
---@param footer_lines integer
---@param text_area games.Size|nil
---@return games.Size|nil
local function layout_space(seats, footer_lines, text_area)
  if not text_area then
    return nil
  end
  local chrome = seats + STATUS_LINES + BODY_BLANK_LINES + footer_lines
  return { w = text_area.w, h = text_area.h - chrome }
end

---Whether a flippable module's board draws flipped: its bottom seat's player sees it so, and
---either player sees it the other way once they turn it.
---@param update games.GameUpdate
---@param module games.GameModule
---@param view games.ViewOptions
---@param me games.UserID
---@return boolean
local function is_flipped(update, module, view, me)
  local flippable = module.flippable
  if not flippable then
    return false
  end
  local bottom_seat = flippable.bottom_seat(update)
  local sees_it_flipped = bottom_seat ~= nil and players.seat_of_user(update, me) == bottom_seat
  return sees_it_flipped ~= view.flipped
end

---@param update games.GameUpdate
---@param module games.GameModule
---@param view games.ViewOptions
---@param me games.UserID
---@param elapsed_ms? integer time since the clock's as_of_ms, charged to the running clock
---@param display? games.Display nil draws the smallest board
---@param catalogued? table<games.ActionKind, games.ActionSpec> the actions the server lists, by kind; nil, until they are known, binds none
---@return games.Rendered
---@return integer header_height lines owned by the header
---@return games.KeyBinding[] bindings the footer lists
---@return games.ClockColumn|nil clocks where the header draws the clocks
function M.render(update, module, view, me, elapsed_ms, display, catalogued)
  assert(
    view.art or not module.asset_keys,
    ("games: %s draws from an asset pack, and none is loaded"):format(update.game)
  )
  local measured = display or {}
  local bindings = shell_bindings.bindings_for(update, module, view, me, catalogued, SHELL_OPS)
  local footer = key_hints(bindings, measured.text_area and measured.text_area.w)
  local seats = measured.seats or #update.players
  local ctx = vim.tbl_extend("force", view, {
    me = me,
    space = layout_space(seats, compose.measure(footer).h, measured.text_area),
    ambiwidth = measured.ambiwidth or types.DEFAULT_AMBIWIDTH,
    flip = is_flipped(update, module, view, me),
  }) --[[@as games.LayoutCtx]]
  local header_node, clocks =
    header.layout(update, me, elapsed_ms or 0, header.extras_column(update, module, ctx))
  local rendered = compose.render(compose.col({
    header_node,
    compose.text(""),
    module.layout(update, ctx),
    compose.text(""),
    footer,
  }))
  return rendered, compose.measure(header_node).h, bindings, clocks
end

---@param board games.Board
---@return integer
local function elapsed_ms(board)
  return describe.clock_elapsed_ms(board.update, timer.unix_ms())
end

render_board = function(board)
  local rendered, _, bindings, clocks =
    M.render(board.update, board.module, board.view, board.me, elapsed_ms(board), {
      text_area = board.text_area,
      ambiwidth = vim.o.ambiwidth,
      seats = board.seats,
    }, board.client:action_specs(board.update.game))
  local painted = board.painted
  board.painted = nil
  board.painted = paint.update(board.buf, NAMESPACE, painted, rendered)
  board.clocks = clocks
  board.dirty = false
  shell_bindings.map_keys(board, bindings)
end

---Repaints only the clocks, or the whole board once the clock column changes width or the board
---holds an update it has not drawn.
---@param board games.Board
local function repaint_clocks(board)
  local clocks = board.clocks
  local cells = header.clock_cells(board.update, elapsed_ms(board))
  if board.dirty or not (clocks and header.widest_clock(cells) == clocks.width) then
    render_board(board)
    return
  end
  for i, cell in ipairs(cells) do
    header.paint_clock(board.buf, NAMESPACE, i - 1, clocks.cols[i], clocks.width, cell)
  end
end

---Fits the board to the windows of the current tabpage that show it, re-rendering and scrolling
---them to the top only when their space changed or the board is dirty.
---@param board games.Board
local function refit(board)
  local wins = shell_windows.windows_in_tab(board.buf)
  if #wins == 0 then
    return
  end
  vim.iter(wins):each(shell_windows.prepare_window)
  local text_area = shell_windows.shared_text_area(wins)
  if board.dirty or not vim.deep_equal(text_area, board.text_area) then
    board.text_area = text_area
    render_board(board)
    vim.iter(wins):each(shell_windows.scroll_to_top)
  end
end

---@param board games.Board
local function request_refit(board)
  board.request_refit()
end

---@return games.Board[] boards with a window in the current tabpage
local function boards_in_tab()
  return vim.tbl_filter(function(board)
    return #shell_windows.windows_in_tab(board.buf) > 0
  end, boards)
end

vim.api.nvim_create_autocmd({ "WinResized", "VimResized", "WinEnter", "TabEnter" }, {
  group = AUGROUP,
  callback = function()
    vim.iter(boards_in_tab()):each(request_refit)
  end,
})

vim.api.nvim_create_autocmd("OptionSet", {
  group = AUGROUP,
  pattern = "ambiwidth",
  callback = function()
    for _, board in pairs(boards) do
      board.dirty = true
    end
    vim.iter(boards_in_tab()):each(request_refit)
  end,
})

---@return games.Board[]
local function ticking_boards()
  return vim.tbl_filter(function(board)
    return describe.has_running_clock(board.update) and shell_windows.is_shown(board.buf)
  end, boards)
end

local clock_ticker = timer.ticker(timer.CLOCK_TICK_MS, function()
  vim.iter(ticking_boards()):each(repaint_clocks)
end)

local function sync_clock_timer()
  clock_ticker.sync(#ticking_boards() > 0)
end

---@param board games.Board
---@param update games.GameUpdate
local function apply_update(board, update)
  board.update = update
  board.dirty = true
  if shell_windows.is_shown(board.buf) then
    board.request_render()
  end
  sync_clock_timer()
end

---@param client games.Client
---@param game_id games.GameID
---@return games.Board|nil
local function board_for(client, game_id)
  return vim.iter(vim.tbl_values(boards)):find(function(board)
    return board.client == client and board.update.game_id == game_id
  end)
end

---Whether some window shows the board of game_id.
---@param game_id games.GameID
---@return boolean
function M.is_visible(game_id)
  return vim.iter(vim.tbl_values(boards)):any(function(board)
    return board.update.game_id == game_id and shell_windows.is_shown(board.buf)
  end)
end

---The board buf shows, while it is open.
---@param buf integer
---@return games.Board|nil
function M.board(buf)
  return boards[buf]
end

---@param client games.Client
local function subscribe(client)
  if subscribed_clients[client] then
    return
  end
  subscribed_clients[client] = true
  client:on_update(function(update)
    local board = board_for(client, update.game_id)
    if board then
      apply_update(board, update)
    end
  end)
  client:on_catalog(function()
    for _, board in pairs(boards) do
      if board.client == client then
        render_board(board)
      end
    end
  end)
end

---@param buf integer
local function forget_board(buf)
  boards[buf] = nil
  vim.api.nvim_clear_autocmds({ group = AUGROUP, buffer = buf })
  -- A buffer cannot be wiped from its own unload autocmd, so the husk goes next tick.
  vim.schedule(function()
    if vim.api.nvim_buf_is_valid(buf) then
      vim.api.nvim_buf_delete(buf, { force = true })
    end
    sync_clock_timer()
  end)
end

---@param board games.Board
local function watch_buffer(board)
  local buf = board.buf
  vim.api.nvim_create_autocmd("BufWinEnter", {
    group = AUGROUP,
    buffer = buf,
    callback = function()
      if vim.api.nvim_win_get_buf(0) == buf then
        shell_windows.scroll_to_top(vim.api.nvim_get_current_win())
      end
      request_refit(board)
      sync_clock_timer()
    end,
  })
  vim.api.nvim_create_autocmd("BufWinLeave", {
    group = AUGROUP,
    buffer = buf,
    callback = function()
      vim.schedule(sync_clock_timer)
    end,
  })
  vim.api.nvim_create_autocmd({ "BufUnload", "BufDelete" }, {
    group = AUGROUP,
    buffer = buf,
    once = true,
    callback = function()
      forget_board(buf)
    end,
  })
end

---@param game games.GameType
---@param module games.GameModule
---@param opts games.OpenOptions
---@return games.ViewOptions
local function view_for(game, module, opts)
  return {
    glyphs = opts.glyphs,
    flipped = false,
    max_cell_height = opts.max_cell_height,
    art = assets.module_art(game, module, opts),
    grid_rows = {},
  }
end

---@type fun(client: games.Client, update: games.GameUpdate, me: games.UserID, opts?: games.OpenOptions): integer|nil
local open_update

---Opens the board of game_id, fetching its snapshot first when the client holds none, as for a
---finished game it no longer caches; then the board opens once the snapshot arrives.
---@param client games.Client
---@param game_id games.GameID
---@param opts? games.OpenOptions defaults to the user's config
---@return integer|nil buf nil when it cannot open now
function M.open(client, game_id, opts)
  local me = notify.require_me(client)
  if not me then
    return nil
  end
  local existing = board_for(client, game_id)
  if existing then
    shell_windows.focus(existing.buf)
    return existing.buf
  end
  local update = client:game(game_id)
  if update then
    return open_update(client, update, me, opts)
  end
  client:game_snapshot({ game_id = game_id }, function(err, snapshot)
    if err then
      notify.say(("game %s: %s"):format(game_id, err.message), vim.log.levels.ERROR)
    elseif snapshot and not board_for(client, game_id) then
      open_update(client, snapshot, me, opts)
    end
  end)
  return nil
end

open_update = function(client, update, me, opts)
  local game_id = update.game_id
  local module = registry.game_module(update.game)
  if not module then
    notify.say(("no board for game type %s"):format(update.game), vim.log.levels.ERROR)
    return nil
  end
  shell_bindings.check_action_keys(module)

  subscribe(client)
  local buf = vim.api.nvim_create_buf(true, true)
  local name = ("games://%s/%s/%s"):format(
    update.game,
    describe.opponents_text(update, me, ","),
    describe.short_id(game_id)
  )
  vim.api.nvim_buf_set_name(buf, name)
  vim.bo[buf].modifiable = false
  vim.bo[buf].undolevels = -1

  local options = opts or games.open_options()
  ---@type games.Board
  local board = {
    buf = buf,
    client = client,
    module = module,
    me = me,
    options = options,
    view = view_for(update.game, module, options),
    update = update,
    bindings = {},
    dirty = false,
    request_refit = timer.coalesced(function()
      local open = boards[buf]
      if open then
        refit(open)
      end
    end),
    request_render = timer.coalesced(function()
      local open = boards[buf]
      if open and open.dirty and shell_windows.is_shown(buf) then
        render_board(open)
      end
    end),
    seats = #update.players,
  }
  boards[buf] = board
  watch_buffer(board)
  shell_windows.show_tiled(buf)
  refit(board)
  sync_clock_timer()
  return buf
end

---@param client games.Client
---@param after_game_id? games.GameID
---@param opts? games.OpenOptions defaults to the user's config
---@return integer|nil buf
function M.open_next(client, after_game_id, opts)
  local me = notify.require_me(client)
  if not me then
    return nil
  end
  local waiting = vim.tbl_filter(function(update)
    return update.game_id ~= after_game_id and players.is_to_act(update, me)
  end, client:games())
  if #waiting == 0 then
    notify.say("no game is waiting for your move")
    return nil
  end
  table.sort(waiting, function(a, b)
    return a.game_id < b.game_id
  end)
  local next_update = vim.iter(waiting):find(function(update)
    return after_game_id ~= nil and update.game_id > after_game_id
  end) or waiting[1]
  return M.open(client, next_update.game_id, opts)
end

return M
