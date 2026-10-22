local Conn = require("games.core.conn")
local describe = require("games.core.describe")
local json = require("games.core.json")
local registry = require("games.core.registry")
local tables = require("games.core.tables")

local M = {}

local REPO_ROOT = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")
local SERVER_BIN = REPO_ROOT .. "/bin/games-server"
local CATALOG_PATH = REPO_ROOT .. "/docs/schema/catalog.json"
local USERS = { "alice", "bob" }
M.PASSWORD = "pw"
M.WAIT_MS = 2000

---@class games.test.Server
---@field port integer
---@field pause fun()
---@field restart fun() kills the server and starts a new one on the same port and database
---@field stop fun()

---@param db string
---@param username string
local function add_user(db, username)
  local result = vim
    .system(
      { SERVER_BIN, "adduser", "--db", db, "--password-stdin", username },
      { stdin = M.PASSWORD .. "\n" }
    )
    :wait()
  assert(result.code == 0, ("adduser %s failed: %s"):format(username, result.stderr))
end

---@param db string
---@param addr string
---@param serve_args string[]
---@return vim.SystemObj process
---@return integer|nil port nil if the server did not report one in time
local function spawn(db, addr, serve_args)
  local port
  local cmd = vim.list_extend({ SERVER_BIN, "serve", "--addr", addr, "--db", db }, serve_args)
  local process = vim.system(cmd, {
    stderr = false,
    stdout = function(_, data)
      port = port or (data and tonumber(data:match("^listening on 127%.0%.0%.1:(%d+)")))
    end,
  })
  vim.wait(M.WAIT_MS, function()
    return port ~= nil
  end)
  return process, port
end

---@param process vim.SystemObj
local function kill(process)
  process:kill("sigkill")
  process:wait()
end

---@param serve_args? string[]
---@return games.test.Server
function M.start_server(serve_args)
  serve_args = serve_args or {}
  local dir = vim.fn.tempname()
  vim.fn.mkdir(dir, "p")
  local db = dir .. "/games.db"
  local added, add_err = pcall(function()
    for _, username in ipairs(USERS) do
      add_user(db, username)
    end
  end)
  if not added then
    vim.fn.delete(dir, "rf")
    error(add_err, 0)
  end

  local process, port = spawn(db, "127.0.0.1:0", serve_args)
  local stopped = false
  local function stop()
    if stopped then
      return
    end
    stopped = true
    kill(process)
    vim.fn.delete(dir, "rf")
  end
  if not port then
    stop()
    error("games-server did not report its port")
  end

  return {
    port = port,
    pause = function()
      process:kill("sigstop")
    end,
    restart = function()
      kill(process)
      local restarted_port
      process, restarted_port = spawn(db, "127.0.0.1:" .. port, serve_args)
      assert(restarted_port == port, "games-server did not restart on port " .. port)
    end,
    stop = stop,
  }
end

---@class games.test.Env
---@field server games.test.Server
---@field new_conn fun(username: string, password?: string|fun(): string?): games.Conn

---Runs fn against a fresh server and closes every conn it made and the server, even on failure.
---@param serve_args string[]|nil
---@param fn fun(env: games.test.Env)
function M.with_server(serve_args, fn)
  local server = M.start_server(serve_args)
  local conns = {}
  local env = {
    server = server,
    new_conn = function(username, password)
      local conn = Conn.new({
        host = "127.0.0.1",
        port = server.port,
        username = username,
        password = password or M.PASSWORD,
      })
      table.insert(conns, conn)
      return conn
    end,
  }
  local ok, err = xpcall(fn, debug.traceback, env)
  for _, conn in ipairs(conns) do
    conn:close()
  end
  server.stop()
  if not ok then
    error(err, 0)
  end
end

---@class games.test.EventLog
---@field connected games.LoginResult[]
---@field disconnected games.Disconnect[]

---@param conn games.Conn
---@return games.test.EventLog
function M.record_lifecycle(conn)
  local log = { connected = {}, disconnected = {} }
  conn:on("connected", function(result)
    table.insert(log.connected, result)
  end)
  conn:on("disconnected", function(err)
    table.insert(log.disconnected, err)
  end)
  return log
end

---@alias games.test.Override {[1]: table, [2]: string, [3]: any} target, key, value

