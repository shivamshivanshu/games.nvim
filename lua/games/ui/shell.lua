local assets = require("games.ui.assets")
local compose = require("games.ui.compose")
local describe = require("games.core.describe")
local games = require("games")
local key_hints = require("games.ui.widgets.key_hints")
local notify = require("games.core.notify")
local paint = require("games.ui.paint")
local theme = require("games.ui.theme")
local players = require("games.core.players")
local prompt = require("games.ui.prompt")
local registry = require("games.core.registry")
local text_util = require("games.ui.text")
local timer = require("games.core.timer")
local types = require("games.types")
local windows = require("games.ui.windows")

local Control = types.Control

---@class games.ViewOptions per-buffer presentation choices
---@field glyphs games.Glyphs
---@field flipped boolean perspective toggled away from the player's default
---@field max_cell_height? integer
---@field art? games.ArtSet the game's asset pack in this glyph mode
---@field cursor? games.GridCursor where the game's cursor keys left the cursor; nil until moved

---@class games.LayoutCtx: games.ViewOptions
---@field me games.UserID
---@field space? games.Size what the board and its panels may fill; nil draws the smallest board
---@field ambiwidth games.Ambiwidth

---@class games.DescribeCtx how event text outside a board draws
---@field art? games.ArtSet the game's asset pack in the user's glyph mode
---@field ambiwidth games.Ambiwidth

---@class games.Display what the shell measured for one render
---@field text_area? games.Size the window space to fill; nil draws the smallest board
---@field ambiwidth? games.Ambiwidth the 'ambiwidth' in effect, default "single"
---@field seats? integer player lines the header reserves, held from the board's first update

---@class games.GameInput
---@field kind games.ActionKind of the action parse builds
---@field prompt string
---@field parse fun(text: string): games.Action
---@field available? fun(update: games.GameUpdate): boolean whether it applies now; default always

---@class games.GameAction
---@field key string
---@field label string|fun(update: games.GameUpdate): string a function reads the label off the update, e.g. "call 40"
---@field action? games.Action
---@field input? games.GameInput prompts for text and sends the action it parses, in place of action
---@field act? fun(update: games.GameUpdate, view: games.ViewOptions): games.Action|nil builds the action from the board's view options, such as its cursor, in place of action; nil binds nothing
---@field available? fun(update: games.GameUpdate): boolean whether the entry applies now; default always
---@field local_fn? fun(view: games.ViewOptions, update: games.GameUpdate): games.ViewOptions changes only the board's view options, which may follow the board's latest update

---@class games.GameModule
---@field layout fun(update: games.GameUpdate, ctx: games.LayoutCtx): games.Node
---@field input? games.GameInput
---@field actions? games.GameAction[]
---@field describe_event? fun(event: games.Event, update: games.GameUpdate, ctx: games.DescribeCtx): string|nil
---@field asset_keys? string[] keys every asset pack of the game must draw
---@field glyph_width? integer display columns of a one-line glyph, its cell_width at h = 1; default 1
---@field picture_keys? string[] keys its packs draw as whole pictures beside the board
---@field player_extra? fun(update: games.GameUpdate, seat: games.Seat, ctx: games.LayoutCtx): games.TextNode[]|nil what the header shows after the clock on seat's line

---@class games.OpenOptions
---@field glyphs games.Glyphs
---@field max_cell_height? integer
---@field assets? table<games.GameType, string> asset pack per game, default "default"

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
---@field refit_pending boolean
---@field seats integer players in its first update, which size the header for its whole life

---Where the header draws each player's clock, which a clock tick rewrites in place. Clock text is
---ASCII, so its bytes are its display columns.
---@class games.ClockColumn
---@field cols integer[] byte column of each player's clock, by header line
---@field width integer columns of the widest clock, to which every clock is padded

---@class (private) games.ClockCell
---@field text string
---@field hl? games.Hl

---@class games.KeyBinding: games.KeyHint
---@field run fun(board: games.Board)

---@class games.ControlKey
---@field control games.Control
---@field key string
---@field wording fun(update: games.GameUpdate): games.ControlWording

---@param label string
---@return fun(): games.ControlWording the same wording for every game
local function labelled(label)
  local wording = { label = label }
  return function()
    return wording
  end
