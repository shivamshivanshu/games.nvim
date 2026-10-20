local compose = require("games.ui.compose")
local theme = require("games.ui.theme")

---@alias games.Glyphs "unicode"|"ascii"

---@class games.ViewOptions per-buffer presentation choices
---@field glyphs games.Glyphs
---@field flipped boolean perspective toggled away from the player's default

---@class games.LayoutCtx: games.ViewOptions
---@field me string

---@class games.GameInput
---@field prompt string
---@field parse fun(text: string): table action sent with game.act

---@class games.GameAction
---@field key string
---@field label string
---@field action? table sent with game.act
---@field local_fn? fun(view: games.ViewOptions): games.ViewOptions

---@class games.GameModule
---@field layout fun(update: games.GameUpdate, ctx: games.LayoutCtx): games.Node
---@field input? games.GameInput
---@field actions? games.GameAction[]
---@field describe_event? fun(event: table, update: games.GameUpdate): string|nil

---@alias games.ReplyFn fun(err: games.ErrorBody|nil, result: table|nil)

---@class games.ShellClient
---@field game fun(self: games.ShellClient, game_id: string): games.GameUpdate|nil
---@field games fun(self: games.ShellClient): games.GameUpdate[]
---@field me fun(self: games.ShellClient): string
---@field on_update fun(self: games.ShellClient, fn: fun(update: games.GameUpdate, is_new: boolean))
---@field game_act fun(self: games.ShellClient, body: table, cb: games.ReplyFn)
---@field game_control fun(self: games.ShellClient, body: table, cb: games.ReplyFn)

---@class games.OpenOptions
---@field glyphs? games.Glyphs

---@class games.Board
---@field buf integer
---@field client games.ShellClient
---@field module games.GameModule
---@field me string
---@field view games.ViewOptions
---@field update games.GameUpdate
---@field received_at integer loop time (ms) when `update` arrived
---@field header_height integer
---@field mapped_keys string[]

---@class games.KeyBinding
---@field key string
---@field label string
---@field run fun(board: games.Board)

---@class games.ControlKey
---@field control string
---@field key string
---@field label string
---@field confirm? string

---@type games.ControlKey[]
local CONTROL_KEYS = {
  { control = "resign", key = "R", label = "resign", confirm = "Resign this game?" },
  { control = "offer_draw", key = "D", label = "offer draw" },
  { control = "decline_draw", key = "X", label = "decline draw" },
  { control = "abort", key = "A", label = "abort" },
}
local INPUT_KEY = "m"
local NEXT_GAME_KEY = "]g"
local CLOSE_KEY = "q"
local DEFAULT_GLYPHS = "unicode"
local CLOCK_TICK_MS = 1000
local CONFIRM_YES = 1
local STALE = "stale"

local NAMESPACE = vim.api.nvim_create_namespace("games.shell")
local AUGROUP = vim.api.nvim_create_augroup("games.shell", { clear = true })

local M = {}

---@type table<integer, games.Board> buffer -> board
local boards = {}
---@type table<games.ShellClient, true>
local subscribed_clients = setmetatable({}, { __mode = "k" })
---@type uv.uv_timer_t|nil
local clock_timer

---@param game string
---@return games.GameModule|nil
local function game_module(game)
  local ok, module = pcall(require, "games.games." .. game)
  return ok and module or nil
end

