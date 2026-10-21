local Client = require("games.core.client")
local compose = require("games.ui.compose")
local describe = require("games.core.describe")
local finder = require("games.ui.finder")
local float = require("games.ui.float")
local highlights = require("games.ui.highlights")
local key_hints = require("games.ui.widgets.key_hints")
local kv = require("games.ui.widgets.kv")
local notify = require("games.core.notify")
local paint = require("games.ui.paint")
local players = require("games.core.players")
local sources = require("games.core.sources")
local text = require("games.ui.text")
local theme = require("games.ui.theme")
local timer = require("games.core.timer")
local windows = require("games.ui.windows")

local ActionName = sources.ActionName
local Group = highlights.Group
local ConnectionState = Client.ConnectionState
local SourceName = sources.SourceName

local RECENT_GAMES = 5
---Digits 1-9 start the first nine game types.
local START_KEYS = 9
local ITEM_INDENT = 2
local MIN_WIDTH = 40
local CLOCK_TICK_MS = 1000
local TITLE = " games "
local SECTION_HL = Group.DASH_SECTION
local EMPTY_HL = Group.MUTED
local RECONNECTING = "reconnecting…"
local NAMESPACE = vim.api.nvim_create_namespace("games.dashboard")
local AUGROUP_NAME = "games.dashboard"

---@class games.DashboardTarget the item a line shows, and the source that acts on it
---@field source games.Source
---@field item any

---@class games.DashboardView
---@field rendered games.Rendered
---@field targets table<integer, games.DashboardTarget> by 1-based line

---@class games.DashboardViewOptions
---@field show_keys boolean list every key instead of the footer's few
---@field connection games.ConnectionState

---@class (private) games.DashboardSection
---@field title string
---@field source games.Source
---@field items any[]
---@field row fun(item: any, index: integer): games.Node

---@class (private) games.DashboardState
---@field client games.Client
---@field me games.UserID logged in when it opened; the snapshot stays theirs across reconnects
---@field connection games.ConnectionState
---@field win integer
---@field buf integer
---@field augroup integer
---@field finished games.GameUpdate[] as the last game.list finished returned them
---@field view? games.DashboardView last rendered
---@field show_keys boolean
---@field request_render fun() renders it on the next tick, once however often it is asked
---@field unsubscribes games.Unsubscribe[]
---@field ticker games.Ticker re-renders it while a shown clock runs

---@class (private) games.DashboardKey: games.KeyHint
---@field run fun(state: games.DashboardState)
---@field hint? boolean listed in the footer as well as under ?

local M = {}

---@type games.DashboardState|nil
local current

---@param source games.Source
---@return fun(item: any): games.Node
local function plain_row(source)
  return function(item)
    return compose.text(source.format(item))
  end
end

---@param source games.Source
---@param me games.UserID
---@return fun(update: games.GameUpdate): games.Node
local function finished_row(source, me)
  return function(update)
    return compose.text(
      source.format(update),
      theme.outcome_hl(describe.outcome(update.result, me))
    )
  end
end

---@param source games.Source
---@return fun(info: games.GameInfo, index: integer): games.Node
local function start_row(source)
  return function(info, index)
    local key = index <= START_KEYS and tostring(index) or " "
    return compose.row({
      compose.text(key, Group.KEY),
      compose.text(" "),
      compose.text(source.format(info)),
    })
  end
end

