local compose = require("games.ui.compose")
local describe = require("games.core.describe")
local games = require("games")
local notify = require("games.core.notify")
local paint = require("games.ui.paint")
local theme = require("games.ui.theme")
local players = require("games.core.players")
local registry = require("games.core.registry")
local types = require("games.types")

local Control = types.Control

---@class games.ViewOptions per-buffer presentation choices
---@field glyphs games.Glyphs
---@field flipped boolean perspective toggled away from the player's default

---@class games.LayoutCtx: games.ViewOptions
---@field me games.UserID

---@class games.GameInput
---@field prompt string
---@field parse fun(text: string): games.Action

---@class games.GameAction
---@field key string
---@field label string
---@field action? games.Action
---@field local_fn? fun(view: games.ViewOptions): games.ViewOptions

---@class games.GameModule
---@field layout fun(update: games.GameUpdate, ctx: games.LayoutCtx): games.Node
---@field input? games.GameInput
---@field actions? games.GameAction[]
---@field describe_event? fun(event: games.Event, update: games.GameUpdate): string|nil

---@class games.OpenOptions
---@field glyphs games.Glyphs

---@class games.Board
---@field buf integer
---@field client games.Client
---@field module games.GameModule
---@field me games.UserID
---@field view games.ViewOptions
---@field update games.GameUpdate
---@field header_height integer
---@field bindings table<string, games.KeyBinding> by key; each is mapped in the buffer

---@class games.KeyBinding
---@field key string
---@field label string
---@field run fun(board: games.Board)

---@class games.ControlKey
---@field control games.Control
---@field key string
---@field label string
---@field confirm? string

---@type games.ControlKey[]
local CONTROL_KEYS = {
  { control = Control.RESIGN, key = "R", label = "resign", confirm = "Resign this game?" },
  { control = Control.OFFER_DRAW, key = "D", label = "offer draw" },
  { control = Control.DECLINE_DRAW, key = "X", label = "decline draw" },
  { control = Control.ABORT, key = "A", label = "abort" },
}
local INPUT_KEY = "m"
local NEXT_GAME_KEY = "]g"
local CLOSE_KEY = "q"
local CLOCK_TICK_MS = 1000
local CONFIRM_YES = 1

---@type table<string, true> keys the shell owns whether or not they are active
local CORE_KEYS = { [INPUT_KEY] = true, [NEXT_GAME_KEY] = true, [CLOSE_KEY] = true }
for _, spec in ipairs(CONTROL_KEYS) do
  CORE_KEYS[spec.key] = true
end

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
    return compose.text("← your move", "GamesYourTurn")
  end
  return compose.text("← to move", "GamesMuted")
end

---@param update games.GameUpdate
---@return games.Node|nil
local function status_banner(update)
  if update.result then
    return compose.text(describe.result_text(update.result))
  end
  if update.draw_agreed and #update.draw_agreed > 0 then
    return compose.text("Draw offered by " .. table.concat(update.draw_agreed, ", "))
  end
end

---@param update games.GameUpdate
---@param me games.UserID
---@param elapsed_ms integer
---@return games.Rendered
local function render_header(update, me, elapsed_ms)
  ---@param cell fun(player: games.PlayerSeat): games.Node
  ---@return games.Node
  local function column(cell)
    return compose.col(vim.tbl_map(cell, update.players))
  end
  local columns = {
    column(function(player)
      return compose.text(player.user)
    end),
    compose.pad(
      2,
      column(function(player)
        return compose.text(player.seat, "GamesMuted")
      end)
    ),
  }
  if update.clock then
    columns[#columns + 1] = compose.pad(
      2,
      column(function(player)
        return compose.text(describe.clock_text(update, player.user, elapsed_ms))
      end)
    )
  end
  columns[#columns + 1] = compose.pad(
    2,
    column(function(player)
      return turn_marker(update, player.user, me)
    end)
  )
  return compose.render(compose.col({ compose.row(columns), status_banner(update) }))
end

