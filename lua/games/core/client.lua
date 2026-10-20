local notify = require("games.core.notify")
local types = require("games.types")

local MessageType = types.MessageType
local ErrorCode = types.ErrorCode
local ChallengeState = types.ChallengeState

---@enum games.ConnectionState
local ConnectionState = { CONNECTED = "connected", DISCONNECTED = "disconnected" }
---Finished games the cache keeps; the server holds every one, so an evicted one is a snapshot away.
local FINISHED_CACHE_SIZE = 32

---@alias games.UpdateHandler fun(update: games.GameUpdate, is_new: boolean)
---@alias games.ChallengeHandler fun(challenge: games.Challenge, is_new: boolean)
---@alias games.CatalogHandler fun(catalog: games.CatalogResult)
---@alias games.ConnectionHandler fun(state: games.ConnectionState)
---@alias games.Unsubscribe fun() removes the handler it was returned for

---@class games.Client
---@field conn games.Conn
---@field package user? games.UserID
---@field private updates table<games.GameID, games.GameUpdate>
---@field private finished_ids games.GameID[] the cached finished games, oldest stored first
---@field private challenges table<games.ChallengeID, games.Challenge> pending only
---@field package resolved_challenges table<games.ChallengeID, true> this connection's, which only its own lists can race
---@field package catalog? games.CatalogResult static per server process, so refetched per connection
---@field private update_handlers games.UpdateHandler[]
---@field private challenge_handlers games.ChallengeHandler[]
---@field package catalog_handlers games.CatalogHandler[]
---@field package connection_handlers games.ConnectionHandler[]
---@field user_list fun(self: games.Client, body: nil, cb?: games.Reply<games.UserListResult>)
---@field challenge_create fun(self: games.Client, body: games.ChallengeCreateRequest, cb?: games.Reply<games.ChallengeCreateResult>)
---@field challenge_accept fun(self: games.Client, body: games.ChallengeRef, cb?: games.ReplyCallback)
---@field challenge_decline fun(self: games.Client, body: games.ChallengeRef, cb?: games.ReplyCallback)
---@field challenge_cancel fun(self: games.Client, body: games.ChallengeRef, cb?: games.ReplyCallback)
---@field challenge_list fun(self: games.Client, body: nil, cb?: games.Reply<games.ChallengeListResult>)
---@field game_list fun(self: games.Client, body: games.GameListRequest|nil, cb?: games.Reply<games.GameListResult>)
---@field game_snapshot fun(self: games.Client, body: games.GameSnapshotRequest, cb?: games.Reply<games.GameUpdate>)
---@field game_act fun(self: games.Client, body: games.GameActRequest, cb?: games.Reply<games.SeqResult>)
---@field game_control fun(self: games.Client, body: games.GameControlRequest, cb?: games.Reply<games.SeqResult>)
---@field game_start fun(self: games.Client, body: games.GameStartRequest, cb?: games.Reply<games.GameUpdate>)
---@field stats_get fun(self: games.Client, body: games.StatsRequest|nil, cb?: games.Reply<games.StatsResult>)
local Client = {}
Client.__index = Client
Client.ConnectionState = ConnectionState

---Sees a reply before cb does; returning true marks err as handled, so cb gets (nil, nil).
---@alias games.ReplyHook fun(self: games.Client, body: table|nil, err: games.ErrorBody|nil, result: table|nil): boolean|nil

---Builds the typed wrapper for one request type; on_reply lets the cache see the reply before cb.
---@param message_type games.MessageType
---@param on_reply? games.ReplyHook
---@return function
local function request_fn(message_type, on_reply)
  return function(self, body, cb)
    self.conn:request(message_type, body, function(err, result)
      local handled = on_reply and on_reply(self, body, err, result)
      if not cb then
        return
      end
      if handled then
        cb(nil, nil)
      else
        cb(err, result)
      end
    end)
  end
end

---Calls every handler even when one raises; each failure is reported, naming what was handled.
---@generic T
---@param handlers fun(item: T, is_new: boolean)[]
---@param item T
---@param is_new boolean
---@param what string e.g. "update for game <id>"
local function emit(handlers, item, is_new, what)
  for _, fn in ipairs(handlers) do
    local ok, err = pcall(fn, item, is_new)
    if not ok then
      notify.say(("games: handling %s failed: %s"):format(what, err), vim.log.levels.ERROR)
    end
  end
end

---@type games.ReplyHook
local function store_listed_games(self, _, _, result)
  for _, update in ipairs(result and result.games or {}) do
    self:store(update)
  end
end

