local Client = require("games.core.client")
local games = require("games")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

---@class games.test.UiStubs
---@field notified string[]
---@field input_text? string

---Alice is the plugin's own client; ui and notify are stubbed and restored even if fn fails.
---@param fn fun(env: games.test.Env, alice: games.Client, stubs: games.test.UiStubs)
local function with_plugin(fn)
  helpers.with_server(nil, function(env)
    local original = { select = vim.ui.select, input = vim.ui.input, notify = vim.notify }
    local plugin = { config = games.config, client = games.client }
    ---@type games.test.UiStubs
    local stubs = { notified = {} }
    vim.ui.select = function(items, _, on_choice)
      -- Defaults everywhere, except alice takes white so she moves first.
      on_choice(vim.tbl_contains(items, "white") and "white" or items[1])
    end
    vim.ui.input = function(_, on_confirm)
      on_confirm(stubs.input_text)
    end
    vim.notify = function(message)
      table.insert(stubs.notified, message)
    end
    local ok, err = xpcall(function()
      games.setup({
        server = { host = "127.0.0.1", port = env.server.port },
        username = "alice",
        password = helpers.PASSWORD,
        autoconnect = false,
        glyphs = "ascii",
      })
      fn(env, assert(games.client), stubs)
    end, debug.traceback)
    vim.ui.select, vim.ui.input, vim.notify = original.select, original.input, original.notify
    if games.client then
      games.client.conn:close()
    end
    games.config, games.client = plugin.config, plugin.client
    vim.cmd("silent! %bwipeout!")
    if not ok then
      error(err, 0)
    end
  end)
end

---Alice challenges bob through :Games and bob accepts; returns once alice's board is current.
---@param env games.test.Env
---@param alice games.Client
---@return games.Client bob
---@return games.GameID
local function start_game(env, alice)
  local bob = Client.new(env.new_conn("bob"))
  alice.conn:connect()
  bob.conn:connect()
  helpers.wait_for(function()
    return alice:me() == "alice" and bob:me() == "bob"
  end, "both logins")

  vim.cmd.runtime("plugin/games.lua")
  vim.cmd("Games challenge bob chess")
  helpers.wait_for(function()
    return #bob:pending_challenges() == 1
  end, "bob's pending challenge")
  bob:challenge_accept({ challenge_id = bob:pending_challenges()[1].challenge_id })
  helpers.wait_for(function()
    return vim.api.nvim_buf_get_name(0):find("^games://chess/bob/") ~= nil
  end, "alice's board to open")
  return bob, alice:games()[1].game_id
end

---@param bob games.Client
---@param game_id games.GameID
---@param seq integer
local function wait_for_bob_seq(bob, game_id, seq)
  helpers.wait_for(function()
    local update = bob:game(game_id)
    return update ~= nil and update.seq == seq
  end, "bob's cache to reach seq " .. seq)
end

---@param buf integer
---@param rank string
---@return string|nil
local function rank_line(buf, rank)
  return vim.iter(vim.api.nvim_buf_get_lines(buf, 0, -1, false)):find(function(line)
    return vim.startswith(line, rank .. " ")
  end)
end

local T = {}

function T.e2e_challenge_accept_move()
  with_plugin(function(env, alice, stubs)
    local bob, game_id = start_game(env, alice)
    local board = vim.api.nvim_get_current_buf()

    stubs.input_text = "e4"
    vim.api.nvim_feedkeys("m", "mx", false)
    wait_for_bob_seq(bob, game_id, 1)
    assert_eq({ "e4" }, bob:game(game_id).view.history_san)
    helpers.wait_for(function()
      local line = rank_line(board, "4")
      return line ~= nil and line:match("^4 %. %. %. %. P") ~= nil
    end, "alice's board to show the pawn on e4")
  end)
end

function T.notify_on_hidden_update()
  with_plugin(function(env, alice, stubs)
    local bob, game_id = start_game(env, alice)
    alice:game_act({
      game_id = game_id,
      seq = alice:game(game_id).seq,
      action = { kind = "move", move = "e4" },
    })
    wait_for_bob_seq(bob, game_id, 1)
    helpers.wait_for(function()
      return alice:game(game_id).seq == 1
    end, "alice's own move to arrive")

    vim.cmd.enew()
    bob:game_act({ game_id = game_id, seq = 1, action = { kind = "move", move = "e5" } })
    helpers.wait_for(function()
      return vim.iter(stubs.notified):any(function(message)
        return message:find("chess vs bob: bob played e5 — your move", 1, true) ~= nil
      end)
    end, "the hidden-update notification")
    assert(not vim.iter(stubs.notified):any(function(message)
      return message:find("alice played e4", 1, true) ~= nil
    end), "a visible board must not notify: " .. vim.inspect(stubs.notified))
  end)
end

return T
