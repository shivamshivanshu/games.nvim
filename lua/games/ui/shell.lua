local compose = require("games.ui.compose")
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
---@field glyphs? games.Glyphs

---@class games.Board
---@field buf integer
---@field client games.Client
---@field module games.GameModule
---@field me games.UserID
---@field view games.ViewOptions
---@field update games.GameUpdate
---@field header_height integer
---@field mapped_keys table<string, true> lhs currently mapped in the buffer

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
local DEFAULT_GLYPHS = "unicode"
local CLOCK_TICK_MS = 1000
local CONFIRM_YES = 1
local SHORT_ID_LENGTH = 4

---@type table<string, true> keys the shell owns whether or not they are active
local CORE_KEYS = { [INPUT_KEY] = true, [NEXT_GAME_KEY] = true, [CLOSE_KEY] = true }
for _, spec in ipairs(CONTROL_KEYS) do
  CORE_KEYS[spec.key] = true
end

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
---@param me games.UserID
---@return games.UserID[]
function M.opponents(update, me)
  local users = {}
  for _, player in ipairs(update.players) do
    if player.user ~= me then
      users[#users + 1] = player.user
    end
  end
  return users
end

---The id prefix that buffer names and pickers show.
---@param game_id games.GameID
---@return string
function M.short_id(game_id)
  return game_id:sub(1, SHORT_ID_LENGTH)
end

---"<game> vs <opponents>", the name notifications and errors use for a game.
---@param update games.GameUpdate
---@param me games.UserID
---@return string
function M.title(update, me)
  return ("%s vs %s"):format(update.game, table.concat(M.opponents(update, me), ", "))
end

---@param update games.GameUpdate with a clock
---@param user games.UserID
---@param elapsed_ms integer charged to user's clock if it is the running one
---@return string
function M.clock_text(update, user, elapsed_ms)
  local remaining_ms = update.clock.remaining_ms[user] or 0
  if update.clock.running == user then
    remaining_ms = remaining_ms - elapsed_ms
  end
  local seconds = math.max(0, math.ceil(remaining_ms / 1000))
  return ("%d:%02d"):format(math.floor(seconds / 60), seconds % 60)
end

---@param update games.GameUpdate
---@param user games.UserID
---@param me games.UserID
---@return games.Node
local function turn_marker(update, user, me)
  if update.result or not vim.tbl_contains(update.to_act, user) then
    return compose.text("")
  end
  if user == me then
    return compose.text("← your move", "GamesYourTurn")
  end
  return compose.text("← to move", "GamesMuted")
end

---@param result games.WireResult
---@return string
function M.result_text(result)
  local reason = (result.reason:gsub("_", " "))
  if #result.winners > 0 then
    return ("Result: %s won (%s)"):format(table.concat(result.winners, ", "), reason)
  end
  if result.reason == "aborted" then
    return "Result: aborted"
  end
  return ("Result: draw (%s)"):format(reason)
end

---@param update games.GameUpdate
---@return games.Node|nil
local function status_banner(update)
  if update.result then
    return compose.text(M.result_text(update.result))
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
  local names, seats, clocks, markers = {}, {}, {}, {}
  for _, player in ipairs(update.players) do
    names[#names + 1] = compose.text(player.user)
    seats[#seats + 1] = compose.text(player.seat, "GamesMuted")
    if update.clock then
      clocks[#clocks + 1] = compose.text(M.clock_text(update, player.user, elapsed_ms))
    end
    markers[#markers + 1] = turn_marker(update, player.user, me)
  end
  local columns = { compose.col(names), compose.pad(2, compose.col(seats)) }
  if update.clock then
    columns[#columns + 1] = compose.pad(2, compose.col(clocks))
  end
  columns[#columns + 1] = compose.pad(2, compose.col(markers))

  local lines = { compose.row(columns) }
  lines[#lines + 1] = status_banner(update)
  return compose.render(compose.col(lines))
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
    -- The client resyncs on stale; the snapshot it fetches re-renders the board.
    if err and err.code ~= types.ErrorCode.STALE then
      vim.notify(
        ("%s: %s"):format(M.title(board.update, board.me), err.message),
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

---@param bindings games.KeyBinding[]
---@param binding games.KeyBinding
local function add_game_binding(bindings, binding)
  assert(
    not CORE_KEYS[binding.key],
    ("game action key %q is reserved by the shell"):format(binding.key)
  )
  for _, existing in ipairs(bindings) do
    assert(existing.key ~= binding.key, ("game action key %q is bound twice"):format(binding.key))
  end
  bindings[#bindings + 1] = binding
end

---@param update games.GameUpdate
---@param module games.GameModule
---@return games.KeyBinding[]
local function bindings_for(update, module)
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
      add_game_binding(bindings, {
        key = entry.key,
        label = entry.label,
        run = function(board)
          board.view = local_fn(board.view)
          render_board(board)
        end,
      })
    elseif ongoing and game_action then
      add_game_binding(bindings, {
        key = entry.key,
        label = entry.label,
        run = function(board)
          act(board, game_action)
        end,
      })
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
function M.render(update, module, view, me, elapsed_ms)
  local header = render_header(update, me, elapsed_ms or 0)
  local ctx = vim.tbl_extend("force", view, { me = me }) --[[@as games.LayoutCtx]]
  local body = compose.render(compose.col({
    compose.text(""),
    module.layout(update, ctx),
    compose.text(""),
    footer_node(bindings_for(update, module)),
  }))
  return stack(header, body), #header.lines
end

---@param event games.Event
---@param update games.GameUpdate
---@return string
local function fallback_description(event, update)
  local actor = event.by and (players.user_of_seat(update, event.by) or event.by)
  return actor and ("%s: %s"):format(actor, event.kind) or event.kind
end

---@param update games.GameUpdate
---@return string
function M.describe(update)
  local module = registry.game_module(update.game)
  local describe_event = module and module.describe_event
  local descriptions = vim.tbl_map(function(event)
    return describe_event and describe_event(event, update) or fallback_description(event, update)
  end, update.events)
  return table.concat(descriptions, "; ")
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

---@param buf integer
---@param first integer
---@param last integer exclusive, -1 for the end of the buffer
---@param rendered games.Rendered
local function paint(buf, first, last, rendered)
  vim.api.nvim_buf_clear_namespace(buf, NAMESPACE, first, last)
  vim.bo[buf].modifiable = true
  local ok, err = pcall(vim.api.nvim_buf_set_lines, buf, first, last, false, rendered.lines)
  vim.bo[buf].modifiable = false
  if not ok then
    error(err, 0)
  end
  for _, mark in ipairs(rendered.extmarks) do
    vim.api.nvim_buf_set_extmark(buf, NAMESPACE, first + mark.row, mark.col, {
      end_col = mark.end_col,
      hl_group = mark.hl,
    })
  end
end

---@param board games.Board
---@param bindings games.KeyBinding[]
local function map_keys(board, bindings)
  local wanted = {}
  for _, binding in ipairs(bindings) do
    wanted[binding.key] = true
    vim.keymap.set("n", binding.key, function()
      binding.run(board)
    end, { buffer = board.buf, nowait = true, desc = "games: " .. binding.label })
  end
  for key in pairs(board.mapped_keys) do
    if not wanted[key] then
      vim.keymap.del("n", key, { buffer = board.buf })
    end
  end
  board.mapped_keys = wanted
end

render_board = function(board)
  local rendered, header_height =
    M.render(board.update, board.module, board.view, board.me, elapsed_ms(board))
  paint(board.buf, 0, -1, rendered)
  board.header_height = header_height
  vim.b[board.buf].games_dirty = false
  map_keys(board, bindings_for(board.update, board.module))
end

---@param board games.Board
local function repaint_header(board)
  paint(board.buf, 0, board.header_height, render_header(board.update, board.me, elapsed_ms(board)))
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
        vim.tbl_map(repaint_header, ticking_boards())
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
---@return games.UserID|nil
local function connected_user(client)
  local me = client:me()
  if not me then
    vim.notify("games: not connected", vim.log.levels.WARN)
  end
  return me
end

---@param client games.Client
---@param game_id games.GameID
---@param opts? games.OpenOptions
---@return integer|nil buf
function M.open(client, game_id, opts)
  local me = connected_user(client)
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
    vim.notify(("games: unknown game %s"):format(game_id), vim.log.levels.ERROR)
    return nil
  end
  local module = registry.game_module(update.game)
  if not module then
    vim.notify(("games: no board for game type %s"):format(update.game), vim.log.levels.ERROR)
    return nil
  end

  theme.setup()
  subscribe(client)
  local buf = vim.api.nvim_create_buf(true, true)
  local name = ("games://%s/%s/%s"):format(
    update.game,
    table.concat(M.opponents(update, me), ","),
    M.short_id(game_id)
  )
  vim.api.nvim_buf_set_name(buf, name)
  vim.bo[buf].modifiable = false

  ---@type games.Board
  local board = {
    buf = buf,
    client = client,
    module = module,
    me = me,
    view = { glyphs = opts and opts.glyphs or DEFAULT_GLYPHS, flipped = false },
    update = update,
    header_height = 0,
    mapped_keys = {},
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
---@param opts? games.OpenOptions
---@return integer|nil buf
function M.open_next(client, after_game_id, opts)
  local me = connected_user(client)
  if not me then
    return nil
  end
  local waiting = vim.tbl_filter(function(update)
    return update.result == nil
      and update.game_id ~= after_game_id
      and vim.tbl_contains(update.to_act, me)
  end, client:games())
  if #waiting == 0 then
    vim.notify("games: no game is waiting for your move", vim.log.levels.INFO)
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