---@type games.ReplyHook
local function store_snapshot(self, _, _, result)
  if result then
    self:store(result --[[@as games.GameUpdate]])
  end
end

---The snapshot replaces the stale state, so the caller never sees the error.
---@type games.ReplyHook
local function snapshot_if_stale(self, body, err)
  if err and err.code == ErrorCode.STALE and body then
    self:game_snapshot({ game_id = body.game_id })
    return true
  end
end

---@type games.ReplyHook
local function cache_catalog(self, _, _, result)
  if result then
    self.catalog = result --[[@as games.CatalogResult]]
    emit(self.catalog_handlers, self.catalog, true, "the game catalog")
  end
end

---@type games.ReplyHook
local function cache_challenges(self, _, _, result)
  if result then
    self:replace_challenges(result.challenges)
  end
end

Client.user_list = request_fn(MessageType.USER_LIST)
Client.challenge_create = request_fn(MessageType.CHALLENGE_CREATE)
Client.challenge_accept = request_fn(MessageType.CHALLENGE_ACCEPT)
Client.challenge_decline = request_fn(MessageType.CHALLENGE_DECLINE)
Client.challenge_cancel = request_fn(MessageType.CHALLENGE_CANCEL)
Client.challenge_list = request_fn(MessageType.CHALLENGE_LIST, cache_challenges)
Client.game_list = request_fn(MessageType.GAME_LIST, store_listed_games)
Client.game_snapshot = request_fn(MessageType.GAME_SNAPSHOT, store_snapshot)
Client.game_act = request_fn(MessageType.GAME_ACT, snapshot_if_stale)
Client.game_control = request_fn(MessageType.GAME_CONTROL, snapshot_if_stale)
Client.game_start = request_fn(MessageType.GAME_START, store_snapshot)
Client.stats_get = request_fn(MessageType.STATS_GET)

local request_catalog = request_fn(MessageType.CATALOG_LIST, cache_catalog)

---The server's game catalog, requested once per connection.
---@param body nil
---@param cb? fun(err: games.ErrorBody|nil, result: games.CatalogResult|nil)
function Client:catalog_list(body, cb)
  local catalog = self.catalog
  if not catalog then
    request_catalog(self, body, cb)
    return
  end
  if cb then
    vim.schedule(function()
      cb(nil, catalog)
    end)
  end
end

---Calls fn with each catalog game by type; on a failed request the map is empty.
---@param fn fun(infos: table<games.GameType, games.GameInfo>)
function Client:game_infos(fn)
  self:catalog_list(nil, function(_, catalog)
    local infos = {}
    for _, info in ipairs(catalog and catalog.games or {}) do
      infos[info.id] = info
    end
    fn(infos)
  end)
end

---The games in the catalog, empty until this connection has fetched it.
---@return games.GameInfo[]
function Client:catalog_games()
  return self.catalog and self.catalog.games or {}
end

---The catalog entry of game, or nil until this connection has fetched the catalog.
---@param game games.GameType
---@return games.GameInfo|nil
function Client:game_info(game)
  local catalog = self.catalog
  return catalog and vim.iter(catalog.games):find(function(info)
    return info.id == game
  end)
end

---@param conn games.Conn
---@return games.Client
function Client.new(conn)
  local self = setmetatable({
    conn = conn,
    updates = {},
    finished_ids = {},
    challenges = {},
    resolved_challenges = {},
    update_handlers = {},
    challenge_handlers = {},
    catalog_handlers = {},
    connection_handlers = {},
  }, Client)
  conn:on(MessageType.GAME_UPDATE, function(update)
    self:receive_update(update)
  end)
  conn:on(MessageType.CHALLENGE_CHANGED, function(challenge)
    self:receive_challenge(challenge)
  end)
  conn:on("connected", function(result)
    self.user = result.user
    self.catalog = nil
    self.resolved_challenges = {}
    self:resync()
    emit(self.connection_handlers, ConnectionState.CONNECTED, false, "the connection")
  end)
  conn:on("disconnected", function()
    self.user = nil
    self.catalog = nil
    emit(self.connection_handlers, ConnectionState.DISCONNECTED, false, "the connection")
  end)
  return self
end

---The logged-in user, or nil while disconnected.
---@return games.UserID|nil
function Client:me()
  return self.user
end

---@param game_id games.GameID
---@return games.GameUpdate|nil
function Client:game(game_id)
  return self.updates[game_id]
end

---@return games.GameUpdate[]
function Client:games()
  return vim.tbl_values(self.updates)
end

