local compose = require("games.ui.compose")
local describe = require("games.core.describe")
local finder = require("games.ui.finder")
local key_hints = require("games.ui.widgets.key_hints")
local kv = require("games.ui.widgets.kv")
local notify = require("games.core.notify")
local paint = require("games.ui.paint")
local sources = require("games.core.sources")
local text = require("games.ui.text")
local theme = require("games.ui.theme")
local timer = require("games.core.timer")
local types = require("games.types")

local ActionName = sources.ActionName
local SourceName = sources.SourceName

local RECENT_GAMES = 5
---Digits 1-9 start the first nine game types.
local START_KEYS = 9
local ITEM_INDENT = 2
local MIN_WIDTH = 40
---Columns and lines kept free around the float, so its border shows.
local EDGE_MARGIN = 2
local CLOCK_TICK_MS = 1000
local TITLE = " games "
local SECTION_HL = "GamesDashSection"
local EMPTY_HL = "GamesMuted"
local KEY_HL = "GamesKey"
local NAMESPACE = vim.api.nvim_create_namespace("games.dashboard")
local AUGROUP_NAME = "games.dashboard"

---@class games.DashboardTarget the item a line shows, and the source that acts on it
---@field source games.Source
---@field item any

---@class games.DashboardView
---@field rendered games.Rendered
---@field targets table<integer, games.DashboardTarget> by 1-based line
---@field starts games.DashboardTarget[] the game type each start key starts, by digit

---@class (private) games.DashboardSection
---@field title string
---@field source games.Source
---@field items any[]
---@field row fun(item: any, index: integer): games.Node

---@class (private) games.DashboardState
---@field client games.Client
---@field win integer
---@field buf integer
---@field augroup integer
---@field finished games.GameUpdate[] as the last game.list finished returned them
---@field view? games.DashboardView last rendered
---@field show_keys boolean
---@field render_pending boolean
---@field unsubscribes games.Unsubscribe[]
---@field tick? uv.uv_timer_t running while a shown clock is

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
      compose.text(key, KEY_HL),
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
      or sources.is_your_move(update, me) and your_move
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
---@param show_keys boolean list every key instead of the footer's few
---@return games.DashboardView
function M.view(snapshot, show_keys)
  local nodes, targets, starts = {}, {}, {}
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
      if section.source.name == SourceName.TYPES and item_index <= START_KEYS then
        starts[item_index] = target
      end
    end
  end
  nodes[#nodes + 1] = compose.text("")
  if show_keys then
    local key_pairs = vim.tbl_map(function(spec)
      return { spec.key, spec.label }
    end, KEYS)
    nodes[#nodes + 1] = kv({
      pairs = vim.list_extend(key_pairs, UNLISTED_KEYS),
      key_hl = KEY_HL,
    })
  else
    nodes[#nodes + 1] = key_hints(vim.tbl_filter(function(spec)
      return spec.hint
    end, KEYS))
  end
  return { rendered = compose.render(compose.col(nodes)), targets = targets, starts = starts }
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
  local lines = vim.tbl_keys(view.targets)
  table.sort(lines)
  return lines[1]
end

---@param snapshot games.SourceSnapshot
---@return boolean
local function shows_running_clock(snapshot)
  return vim.iter(snapshot.games):any(function(update)
    return update.result == nil and update.clock ~= nil and update.clock.running ~= nil
  end)
end

---@type fun(state: games.DashboardState)
local request_render

---@param state games.DashboardState
---@param needed boolean
local function sync_tick(state, needed)
  if needed and not state.tick then
    state.tick = assert(vim.uv.new_timer())
    state.tick:start(
      CLOCK_TICK_MS,
      CLOCK_TICK_MS,
      vim.schedule_wrap(function()
        request_render(state)
      end)
    )
  elseif not needed and state.tick then
    timer.close(state.tick)
    state.tick = nil
  end
end