---@param snapshot games.SourceSnapshot
---@return games.DashboardSection[]
local function sections(snapshot)
  local me = snapshot.me
  local requests = sources.build(SourceName.REQUESTS, snapshot)
  local games = sources.build(SourceName.GAMES, snapshot)
  local game_types = sources.build(SourceName.TYPES, snapshot)
  local your_move, ongoing, finished = {}, {}, {}
  for _, update in ipairs(games.items) do
    local bucket = update.result and finished
      or players.is_to_act(update, me) and your_move
      or ongoing
    bucket[#bucket + 1] = update
  end
  return {
    { title = "Requests", source = requests, items = requests.items, row = plain_row(requests) },
    { title = "Your move", source = games, items = your_move, row = plain_row(games) },
    { title = "Ongoing", source = games, items = ongoing, row = plain_row(games) },
    {
      title = "Recent",
      source = games,
      items = vim.list_slice(finished, 1, RECENT_GAMES),
      row = finished_row(games, me),
    },
    { title = "New", source = game_types, items = game_types.items, row = start_row(game_types) },
  }
end

---@type games.DashboardKey[]
local KEYS
---@type games.KvPair[] keys mapped apart from KEYS, listed under ? after them
local UNLISTED_KEYS = { { "1-9", "start the New game of that number" }, { "<Esc>", "close" } }

---The dashboard of snapshot as lines, with the item each line shows.
---@param snapshot games.SourceSnapshot
---@param options games.DashboardViewOptions
---@return games.DashboardView
function M.view(snapshot, options)
  local nodes, targets = {}, {}
  if options.connection == ConnectionState.DISCONNECTED then
    vim.list_extend(nodes, { compose.text(RECONNECTING, EMPTY_HL), compose.text("") })
  end
  for index, section in ipairs(sections(snapshot)) do
    if index > 1 then
      nodes[#nodes + 1] = compose.text("")
    end
    nodes[#nodes + 1] = compose.text(section.title, SECTION_HL)
    if #section.items == 0 then
      nodes[#nodes + 1] = compose.pad(ITEM_INDENT, compose.text("none", EMPTY_HL))
    end
    for item_index, item in ipairs(section.items) do
      nodes[#nodes + 1] = compose.pad(ITEM_INDENT, section.row(item, item_index))
      local target = { source = section.source, item = item }
      targets[#nodes] = target
    end
  end
  nodes[#nodes + 1] = compose.text("")
  if options.show_keys then
    local key_pairs = vim.tbl_map(function(spec)
      return { spec.key, spec.label }
    end, KEYS)
    nodes[#nodes + 1] = kv({
      pairs = vim.list_extend(key_pairs, UNLISTED_KEYS),
      key_hl = Group.KEY,
    })
  else
    nodes[#nodes + 1] = key_hints(vim.tbl_filter(function(spec)
      return spec.hint
    end, KEYS))
  end
  return { rendered = compose.render(compose.col(nodes)), targets = targets }
end

---@param state games.DashboardState
---@return games.DashboardTarget|nil
local function target_at_cursor(state)
  local line = vim.api.nvim_win_get_cursor(state.win)[1]
  return state.view and state.view.targets[line]
end

---@param target games.DashboardTarget|nil
---@return string|nil
local function target_key(target)
  return target and (target.source.name .. ":" .. target.source.key(target.item))
end

---@param view games.DashboardView
---@param key string|nil
---@return integer|nil line
local function line_of(view, key)
  for line, target in pairs(view.targets) do
    if target_key(target) == key then
      return line
    end
  end
end

---@param view games.DashboardView
---@return integer|nil
local function first_target_line(view)
  local first
  for line in pairs(view.targets) do
    first = math.min(first or line, line)
  end
  return first
end

---@param snapshot games.SourceSnapshot
---@return boolean
local function shows_running_clock(snapshot)
  return vim.iter(snapshot.games):any(describe.has_running_clock)
end

---@param lines string[]
---@return vim.api.keyset.win_config
local function centred(lines)
  return float.centred(lines, math.max(MIN_WIDTH, text.width(TITLE)))
end

---@param state games.DashboardState
local function render(state)
  local snapshot = sources.snapshot(state.client, state.me, { finished = state.finished })
  local kept = target_key(target_at_cursor(state))
  local view = M.view(snapshot, { show_keys = state.show_keys, connection = state.connection })
  state.view = view
  paint(state.buf, NAMESPACE, 0, -1, view.rendered)
  vim.api.nvim_win_set_config(state.win, centred(view.rendered.lines))
  local line = line_of(view, kept) or first_target_line(view) or 1
  vim.api.nvim_win_set_cursor(state.win, { math.min(line, #view.rendered.lines), 0 })
  local connected = state.connection == ConnectionState.CONNECTED
  state.ticker.sync(connected and shows_running_clock(snapshot))
end

---@param state games.DashboardState
local function fetch_finished(state)
  state.client:finished_games(function(finished)
    state.finished = finished
    state.request_render()
  end)
end

---@param state games.DashboardState
local function refresh(state)
  state.client:game_list(nil)
  state.client:challenge_list(nil)
  fetch_finished(state)
end

---Runs the first of names that acts on the item under the cursor.
---@param names games.SourceActionName[]
---@param verb string what the key does, for when nothing here takes it
---@return fun(state: games.DashboardState)
local function act(names, verb)
  return function(state)
    local target = target_at_cursor(state)
    local action = target and sources.find_action(target.source, target.item, names)
    if not (target and action) then
      notify.say(("nothing to %s here"):format(verb))
      return
    end
    action.run(target.item)
  end
end

---@param state games.DashboardState
local function confirm(state)
  local target = target_at_cursor(state)
  if target then
    finder.confirm(target.source, target.item)
  end
end

---@param state games.DashboardState
local function find(state)
  local target = target_at_cursor(state)
  local name = target and target.source.name or SourceName.GAMES
  M.close()
  require("games.core.commands.find").find(state.client, state.me, name)
end

---@param state games.DashboardState
local function toggle_keys(state)
  state.show_keys = not state.show_keys
  render(state)
end

KEYS = {
  { key = "<CR>", label = "open", run = confirm, hint = true },
  { key = "a", label = "accept", run = act({ ActionName.ACCEPT }, "accept") },
  {
    key = "d",
    label = "decline or cancel",
    run = act({ ActionName.DECLINE, ActionName.CANCEL }, "decline"),
  },
  {
    key = "r",
    label = "resign or give up",
    run = act({ ActionName.RESIGN, ActionName.GIVE_UP }, "resign"),
  },
  { key = "/", label = "find", run = find, hint = true },
  { key = "R", label = "refresh", run = refresh },
  { key = "?", label = "keys", run = toggle_keys, hint = true },
  {
    key = "q",
    label = "close",
    run = function()
      M.close()
    end,
    hint = true,
  },
}

---The game type start key digit starts: the digit-th of the New game section.
---@param view games.DashboardView
---@param digit integer 1 to START_KEYS
---@return games.DashboardTarget|nil
function M.start_target(view, digit)
  local type_lines = vim.tbl_filter(function(line)
    return view.targets[line].source.name == SourceName.TYPES
  end, vim.tbl_keys(view.targets))
  table.sort(type_lines)
  local line = type_lines[digit]
  return line and view.targets[line]
end

---@param state games.DashboardState
---@param digit integer
local function start(state, digit)
  local target = state.view and M.start_target(state.view, digit)
  if target then
    finder.confirm(target.source, target.item)
  end
end

---@param state games.DashboardState
local function map_keys(state)
  ---@param key string
  ---@param label string
  ---@param run fun(state: games.DashboardState)
  local function map(key, label, run)
    vim.keymap.set("n", key, function()
      run(state)
    end, { buffer = state.buf, nowait = true, desc = "games: " .. label })
  end
  for _, spec in ipairs(KEYS) do
    map(spec.key, spec.label, spec.run)
  end
  map("<Esc>", "close", function()
    M.close()
  end)
  for digit = 1, START_KEYS do
    map(tostring(digit), "start a new game", function()
      start(state, digit)
    end)
  end
end

---@type fun(state: games.DashboardState)
local release

---@param state games.DashboardState
local function close_window(state)
  if vim.api.nvim_win_is_valid(state.win) then
    vim.api.nvim_win_close(state.win, true)
  end
end

---Focus landing in a tiled window, such as a board's, closes the dashboard so it never covers
---it; a float, such as a picker the dashboard opened, leaves it open to return to.
---@param state games.DashboardState
local function watch_window(state)
  vim.api.nvim_create_autocmd("WinClosed", {
    group = state.augroup,
    pattern = tostring(state.win),
    callback = function()
      if current == state then
        release(state)
      end
    end,
  })
  vim.api.nvim_create_autocmd("WinEnter", {
    group = state.augroup,
    callback = function()
      vim.schedule(function()
        local left = not vim.api.nvim_win_is_valid(state.win)
          or windows.is_tiled(vim.api.nvim_get_current_win())
        if current == state and left then
          M.close()
        end
      end)
    end,
  })
  vim.api.nvim_create_autocmd({ "BufWinLeave", "BufWipeout" }, {
    group = state.augroup,
    buffer = state.buf,
    callback = function()
      if current ~= state then
        return
      end
      release(state)
      vim.schedule(function()
        close_window(state)
      end)
    end,
  })
  vim.api.nvim_create_autocmd("VimResized", {
    group = state.augroup,
    callback = function()
      state.request_render()
    end,
  })
end

---@param state games.DashboardState
local function subscribe(state)
  local function rerender()
    state.request_render()
  end
  state.unsubscribes = {
    state.client:on_update(rerender),
    state.client:on_challenge(rerender),
    state.client:on_catalog(rerender),
    state.client:on_connection(function(connection)
      state.connection = connection
      if connection == ConnectionState.CONNECTED then
        fetch_finished(state)
      end
      rerender()
    end),
  }
end

---Whether the dashboard is open.
---@return boolean
function M.is_open()
  return current ~= nil
end

---Stops following the client and the window, leaving the window to whoever closes it.
release = function(state)
  current = nil
  for _, unsubscribe in ipairs(state.unsubscribes) do
    unsubscribe()
  end
  state.ticker.close()
  vim.api.nvim_del_augroup_by_id(state.augroup)
end

---Closes the dashboard and stops following the client.
function M.close()
  local state = current
  if not state then
    return
  end
  release(state)
  close_window(state)
end

---Opens the dashboard of client in a centred float, or focuses the one already open.
---@param client games.Client
function M.open(client)
  if current and current.client == client then
    vim.api.nvim_set_current_win(current.win)
    return
  end
  M.close()
  local me = notify.require_me(client)
  if not me then
    return
  end
  local buf = vim.api.nvim_create_buf(false, true)
  vim.bo[buf].bufhidden = "wipe"
  vim.bo[buf].modifiable = false
  local config = vim.tbl_extend("force", centred({ "" }), {
    style = "minimal",
    border = "rounded",
    title = TITLE,
    title_pos = "center",
  })
  local win = vim.api.nvim_open_win(buf, true, config)
  vim.wo[win].cursorline = true
  vim.wo[win].winfixbuf = true
  ---@type games.DashboardState
  local state
  local function render_if_current()
    if current == state then
      render(state)
    end
  end
  state = {
    client = client,
    me = me,
    connection = ConnectionState.CONNECTED,
    win = win,
    buf = buf,
    augroup = vim.api.nvim_create_augroup(AUGROUP_NAME, { clear = true }),
    finished = {},
    show_keys = false,
    request_render = timer.coalesced(render_if_current),
    unsubscribes = {},
    ticker = timer.ticker(CLOCK_TICK_MS, render_if_current),
  }
  current = state
  map_keys(state)
  watch_window(state)
  subscribe(state)
  render(state)
  fetch_finished(state)
end

return M