end

---@type games.ControlKey[]
local CONTROL_KEYS = {
  { control = Control.RESIGN, key = "R", wording = describe.resign_wording },
  { control = Control.OFFER_DRAW, key = "D", wording = labelled("offer draw") },
  { control = Control.DECLINE_DRAW, key = "X", wording = labelled("decline draw") },
  { control = Control.ABORT, key = "A", wording = labelled("abort") },
}
local INPUT_KEY = "m"
local NEXT_GAME_KEY = "]g"
local CLOSE_KEY = "q"
local CLOCK_TICK_MS = 1000
local LOW_CLOCK_MS = 10000
local LOW_CLOCK_HL = "GamesClockLow"
local HEADER_COLUMN_GAP = 2
---The header always has a status line, blank without a banner, so a banner never moves the board.
local STATUS_LINES = 1
local BODY_CHROME_LINES = 3
---@type games.Ambiwidth
local DEFAULT_AMBIWIDTH = "single"

---@type table<string, true> keys the shell owns whether or not they are active
local CORE_KEYS = vim.iter(CONTROL_KEYS):fold(
  { [INPUT_KEY] = true, [NEXT_GAME_KEY] = true, [CLOSE_KEY] = true },
  function(keys, spec)
    return vim.tbl_extend("error", keys, { [spec.key] = true })
  end
)

---@type table<games.GameModule, true>
local checked_modules = setmetatable({}, { __mode = "k" })

local NAMESPACE = vim.api.nvim_create_namespace("games.shell")
local AUGROUP = vim.api.nvim_create_augroup("games.shell", { clear = true })

local M = {}

---@type table<integer, games.Board> buffer -> board
local boards = {}
---@type table<games.Client, true>
local subscribed_clients = setmetatable({}, { __mode = "k" })
---@type uv.uv_timer_t|nil
local clock_timer

---@param update games.GameUpdate
---@param user games.UserID
---@param me games.UserID
---@return games.Node
local function turn_marker(update, user, me)
  if not players.is_to_act(update, user) then
    return compose.text("")
  end
  if user == me then
    return compose.row({ compose.text("← "), compose.text(" your move ", "GamesYourTurn") })
  end
  return compose.text("← to move", "GamesMuted")
end

---@param update games.GameUpdate with a clock
---@param elapsed_ms integer
---@return games.ClockCell[] by player
local function clock_cells(update, elapsed_ms)
  return vim.tbl_map(function(player)
    local remaining_ms = describe.clock_remaining_ms(update, player.user, elapsed_ms)
    return {
      text = describe.clock_text(remaining_ms),
      hl = remaining_ms < LOW_CLOCK_MS and LOW_CLOCK_HL or nil,
    }
  end, update.players)
end