---@param lines string[]
---@return vim.api.keyset.win_config
local function centred(lines)
  local width = math.min(
    math.max(text.max_display_width(lines), MIN_WIDTH, #TITLE),
    vim.o.columns - 2 * EDGE_MARGIN
  )
  local height = math.max(1, math.min(#lines, vim.o.lines - 2 * EDGE_MARGIN))
  return {
    relative = "editor",
    width = width,
    height = height,
    row = math.floor((vim.o.lines - height) / 2),
    col = math.floor((vim.o.columns - width) / 2),
  }
end

---@param state games.DashboardState
local function render(state)
  local me = state.client:me()
  if not me then
    notify.say("games: not connected", vim.log.levels.WARN)
    M.close()
    return
  end
  local snapshot = sources.snapshot(state.client, me, { finished = state.finished })
  local kept = target_key(target_at_cursor(state))
  local view = M.view(snapshot, state.show_keys)
  state.view = view
  paint(state.buf, NAMESPACE, 0, -1, view.rendered)
  vim.api.nvim_win_set_config(state.win, centred(view.rendered.lines))
  local line = line_of(view, kept) or first_target_line(view) or 1
  vim.api.nvim_win_set_cursor(state.win, { math.min(line, #view.rendered.lines), 0 })
  sync_tick(state, shows_running_clock(snapshot))
end

---Coalesces a burst of client events into one render on the next tick.
request_render = function(state)
  if state.render_pending then
    return
  end
  state.render_pending = true
  vim.schedule(function()
    state.render_pending = false
    if current == state then
      render(state)
    end
  end)
end

---@param state games.DashboardState
local function fetch_finished(state)
  state.client:game_list({ status = types.GameStatus.FINISHED }, function(err, result)
    if err then
      notify.say("games: " .. err.message, vim.log.levels.ERROR)
      return
    end
    if result then
      state.finished = result.games
      request_render(state)
    end
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
      notify.say(("games: nothing to %s here"):format(verb))
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
  local client, me = state.client, state.client:me()
  M.close()
  if me then
    require("games.core.commands").find(client, me, name)
  end
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
  { key = "r", label = "resign", run = act({ ActionName.RESIGN }, "resign") },
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

---@param state games.DashboardState
---@param digit integer
local function start(state, digit)
  local target = state.view and state.view.starts[digit]
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

---Closing on leave keeps a stale float from covering whatever took focus, such as a board.
---@param state games.DashboardState
local function watch_window(state)
  vim.api.nvim_create_autocmd("WinClosed", {
    group = state.augroup,
    pattern = tostring(state.win),
    callback = function()
      M.close()
    end,
  })
  vim.api.nvim_create_autocmd("WinLeave", {
    group = state.augroup,
    buffer = state.buf,
    callback = function()
      vim.schedule(function()
        if current == state then
          M.close()
        end
      end)
    end,
  })
  vim.api.nvim_create_autocmd("VimResized", {
    group = state.augroup,
    callback = function()
      request_render(state)
    end,
  })
end

---@param state games.DashboardState
local function subscribe(state)
  local function rerender()
    request_render(state)
  end
  state.unsubscribes = {
    state.client:on_update(rerender),
    state.client:on_challenge(rerender),
    state.client:on_catalog(rerender),
  }
end

---Whether the dashboard is open.
---@return boolean
function M.is_open()
  return current ~= nil
end

---Closes the dashboard and stops following the client.
function M.close()
  local state = current
  if not state then
    return
  end
  current = nil
  for _, unsubscribe in ipairs(state.unsubscribes) do
    unsubscribe()
  end
  sync_tick(state, false)
  vim.api.nvim_del_augroup_by_id(state.augroup)
  if vim.api.nvim_win_is_valid(state.win) then
    vim.api.nvim_win_close(state.win, true)
  end
end

---Opens the dashboard of client in a centred float, or focuses the one already open.
---@param client games.Client
function M.open(client)
  if current and current.client == client then
    vim.api.nvim_set_current_win(current.win)
    return
  end
  M.close()
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
  ---@type games.DashboardState
  local state = {
    client = client,
    win = win,
    buf = buf,
    augroup = vim.api.nvim_create_augroup(AUGROUP_NAME, { clear = true }),
    finished = {},
    show_keys = false,
    render_pending = false,
    unsubscribes = {},
  }
  current = state
  map_keys(state)
  watch_window(state)
  subscribe(state)
  render(state)
  fetch_finished(state)
end

return M