---Pending challenges from and to the logged-in user.
---@return games.Challenge[]
function Client:pending_challenges()
  return vim.tbl_values(self.challenges)
end

---Adds fn to the handlers field holds; the handlers are replaced, never edited, so an emit in
---progress calls the list it started with.
---@param self games.Client
---@param field "update_handlers"|"challenge_handlers"|"catalog_handlers"|"connection_handlers"
---@param fn function
---@return games.Unsubscribe
local function subscribe(self, field, fn)
  self[field] = vim.list_extend(vim.list_slice(self[field]), { fn })
  return function()
    self[field] = vim.tbl_filter(function(handler)
      return handler ~= fn
    end, self[field])
  end
end

---@param fn games.UpdateHandler called with is_new = true the first time a game_id is seen
---@return games.Unsubscribe
function Client:on_update(fn)
  return subscribe(self, "update_handlers", fn)
end

---@param fn games.ChallengeHandler called with is_new = true the first time a challenge_id is seen
---@return games.Unsubscribe
function Client:on_challenge(fn)
  return subscribe(self, "challenge_handlers", fn)
end

---@param fn games.CatalogHandler called each time a connection fetches the catalog
---@return games.Unsubscribe
function Client:on_catalog(fn)
  return subscribe(self, "catalog_handlers", fn)
end

---@param fn games.ConnectionHandler called after each login, once resync is sent, and each drop
---@return games.Unsubscribe
function Client:on_connection(fn)
  return subscribe(self, "connection_handlers", fn)
end

---Caches update unless the cache already holds this seq or a later one; a finished one is
---handled first and then counted against the finished games the cache keeps.
---@package
---@param update games.GameUpdate
function Client:store(update)
  local cached = self.updates[update.game_id]
  if cached and cached.seq >= update.seq then
    return
  end
  self.updates[update.game_id] = update
  emit(self.update_handlers, update, cached == nil, "update for game " .. update.game_id)
  if update.result then
    self:keep_finished(update.game_id)
  end
end

---Evicts the oldest stored finished games beyond FINISHED_CACHE_SIZE.
---@private
---@param game_id games.GameID just stored finished
function Client:keep_finished(game_id)
  local kept = vim.tbl_filter(function(id)
    return id ~= game_id
  end, self.finished_ids)
  kept[#kept + 1] = game_id
  while #kept > FINISHED_CACHE_SIZE do
    self.updates[table.remove(kept, 1)] = nil
  end
  self.finished_ids = kept
end

---A push that skips a seq means we missed steps, so the snapshot (not the push) becomes the truth.
---@package
---@param update games.GameUpdate
function Client:receive_update(update)
  local cached = self.updates[update.game_id]
  if cached and update.seq > cached.seq + 1 then
    self:game_snapshot({ game_id = update.game_id })
    return
  end
  self:store(update)
end

---Resolution is terminal, so a pending snapshot that arrives after it is stale and dropped.
---@package
---@param challenge games.Challenge
function Client:receive_challenge(challenge)
  local id = challenge.challenge_id
  if self.resolved_challenges[id] then
    return
  end
  local is_new = self.challenges[id] == nil
  if challenge.state == ChallengeState.PENDING then
    self.challenges[id] = challenge
  else
    self.challenges[id] = nil
    self.resolved_challenges[id] = true
  end
  emit(self.challenge_handlers, challenge, is_new, "challenge " .. id)
end

---@package
---@param challenges games.Challenge[]
function Client:replace_challenges(challenges)
  local previous = self.challenges
  self.challenges = {}
  local unresolved = vim.tbl_filter(function(challenge)
    return not self.resolved_challenges[challenge.challenge_id]
  end, challenges)
  for _, challenge in ipairs(unresolved) do
    self.challenges[challenge.challenge_id] = challenge
    if not previous[challenge.challenge_id] then
      emit(self.challenge_handlers, challenge, true, "challenge " .. challenge.challenge_id)
    end
  end
end

---Games that finished while we were offline drop out of the ongoing list, so snapshot them.
---@package
---@param listed games.GameUpdate[]
function Client:snapshot_unlisted(listed)
  local listed_ids = {}
  for _, update in ipairs(listed) do
    listed_ids[update.game_id] = true
  end
  for game_id, update in pairs(self.updates) do
    if update.result == nil and not listed_ids[game_id] then
      self:game_snapshot({ game_id = game_id })
    end
  end
end

---@package
function Client:resync()
  self:game_list(nil, function(_, result)
    if result then
      self:snapshot_unlisted(result.games)
    end
  end)
  self:challenge_list(nil)
  self:catalog_list(nil)
end

return Client