---@param update games.GameUpdate
---@param me string
---@return string[]
local function opponents(update, me)
  local users = {}
  for _, player in ipairs(update.players) do
    if player.user ~= me then
      users[#users + 1] = player.user
    end
  end
  return users
end

---@param update games.GameUpdate
---@param me string
---@return string
local function title(update, me)
  return ("%s vs %s"):format(update.game, table.concat(opponents(update, me), ", "))
end

---@param update games.GameUpdate
---@param user string
---@param elapsed_ms integer
---@return string
local function clock_text(update, user, elapsed_ms)
  local remaining_ms = update.clock.remaining_ms[user] or 0
  if update.clock.running == user then
    remaining_ms = remaining_ms - elapsed_ms
  end
  local seconds = math.max(0, math.ceil(remaining_ms / 1000))
  return ("%d:%02d"):format(math.floor(seconds / 60), seconds % 60)
end

---@param update games.GameUpdate
---@param user string
---@param me string
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
local function result_text(result)
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
    return compose.text(result_text(update.result))
  end
  if update.draw_agreed and #update.draw_agreed > 0 then
    return compose.text("Draw offered by " .. table.concat(update.draw_agreed, ", "))
  end
end

---@param update games.GameUpdate
---@param me string
---@param elapsed_ms integer
---@return games.Node
local function header_node(update, me, elapsed_ms)
  local names, seats, clocks, markers = {}, {}, {}, {}
  for _, player in ipairs(update.players) do
    names[#names + 1] = compose.text(player.user)
    seats[#seats + 1] = compose.text(player.seat, "GamesMuted")
    if update.clock then
      clocks[#clocks + 1] = compose.text(clock_text(update, player.user, elapsed_ms))
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
  return compose.col(lines)
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

---@param board games.Board
---@return games.ReplyFn
local function report_error(board)
  return function(err)
    -- The client resyncs on stale; the snapshot it fetches re-renders the board.
    if err and err.code ~= STALE then
      vim.notify(
        ("%s: %s"):format(title(board.update, board.me), err.message),
        vim.log.levels.ERROR
      )
    end
  end
end

---@param board games.Board
---@param game_action table
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
  if #vim.api.nvim_list_wins() > 1 then
    vim.api.nvim_win_close(0, false)
  else
    vim.cmd.enew()
  end
end

---@type fun(board: games.Board)
local render_board

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
    local local_fn = entry.local_fn
    if local_fn then
      bindings[#bindings + 1] = {
        key = entry.key,
        label = entry.label,
        run = function(board)
          board.view = local_fn(board.view)
          render_board(board)
        end,
      }
    elseif ongoing then
      bindings[#bindings + 1] = {
        key = entry.key,
        label = entry.label,
        run = function(board)
          act(board, entry.action)
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
---@param me string
---@param elapsed_ms? integer time since `update` arrived, charged to the running clock
---@return games.Rendered
function M.render(update, module, view, me, elapsed_ms)
  local ctx = vim.tbl_extend("force", view, { me = me }) --[[@as games.LayoutCtx]]
  return compose.render(compose.col({
    header_node(update, me, elapsed_ms or 0),
    compose.text(""),
    module.layout(update, ctx),
    compose.text(""),
    footer_node(bindings_for(update, module)),
  }))
end

---@param event table
---@param update games.GameUpdate
---@return string
local function fallback_description(event, update)
  local player = vim.iter(update.players):find(function(p)
    return p.seat == event.by or p.user == event.by
  end)
  local actor = player and player.user or event.by
  return actor and ("%s: %s"):format(actor, event.kind) or event.kind
end

---@param update games.GameUpdate
---@return string
function M.describe(update)
  local module = game_module(update.game)
  local describe_event = module and module.describe_event
  local descriptions = vim.tbl_map(function(event)
    return describe_event and describe_event(event, update) or fallback_description(event, update)
  end, update.events)
  return table.concat(descriptions, "; ")
end

---@param buf integer
---@return boolean
local function is_visible(buf)
  return #vim.fn.win_findbuf(buf) > 0
end

---@param board games.Board
---@return integer
local function elapsed_ms(board)
  return vim.uv.now() - board.received_at
end

---@param buf integer
---@param first integer
---@param last integer exclusive, -1 for the end of the buffer
---@param rendered games.Rendered
local function paint(buf, first, last, rendered)
  vim.api.nvim_buf_clear_namespace(buf, NAMESPACE, first, last)
  vim.bo[buf].modifiable = true
  vim.api.nvim_buf_set_lines(buf, first, last, false, rendered.lines)
  vim.bo[buf].modifiable = false
  for _, mark in ipairs(rendered.extmarks) do
    vim.api.nvim_buf_set_extmark(buf, NAMESPACE, first + mark.row, mark.col, {
      end_col = mark.end_col,
      hl_group = mark.hl,
    })
  end
end

---@param board games.Board
local function map_keys(board)
  for _, key in ipairs(board.mapped_keys) do
    vim.keymap.del("n", key, { buffer = board.buf })
  end
  board.mapped_keys = {}
  for _, binding in ipairs(bindings_for(board.update, board.module)) do
    vim.keymap.set("n", binding.key, function()
      binding.run(board)
    end, { buffer = board.buf, nowait = true, desc = "games: " .. binding.label })
    table.insert(board.mapped_keys, binding.key)
  end
end

render_board = function(board)
  local elapsed = elapsed_ms(board)
  paint(board.buf, 0, -1, M.render(board.update, board.module, board.view, board.me, elapsed))
  board.header_height = #compose.render(header_node(board.update, board.me, elapsed)).lines
  vim.b[board.buf].games_dirty = false
  map_keys(board)
end

---@param board games.Board
local function repaint_header(board)
  local header = compose.render(header_node(board.update, board.me, elapsed_ms(board)))
  paint(board.buf, 0, board.header_height, header)
end

---@return games.Board[]
local function ticking_boards()
  return vim.tbl_filter(function(board)
    return board.update.clock ~= nil and board.update.clock.running ~= nil and is_visible(board.buf)
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
  board.received_at = vim.uv.now()
  if is_visible(board.buf) then
    render_board(board)
  else
    vim.b[board.buf].games_dirty = true
  end
  sync_clock_timer()
end

---@param client games.ShellClient
---@param game_id string
---@return games.Board|nil
local function board_for(client, game_id)
  return vim.iter(vim.tbl_values(boards)):find(function(board)
    return board.client == client and board.update.game_id == game_id
  end)
end

---@param client games.ShellClient
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
  vim.api.nvim_create_autocmd("BufWipeout", {
    group = AUGROUP,
    buffer = buf,
    callback = function()
      boards[buf] = nil
      vim.schedule(sync_clock_timer)
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

---@param client games.ShellClient
---@param game_id string
---@param opts? games.OpenOptions
---@return integer|nil buf
function M.open(client, game_id, opts)
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
  local module = game_module(update.game)
  if not module then
    vim.notify(("games: no board for game type %s"):format(update.game), vim.log.levels.ERROR)
    return nil
  end

  theme.setup()
  subscribe(client)
  local me = client:me()
  local buf = vim.api.nvim_create_buf(true, true)
  local name = ("games://%s/%s/%s"):format(
    update.game,
    table.concat(opponents(update, me), ","),
    game_id:sub(1, 4)
  )
  vim.api.nvim_buf_set_name(buf, name)
  vim.bo[buf].modifiable = false

  local board = {
    buf = buf,
    client = client,
    module = module,
    me = me,
    view = { glyphs = opts and opts.glyphs or DEFAULT_GLYPHS, flipped = false },
    update = update,
    received_at = vim.uv.now(),
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

---@param client games.ShellClient
---@param after_game_id? string
---@param opts? games.OpenOptions
---@return integer|nil buf
function M.open_next(client, after_game_id, opts)
  local me = client:me()
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