---@param cells games.ClockCell[]
---@return integer
local function widest_clock(cells)
  return vim.iter(cells):fold(0, function(width, cell)
    return math.max(width, #cell.text)
  end)
end

---@param update games.GameUpdate
---@param me games.UserID
---@return games.Node|nil
local function status_banner(update, me)
  if update.result then
    local hl = theme.outcome_hl(describe.outcome(update.result, me))
    return compose.text(describe.result_text(update.result), hl)
  end
  if update.draw_agreed and #update.draw_agreed > 0 then
    return compose.text("Draw offered by " .. table.concat(update.draw_agreed, ", "))
  end
end

---Each player's extra segments, one line per player; nil when the game has none for any player.
---@param update games.GameUpdate
---@param module games.GameModule
---@param ctx games.LayoutCtx
---@return games.Node|nil
local function extras_column(update, module, ctx)
  local player_extra = module.player_extra
  if not player_extra then
    return nil
  end
  local by_player = vim.tbl_map(function(player)
    return player_extra(update, player.seat, ctx) or false
  end, update.players)
  if not vim.iter(by_player):any(function(segments)
    return segments ~= false
  end) then
    return nil
  end
  return compose.col(vim.tbl_map(function(segments)
    return compose.row(segments or { compose.text("") })
  end, by_player))
end

---@param update games.GameUpdate
---@param me games.UserID
---@param elapsed_ms integer
---@param extras? games.Node drawn after the clocks, a line per player
---@return games.Rendered
---@return games.ClockColumn|nil
local function render_header(update, me, elapsed_ms, extras)
  ---@param cell fun(player: games.PlayerSeat, i: integer): games.Node
  ---@return games.Node
  local function column(cell)
    return compose.col(vim
      .iter(ipairs(update.players))
      :map(function(i, player)
        return cell(player, i)
      end)
      :totable())
  end
  local columns = {
    column(function(player, i)
      return compose.text(player.user, theme.seat_hl(i))
    end),
    compose.pad(
      HEADER_COLUMN_GAP,
      column(function(player)
        return compose.text(player.seat, "GamesMuted")
      end)
    ),
  }
  local clocks
  if update.clock then
    local cells = clock_cells(update, elapsed_ms)
    clocks = {
      cols = vim.tbl_map(function(line)
        return #line + HEADER_COLUMN_GAP
      end, compose.render(compose.row(columns)).lines),
      width = widest_clock(cells),
    }
    columns[#columns + 1] = compose.pad(
      HEADER_COLUMN_GAP,
      column(function(_, i)
        return compose.text(cells[i].text, cells[i].hl)
      end)
    )
  end
  if extras then
    columns[#columns + 1] = compose.pad(HEADER_COLUMN_GAP, extras)
  end
  columns[#columns + 1] = compose.pad(
    HEADER_COLUMN_GAP,
    column(function(player)
      return turn_marker(update, player.user, me)
    end)
  )
  local header = compose.render(
    compose.col({ compose.row(columns), status_banner(update, me) or compose.text("") })
  )
  return header, clocks
end

---@param top games.Rendered
---@param bottom games.Rendered
---@return games.Rendered
local function stack(top, bottom)
  local lines = vim.list_extend(vim.list_extend({}, top.lines), bottom.lines)
  local extmarks = vim.list_extend({}, top.extmarks)
  for _, mark in ipairs(bottom.extmarks) do
    extmarks[#extmarks + 1] = vim.tbl_extend("force", mark, { row = mark.row + #top.lines })
  end
  return { lines = lines, extmarks = extmarks }
end

---@param board games.Board
---@return games.ReplyCallback
local function report_error(board)
  return function(err)
    if err then
      notify.say(
        ("%s: %s"):format(describe.title(board.update, board.me), err.message),
        vim.log.levels.ERROR
      )
    end
  end
end

---@param board games.Board
---@param game_action games.Action
local function act(board, game_action)
  board.client:game_act(
    { game_id = board.update.game_id, seq = board.update.seq, action = game_action },
    report_error(board)
  )
end

---@param board games.Board
---@param control games.Control
---@param wording games.ControlWording
local function send_control(board, control, wording)
  if wording.confirm and not prompt.confirm(wording.confirm) then
    return
  end
  board.client:game_control(
    { game_id = board.update.game_id, seq = board.update.seq, control = control },
    report_error(board)
  )
end

---@param board games.Board
---@param input games.GameInput
local function prompt_input(board, input)
  vim.ui.input({ prompt = input.prompt }, function(text)
    if text and vim.trim(text) ~= "" then
      act(board, input.parse(text))
    end
  end)
end

local function close_window()
  local tiled = vim.tbl_filter(windows.is_tiled, vim.api.nvim_tabpage_list_wins(0))
  if #tiled > 1 then
    vim.api.nvim_win_close(0, false)
  else
    vim.cmd.enew()
  end
end

---@type fun(board: games.Board)
local render_board

---@param module games.GameModule
local function check_action_keys(module)
  if checked_modules[module] then
    return
  end
  local seen = {}
  for _, entry in ipairs(module.actions or {}) do
    assert(
      not CORE_KEYS[entry.key],
      ("game action key %q is reserved by the shell"):format(entry.key)
    )
    assert(not seen[entry.key], ("game action key %q is bound twice"):format(entry.key))
    seen[entry.key] = true
  end
  checked_modules[module] = true
end

---Whether the catalogue lists kind; nothing is, until the catalogue is known.
---@param catalogued? table<games.ActionKind, true>
---@param kind games.ActionKind
---@return boolean
local function is_catalogued(catalogued, kind)
  return catalogued ~= nil and catalogued[kind] == true
end

---@param entry games.GameAction
---@param update games.GameUpdate
---@return string
local function label_of(entry, update)
  local label = entry.label
  if type(label) == "function" then
    return label(update)
  end
  return label
end

---@param entry games.GameAction|games.GameInput
---@param update games.GameUpdate
---@return boolean
local function is_available(entry, update)
  return not entry.available or entry.available(update)
end

---@param update games.GameUpdate
---@param module games.GameModule
---@param view games.ViewOptions
---@param catalogued? table<games.ActionKind, true> the action kinds the server lists for the game
---@return games.KeyBinding[]
local function bindings_for(update, module, view, catalogued)
  check_action_keys(module)
  local ongoing = update.result == nil
  local bindings = {}
  for _, spec in ipairs(CONTROL_KEYS) do
    if vim.tbl_contains(update.controls, spec.control) then
      local wording = spec.wording(update)
      bindings[#bindings + 1] = {
        key = spec.key,
        label = wording.label,
        run = function(board)
          send_control(board, spec.control, wording)
        end,
      }
    end
  end
  local input = module.input
  if
    ongoing
    and input
    and is_catalogued(catalogued, input.kind)
    and is_available(input, update)
  then
    bindings[#bindings + 1] = {
      key = INPUT_KEY,
      label = (input.prompt:gsub(":%s*$", ""):lower()),
      run = function(board)
        prompt_input(board, input)
      end,
    }
  end
  for _, entry in ipairs(module.actions or {}) do
    local local_fn, entry_input = entry.local_fn, entry.input
    local game_action = entry.act and entry.act(update, view) or entry.action
    local kind = game_action and game_action.kind or entry_input and entry_input.kind
    if local_fn and is_available(entry, update) then
      bindings[#bindings + 1] = {
        key = entry.key,
        label = label_of(entry, update),
        run = function(board)
          board.view = local_fn(board.view, board.update)
          render_board(board)
        end,
      }
    elseif ongoing and kind and is_catalogued(catalogued, kind) and is_available(entry, update) then
      bindings[#bindings + 1] = {
        key = entry.key,
        label = label_of(entry, update),
        run = function(board)
          if entry_input then
            prompt_input(board, entry_input)
          else
            act(board, game_action --[[@as games.Action]])
          end
        end,
      }
    end
  end
  bindings[#bindings + 1] = {
    key = NEXT_GAME_KEY,
    label = "next game",
    run = function(board)
      M.open_next(board.client, board.update.game_id, board.options)
    end,
  }
  bindings[#bindings + 1] = { key = CLOSE_KEY, label = "close", run = close_window }
  return bindings
end

---What the game layout may fill of a text area, after the header, blank lines and footer.
---@param seats integer player lines in the header
---@param text_area games.Size|nil
---@return games.Size|nil
local function layout_space(seats, text_area)
  if not text_area then
    return nil
  end
  local chrome = seats + STATUS_LINES + BODY_CHROME_LINES
  return { w = text_area.w, h = text_area.h - chrome }
end

---@param update games.GameUpdate
---@param module games.GameModule
---@param view games.ViewOptions
---@param me games.UserID
---@param elapsed_ms? integer time since the clock's as_of_ms, charged to the running clock
---@param display? games.Display nil draws the smallest board
---@param catalogued? table<games.ActionKind, true> action kinds the server lists; nil, until it is known, binds none
---@return games.Rendered
---@return integer header_height lines owned by the header
---@return games.KeyBinding[] bindings the footer lists
---@return games.ClockColumn|nil clocks where the header draws the clocks
function M.render(update, module, view, me, elapsed_ms, display, catalogued)
  local measured = display or {}
  local ctx = vim.tbl_extend("force", view, {
    me = me,
    space = layout_space(measured.seats or #update.players, measured.text_area),
    ambiwidth = measured.ambiwidth or DEFAULT_AMBIWIDTH,
  }) --[[@as games.LayoutCtx]]
  local header, clocks =
    render_header(update, me, elapsed_ms or 0, extras_column(update, module, ctx))
  local bindings = bindings_for(update, module, view, catalogued)
  local body = compose.render(compose.col({
    compose.text(""),
    module.layout(update, ctx),
    compose.text(""),
    key_hints(bindings),
  }))
  return stack(header, body), #header.lines, bindings, clocks
end

---@param buf integer
---@return boolean
local function is_shown(buf)
  return #vim.fn.win_findbuf(buf) > 0
end

---@param board games.Board
---@return integer
local function elapsed_ms(board)
  return describe.clock_elapsed_ms(board.update, timer.unix_ms())
end

---@param board games.Board
---@return table<games.ActionKind, true>|nil
local function catalogued_kinds(board)
  local info = board.client:game_info(board.update.game)
  if not info then
    return nil
  end
  return vim.iter(info.actions):fold({}, function(kinds, spec)
    kinds[spec.kind] = true
    return kinds
  end)
end

---@param board games.Board
---@param bindings games.KeyBinding[]
local function map_keys(board, bindings)
  local by_key = {}
  for _, binding in ipairs(bindings) do
    local key = binding.key
    by_key[key] = binding
    local mapped = board.bindings[key]
    if not (mapped and mapped.label == binding.label) then
      vim.keymap.set("n", key, function()
        board.bindings[key].run(board)
      end, { buffer = board.buf, nowait = true, desc = "games: " .. binding.label })
    end
  end
  for key in pairs(board.bindings) do
    if not by_key[key] then
      vim.keymap.del("n", key, { buffer = board.buf })
    end
  end
  board.bindings = by_key
end

render_board = function(board)
  local rendered, _, bindings, clocks =
    M.render(board.update, board.module, board.view, board.me, elapsed_ms(board), {
      text_area = board.text_area,
      ambiwidth = vim.o.ambiwidth,
      seats = board.seats,
    }, catalogued_kinds(board))
  paint(board.buf, NAMESPACE, 0, -1, rendered)
  board.clocks = clocks
  vim.b[board.buf].games_dirty = false
  map_keys(board, bindings)
end

---Rewrites one clock, which keeps its byte length, and its low-time highlight.
---@param buf integer
---@param row integer
---@param col integer
---@param width integer
---@param cell games.ClockCell
local function paint_clock(buf, row, col, width, cell)
  local stale = vim.api.nvim_buf_get_extmarks(
    buf,
    NAMESPACE,
    { row, col },
    { row, col + width - 1 },
    {}
  )
  for _, mark in ipairs(stale) do
    vim.api.nvim_buf_del_extmark(buf, NAMESPACE, mark[1])
  end
  local padded = cell.text .. text_util.spaces(width - #cell.text)
  paint.with_modifiable(buf, function()
    vim.api.nvim_buf_set_text(buf, row, col, row, col + width, { padded })
  end)
  if cell.hl then
    vim.api.nvim_buf_set_extmark(
      buf,
      NAMESPACE,
      row,
      col,
      { end_col = col + #cell.text, hl_group = cell.hl }
    )
  end
end

---Repaints only the clocks, or the whole board once the clock column changes width or the board
---holds an update it has not drawn.
---@param board games.Board
local function repaint_clocks(board)
  local clocks = board.clocks
  local cells = clock_cells(board.update, elapsed_ms(board))
  if vim.b[board.buf].games_dirty or not (clocks and widest_clock(cells) == clocks.width) then
    render_board(board)
    return
  end
  for i, cell in ipairs(cells) do
    paint_clock(board.buf, i - 1, clocks.cols[i], clocks.width, cell)
  end
end

---@param buf integer
---@return integer[]
local function windows_in_tab(buf)
  local tab = vim.api.nvim_get_current_tabpage()
  return vim.tbl_filter(function(win)
    return vim.api.nvim_win_get_tabpage(win) == tab
  end, vim.fn.win_findbuf(buf))
end

---@param win integer
local function prepare_window(win)
  local options = vim.wo[win][0]
  options.wrap = false
  options.number = false
  options.relativenumber = false
  options.signcolumn = "no"
  options.foldcolumn = "0"
end

---The text area every one of wins shows in full.
---@param wins integer[]
---@return games.Size
local function shared_text_area(wins)
  local infos = vim.tbl_map(function(win)
    return vim.fn.getwininfo(win)[1]
  end, wins)
  return {
    w = math.min(unpack(vim.tbl_map(function(info)
      return info.width - info.textoff
    end, infos))),
    h = math.min(unpack(vim.tbl_map(function(info)
      return info.height
    end, infos))),
  }
end

---@param win integer
local function scroll_to_top(win)
  vim.api.nvim_win_call(win, function()
    vim.fn.winrestview({ topline = 1, lnum = 1, col = 0 })
  end)
end

---Fits the board to the windows of the current tabpage that show it, re-rendering and scrolling
---them to the top only when their space changed or the board is dirty.
---@param board games.Board
local function refit(board)
  local wins = windows_in_tab(board.buf)
  if #wins == 0 then
    return
  end
  vim.iter(wins):each(prepare_window)
  local text_area = shared_text_area(wins)
  if vim.b[board.buf].games_dirty or not vim.deep_equal(text_area, board.text_area) then
    board.text_area = text_area
    render_board(board)
    vim.iter(wins):each(scroll_to_top)
  end
end

---Coalesces a burst of window events into one refit on the next tick.
---@param board games.Board
local function request_refit(board)
  if board.refit_pending then
    return
  end
  board.refit_pending = true
  vim.schedule(function()
    board.refit_pending = false
    if boards[board.buf] then
      refit(board)
    end
  end)
end

---@return games.Board[] boards with a window in the current tabpage
local function boards_in_tab()
  return vim.tbl_filter(function(board)
    return #windows_in_tab(board.buf) > 0
  end, vim.tbl_values(boards))
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
      vim.b[board.buf].games_dirty = true
    end
    vim.iter(boards_in_tab()):each(request_refit)
  end,
})

---@return games.Board[]
local function ticking_boards()
  return vim.tbl_filter(function(board)
    return board.update.clock ~= nil and board.update.clock.running ~= nil and is_shown(board.buf)
  end, vim.tbl_values(boards))
end

local function sync_clock_timer()
  local needed = #ticking_boards() > 0
  clock_timer = clock_timer or assert(vim.uv.new_timer())
  if needed and not clock_timer:is_active() then
    clock_timer:start(
      CLOCK_TICK_MS,
      CLOCK_TICK_MS,
      vim.schedule_wrap(function()
        vim.iter(ticking_boards()):each(repaint_clocks)
      end)
    )
  elseif not needed and clock_timer:is_active() then
    clock_timer:stop()
  end
end

---@param board games.Board
---@param update games.GameUpdate
local function apply_update(board, update)
  board.update = update
  if is_shown(board.buf) then
    render_board(board)
  else
    vim.b[board.buf].games_dirty = true
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
    return board.update.game_id == game_id and is_shown(board.buf)
  end)
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
        scroll_to_top(vim.api.nvim_get_current_win())
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

---Shows buf in the current window, or from a float such as the dashboard in the window it was
---opened over, so a board never takes a float's place.
---@param buf integer
local function show_tiled(buf)
  local current = vim.api.nvim_get_current_win()
  if not windows.is_tiled(current) then
    local previous = vim.fn.win_getid(vim.fn.winnr("#"))
    local tiled = previous ~= 0 and windows.is_tiled(previous) and previous
      or vim.iter(vim.api.nvim_tabpage_list_wins(0)):find(windows.is_tiled)
    vim.api.nvim_set_current_win(tiled)
  end
  vim.api.nvim_set_current_buf(buf)
end

---@param buf integer
local function focus(buf)
  local win = vim.iter(vim.fn.win_findbuf(buf)):find(windows.is_tiled)
  if win then
    vim.api.nvim_set_current_win(win)
  else
    show_tiled(buf)
  end
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
    focus(existing.buf)
    return existing.buf
  end
  local update = client:game(game_id)
  if update then
    return open_update(client, update, me, opts)
  end
  client:game_snapshot({ game_id = game_id }, function(err, snapshot)
    if err then
      notify.say(("games: game %s: %s"):format(game_id, err.message), vim.log.levels.ERROR)
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
    notify.say(("games: no board for game type %s"):format(update.game), vim.log.levels.ERROR)
    return nil
  end

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
    refit_pending = false,
    seats = #update.players,
  }
  boards[buf] = board
  watch_buffer(board)
  show_tiled(buf)
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
    notify.say("games: no game is waiting for your move")
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