---@param bindings games.KeyBinding[]
---@return games.Node
local function footer_node(bindings)
  local parts = {}
  for i, binding in ipairs(bindings) do
    if i > 1 then
      parts[#parts + 1] = compose.text("  ")
    end
    parts[#parts + 1] = compose.text(binding.key)
    parts[#parts + 1] = compose.text(" " .. binding.label, "GamesMuted")
  end
  return compose.row(parts)
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
---@param spec games.ControlKey
local function send_control(board, spec)
  if spec.confirm and vim.fn.confirm(spec.confirm, "&Yes\n&No", 2) ~= CONFIRM_YES then
    return
  end
  board.client:game_control(
    { game_id = board.update.game_id, seq = board.update.seq, control = spec.control },
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
  local tiled = vim.tbl_filter(function(win)
    return vim.api.nvim_win_get_config(win).relative == ""
  end, vim.api.nvim_tabpage_list_wins(0))
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

---@param update games.GameUpdate
---@param module games.GameModule
---@return games.KeyBinding[]
local function bindings_for(update, module)
  check_action_keys(module)
  local ongoing = update.result == nil
  local bindings = {}
  for _, spec in ipairs(CONTROL_KEYS) do
    if vim.tbl_contains(update.controls, spec.control) then
      bindings[#bindings + 1] = {
        key = spec.key,
        label = spec.label,
        run = function(board)
          send_control(board, spec)
        end,
      }
    end
  end
  local input = module.input
  if ongoing and input then
    bindings[#bindings + 1] = {
      key = INPUT_KEY,
      label = (input.prompt:gsub(":%s*$", ""):lower()),
      run = function(board)
        prompt_input(board, input)
      end,
    }
  end
  for _, entry in ipairs(module.actions or {}) do
    local local_fn, game_action = entry.local_fn, entry.action
    if local_fn then
      bindings[#bindings + 1] = {
        key = entry.key,
        label = entry.label,
        run = function(board)
          board.view = local_fn(board.view)
          render_board(board)
        end,
      }
    elseif ongoing and game_action then
      bindings[#bindings + 1] = {
        key = entry.key,
        label = entry.label,
        run = function(board)
          act(board, game_action)
        end,
      }
    end
  end
  bindings[#bindings + 1] = {
    key = NEXT_GAME_KEY,
    label = "next game",
    run = function(board)
      M.open_next(board.client, board.update.game_id, { glyphs = board.view.glyphs })
    end,
  }
  bindings[#bindings + 1] = { key = CLOSE_KEY, label = "close", run = close_window }
  return bindings
end

---@param update games.GameUpdate
---@param module games.GameModule
---@param view games.ViewOptions
---@param me games.UserID
---@param elapsed_ms? integer time since `update` arrived, charged to the running clock
---@return games.Rendered
---@return integer header_height lines owned by the header, which the clock rewrites
---@return games.KeyBinding[] bindings the footer lists
function M.render(update, module, view, me, elapsed_ms)
  local header = render_header(update, me, elapsed_ms or 0)
  local ctx = vim.tbl_extend("force", view, { me = me }) --[[@as games.LayoutCtx]]
  local bindings = bindings_for(update, module)
  local body = compose.render(compose.col({
    compose.text(""),
    module.layout(update, ctx),
    compose.text(""),
    footer_node(bindings),
  }))
  return stack(header, body), #header.lines, bindings
end

---@param buf integer
---@return boolean
local function is_shown(buf)
  return #vim.fn.win_findbuf(buf) > 0
end

---@param board games.Board
---@return integer
local function elapsed_ms(board)
  local now = vim.uv.now()
  return now - (board.client:received_at(board.update.game_id) or now)
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
  local rendered, header_height, bindings =
    M.render(board.update, board.module, board.view, board.me, elapsed_ms(board))
  paint(board.buf, NAMESPACE, 0, -1, rendered)
  board.header_height = header_height
  vim.b[board.buf].games_dirty = false
  map_keys(board, bindings)
end

---@param board games.Board
local function repaint_header(board)
  paint(
    board.buf,
    NAMESPACE,
    0,
    board.header_height,
    render_header(board.update, board.me, elapsed_ms(board))
  )
end

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
        vim.iter(ticking_boards()):each(repaint_header)
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
      if vim.b[buf].games_dirty then
        render_board(board)
      end
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

---@param buf integer
local function focus(buf)
  local win = vim.fn.win_findbuf(buf)[1]
  if win then
    vim.api.nvim_set_current_win(win)
  else
    vim.api.nvim_set_current_buf(buf)
  end
end

---@param client games.Client
---@param game_id games.GameID
---@param opts? games.OpenOptions defaults to the user's config
---@return integer|nil buf
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
  if not update then
    notify.say(("games: unknown game %s"):format(game_id), vim.log.levels.ERROR)
    return nil
  end
  local module = registry.game_module(update.game)
  if not module then
    notify.say(("games: no board for game type %s"):format(update.game), vim.log.levels.ERROR)
    return nil
  end

  theme.setup()
  subscribe(client)
  local buf = vim.api.nvim_create_buf(true, true)
  local name = ("games://%s/%s/%s"):format(
    update.game,
    describe.opponent_names(update, me, ","),
    describe.short_id(game_id)
  )
  vim.api.nvim_buf_set_name(buf, name)
  vim.bo[buf].modifiable = false
  vim.bo[buf].undolevels = -1

  ---@type games.Board
  local board = {
    buf = buf,
    client = client,
    module = module,
    me = me,
    view = { glyphs = (opts or games.open_options()).glyphs, flipped = false },
    update = update,
    header_height = 0,
    bindings = {},
  }
  boards[buf] = board
  watch_buffer(board)
  vim.api.nvim_set_current_buf(buf)
  render_board(board)
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