---Sets each target[key] = value while fn runs and restores the originals, even when fn fails.
---@param overrides games.test.Override[]
---@param fn fun()
function M.with_overrides(overrides, fn)
  local originals = {}
  for index, override in ipairs(overrides) do
    originals[index] = override[1][override[2]]
    override[1][override[2]] = override[3]
  end
  local ok, err = xpcall(fn, debug.traceback)
  for index = #overrides, 1, -1 do
    local override = overrides[index]
    override[1][override[2]] = originals[index]
  end
  if not ok then
    error(err, 0)
  end
end

---@class games.test.SelectPrompt
---@field prompt string
---@field labels string[]

---Stubs vim.ui.select to answer each prompt with the item labelled answers[i] (none left
---cancels), recording the prompts.
---@param answers string[]
---@param fn fun(prompts: games.test.SelectPrompt[])
function M.with_select_answers(answers, fn)
  local prompts = {}
  M.with_overrides({
    {
      vim.ui,
      "select",
      function(items, opts, on_choice)
        local labels = vim.tbl_map(opts.format_item or tostring, items)
        table.insert(prompts, { prompt = opts.prompt, labels = labels })
        local answer = answers[#prompts]
        if not answer then
          on_choice(nil)
          return
        end
        local index = vim.fn.index(labels, answer)
        assert(index >= 0, ("%q is not offered by %q"):format(answer, opts.prompt))
        on_choice(items[index + 1])
      end,
    },
  }, function()
    fn(prompts)
  end)
end

---@class games.test.InputPrompt
---@field prompt string
---@field default string

---Stubs vim.ui.input to answer each prompt with answers[i], false cancelling, recording the prompts.
---@param answers (string|false)[]
---@param fn fun(prompts: games.test.InputPrompt[])
function M.with_input_answers(answers, fn)
  local prompts = {}
  M.with_overrides({
    {
      vim.ui,
      "input",
      function(opts, on_confirm)
        table.insert(prompts, { prompt = opts.prompt, default = opts.default })
        on_confirm(answers[#prompts] or nil)
      end,
    },
  }, function()
    fn(prompts)
  end)
end

---Stubs vim.fn.confirm to answer every question with answer while fn runs.
---@param answer integer 1 yes, 2 no
---@param fn fun()
---@return string[] asked the questions, in order
function M.confirming(answer, fn)
  local asked = {}
  M.with_overrides({
    {
      vim.fn,
      "confirm",
      function(message)
        table.insert(asked, message)
        return answer
      end,
    },
  }, fn)
  return asked
end

---Records every vim.notify message while fn runs; fn sees them as they arrive.
---@param fn fun(messages: string[])
---@return string[] messages
function M.capture_notifications(fn)
  local messages = {}
  M.with_overrides({
    {
      vim,
      "notify",
      function(message)
        table.insert(messages, message)
      end,
    },
  }, function()
    fn(messages)
  end)
  return messages
end

---A game's view options in glyphs, drawing from its default asset pack, with extra (e.g. cursor
---or flipped) on top.
---@param game games.GameType
---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
function M.view_options(game, glyphs, extra)
  local module = assert(registry.game_module(game))
  local art = require("games.ui.assets").module_art(game, module, { glyphs = glyphs })
  return vim.tbl_extend("force", { glyphs = glyphs, flipped = false, art = art }, extra or {})
end

---@class games.test.RenderGameOptions
---@field me? games.UserID default "alice"
---@field glyphs? games.Glyphs default "ascii"
---@field view? games.ViewOptions drawn with in place of the glyphs' view_options
---@field display? games.Display

---What the shell draws of game's update for opts.me, at no elapsed time, binding the actions the
---catalog lists for game.
---@param game games.GameType
---@param update games.GameUpdate
---@param opts? games.test.RenderGameOptions
---@return games.Rendered
---@return integer header_height
---@return games.KeyBinding[] bindings
function M.render_game(game, update, opts)
  local options = opts or {}
  local view = options.view or M.view_options(game, options.glyphs or "ascii")
  local module = assert(registry.game_module(game))
  local me = options.me or "alice"
  return require("games.ui.shell").render(
    update,
    module,
    view,
    me,
    0,
    options.display,
    M.catalogued(game)
  )
end

---@class games.Extmark one mark of games.Marks
---@field row integer 0-based line index
---@field col integer byte column
---@field end_col integer byte column, exclusive
---@field hl games.Hl

---The marks of rendered, one table each, in their order.
---@param rendered games.Rendered
---@return games.Extmark[]
function M.extmarks(rendered)
  local marks = rendered.marks
  local list = {}
  for i = 1, #marks.row do
    list[i] =
      { row = marks.row[i], col = marks.col[i], end_col = marks.end_col[i], hl = marks.hl[i] }
  end
  return list
end

---extmarks as the parallel arrays a games.Rendered holds.
---@param extmarks games.Extmark[]
---@return games.Marks
function M.marks_of(extmarks)
  local marks = require("games.ui.compose").no_marks()
  for i, mark in ipairs(extmarks) do
    marks.row[i], marks.col[i], marks.end_col[i], marks.hl[i] =
      mark.row, mark.col, mark.end_col, mark.hl
  end
  return marks
end

---The highlight of the extmark that starts at row and col, both 0-based.
---@param rendered games.Rendered
---@param row integer
---@param col integer byte column
---@return games.Hl|nil
function M.hl_at(rendered, row, col)
  local mark = vim.iter(M.extmarks(rendered)):find(function(candidate)
    return candidate.row == row and candidate.col == col
  end)
  return mark and mark.hl
end

---The highlight stack at row and col, both 0-based: one group as a list of one, none as {}.
---@param rendered games.Rendered
---@param row integer
---@param col integer byte column
---@return string[]
function M.hl_stack_at(rendered, row, col)
  local hl = M.hl_at(rendered, row, col)
  return type(hl) == "table" and hl or { hl }
end

---Where the cells of a board sit in its rendered lines.
---@class games.test.BoardGeometry
---@field top integer 0-based line of row 1
---@field left integer byte column of column 1
---@field step integer bytes from one column's cell to the next

---The highlight stack on cell r, c of a board laid out as board.
---@param rendered games.Rendered
---@param board games.test.BoardGeometry
---@param r integer
---@param c integer
---@return string[]
function M.cell_hl_at(rendered, board, r, c)
  return M.hl_stack_at(rendered, board.top + r - 1, board.left + (c - 1) * board.step)
end

---The highlights of the extmarks on row (0-based) in column order, only those starting before
---end_col when it is given.
---@param rendered games.Rendered
---@param row integer
---@param end_col? integer byte column
---@return games.Hl[]
function M.hls_on_row(rendered, row, end_col)
  local marks = vim
    .iter(ipairs(M.extmarks(rendered)))
    :filter(function(_, mark)
      return mark.row == row and mark.col < (end_col or math.huge)
    end)
    :map(function(index, mark)
      return { index = index, mark = mark }
    end)
    :totable()
  table.sort(marks, function(a, b)
    if a.mark.col ~= b.mark.col then
      return a.mark.col < b.mark.col
    end
    return a.index < b.index
  end)
  return vim.tbl_map(function(entry)
    return entry.mark.hl
  end, marks)
end

local CHESS_START_FEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

---An update of game 0000: fields, then overrides, over a fresh game's seq, options and events.
---Each replaces whole envelope fields, apart from view, whose fields it replaces one by one.
---@param game games.GameType
---@param fields table
---@param overrides? table
---@return games.GameUpdate
function M.game_update(game, fields, overrides)
  overrides = overrides or {}
  local update = vim.tbl_extend(
    "force",
    { game_id = "0000", game = game, seq = 0, options = {}, events = {} },
    fields,
    overrides
  )
  update.view = vim.tbl_extend("force", fields.view or {}, overrides.view or {})
  return update
end

---@param update games.GameUpdate
---@param event games.Event
---@return string what the event reads as in update
function M.described(update, event)
  return describe.events(vim.tbl_extend("force", update, { events = { event } }))
end

---Game 0000 at the start position, alice (white) to move against bob, with overrides on top.
---@param overrides? table
---@return games.ChessUpdate
function M.chess_update(overrides)
  return M.game_update("chess", {
    players = { { user = "alice", seat = "white" }, { user = "bob", seat = "black" } },
    to_act = { "alice" },
    controls = { "resign", "offer_draw", "abort" },
    view = {
      fen = CHESS_START_FEN,
      history_san = {},
      legal_moves = {},
      claimable_draw = false,
      material = { white = { pieces = {}, score = 0 }, black = { pieces = {}, score = 0 } },
      first_ply = 0,
    },
  }, overrides)
end

---@param expected any
---@param actual any
function M.assert_eq(expected, actual)
  assert(vim.deep_equal(expected, actual), vim.inspect({ expected = expected, actual = actual }))
end

---@param predicate fun(): boolean
---@param what string
function M.wait_for(predicate, what)
  assert(vim.wait(M.WAIT_MS, predicate), "timed out waiting for " .. what)
end

---Runs fn with a fresh temp dir path, not yet created, and removes the dir afterwards.
---@param fn fun(dir: string)
function M.with_temp_dir(fn)
  local dir = vim.fn.tempname()
  local ok, err = xpcall(fn, debug.traceback, dir)
  vim.fn.delete(dir, "rf")
  if not ok then
    error(err, 0)
  end
end

---@param path string
---@return string|nil nil when the file does not exist
function M.read_file(path)
  local file = io.open(path, "rb")
  if not file then
    return nil
  end
  local data = file:read("*a")
  file:close()
  return data
end

---The server's catalog as schemagen writes it to docs/schema/catalog.json, read afresh on each call.
---@return games.GameInfo[]
function M.catalog()
  local text = assert(M.read_file(CATALOG_PATH), CATALOG_PATH)
  return (json.decode(text) --[[@as games.CatalogResult]]).games
end

---@param game games.GameType
---@return games.GameInfo
function M.game_info(game)
  local info = vim.iter(M.catalog()):find(function(info)
    return info.id == game
  end)
  assert(info, ("catalog.json has no game %q"):format(game))
  return info
end

---The actions the server catalogues for game, by kind, as the shell takes them.
---@param game games.GameType
---@return table<games.ActionKind, games.ActionSpec>
function M.catalogued(game)
  return tables.index_by(M.game_info(game).actions, function(spec)
    return spec.kind
  end)
end

---A copy of update finished with summary, carrying summary's digest as the server would.
---@param update games.GameUpdate
---@param summary string
---@return games.GameUpdate
function M.seal(update, summary)
  return vim.tbl_extend("force", update, { summary = summary, digest = vim.fn.sha256(summary) })
end

---@class games.test.StubRequest
---@field type string
---@field body table|nil
---@field cb fun(err: games.ErrorBody|nil, result: table|nil)
---@field frame string as Conn encodes it

---@class games.test.StubConn
---@field handlers table<string, fun(body: table)[]>
---@field requests games.test.StubRequest[]
---@field credentials games.Credentials
---@field connects integer how often connect() was called
---@field connecting boolean
---@field error? games.ErrorBody what last_error() returns

---Records requests instead of sending them; push() delivers an event synchronously. sent() and
---respond() act on the newest request of a type.
---@param credentials? games.Credentials
---@return games.test.StubConn
function M.stub_conn(credentials)
  local conn = {
    handlers = vim.defaulttable(),
    requests = {},
    credentials = credentials or {},
    connects = 0,
    connecting = false,
  }
  function conn:connect()
    self.connects = self.connects + 1
  end
  function conn:fill_credentials(entered)
    self.credentials = vim.tbl_extend("keep", self.credentials, entered)
  end
  function conn:missing_credentials()
    return vim.tbl_filter(function(field)
      return self.credentials[field] == nil
    end, { "username", "password" })
  end
  function conn:address()
    return "127.0.0.1:7777"
  end
  function conn:is_connecting()
    return self.connecting
  end
  function conn:last_error()
    return self.error
  end
  function conn:on(event, fn)
    table.insert(self.handlers[event], fn)
  end
  function conn:request(message_type, body, cb)
    local _, frame = Conn.encode_request(#self.requests + 1, message_type, body)
    table.insert(self.requests, { type = message_type, body = body, cb = cb, frame = frame })
  end
  function conn:push(event, body)
    for _, fn in ipairs(self.handlers[event]) do
      fn(body)
    end
  end
  function conn:requests_of(message_type)
    return vim.tbl_filter(function(request)
      return request.type == message_type
    end, self.requests)
  end
  local function newest(message_type)
    local of_type = conn:requests_of(message_type)
    return of_type[#of_type]
  end
  function conn:sent(message_type)
    local request = newest(message_type)
    return request and request.body
  end
  function conn:requested(message_type)
    return newest(message_type) ~= nil
  end
  function conn:respond(message_type, result)
    assert(newest(message_type), "no " .. message_type .. " request").cb(nil, result)
  end
  return conn
end

return M
