local finder = require("games.ui.finder")
local helpers = require("helpers")
local hub = require("fixtures.hub")
local sources = require("games.core.sources")

local assert_eq = helpers.assert_eq
local PickerName = finder.PickerName

---Runs fn with exactly the named picker plugins installed, as their adapters detect them.
---@param installed games.PickerName[]
---@param fn fun()
local function with_installed(installed, fn)
  local overrides = vim.tbl_map(function(name)
    return {
      finder.adapter(name),
      "available",
      function()
        return vim.tbl_contains(installed, name)
      end,
    }
  end, { PickerName.SNACKS, PickerName.TELESCOPE, PickerName.FZF })
  helpers.with_overrides(overrides, fn)
end

---Stubs vim.ui.select to pick the item labelled answers[i], recording each prompt's labels.
---@param answers string[]
---@param fn fun(prompts: string[][])
local function with_select_answers(answers, fn)
  local prompts = {}
  helpers.with_overrides({
    {
      vim.ui,
      "select",
      function(items, opts, on_choice)
        local labels = vim.tbl_map(opts.format_item or tostring, items)
        prompts[#prompts + 1] = labels
        local index = vim.fn.index(labels, answers[#prompts] or "")
        on_choice(index >= 0 and items[index + 1] or nil)
      end,
    },
  }, function()
    fn(prompts)
  end)
end

local T = {}

function T.detection_takes_the_first_installed_in_snacks_telescope_fzf_order()
  local cases = {
    { installed = { "snacks", "telescope", "fzf" }, picked = "snacks" },
    { installed = { "telescope", "fzf" }, picked = "telescope" },
    { installed = { "fzf" }, picked = "fzf" },
    { installed = {}, picked = "select" },
  }
  for _, case in ipairs(cases) do
    with_installed(case.installed, function()
      assert_eq({ case.installed, case.picked }, { case.installed, finder.picker_name() })
    end)
  end
end

function T.a_forced_picker_wins_unless_it_is_missing_then_select_is_used()
  local notified = {}
  helpers.with_overrides({
    {
      vim,
      "notify",
      function(message)
        notified[#notified + 1] = message
      end,
    },
  }, function()
    with_installed({ "snacks", "fzf" }, function()
      assert_eq("fzf", finder.picker_name("fzf"))
      assert_eq("select", finder.picker_name("select"))
      assert_eq("select", finder.picker_name("telescope"))
    end)
  end)
  assert_eq({ 'games: picker "telescope" is not installed; using vim.ui.select' }, notified)
end

function T.select_runs_the_default_action_of_the_picked_item()
  local client, conn = hub.client()
  local games = sources.build(sources.SourceName.GAMES, hub.snapshot(client))
  local opened = {}
  helpers.with_overrides({
    {
      require("games.ui.shell"),
      "open",
      function(_, game_id)
        opened[#opened + 1] = game_id
      end,
    },
  }, function()
    with_installed({}, function()
      with_select_answers({ games.format(hub.WAITING) }, function(prompts)
        finder.open(games)
        assert_eq(1, #prompts)
      end)
    end)
  end)
  assert_eq({ hub.WAITING.game_id }, opened)
  assert_eq(nil, hub.sent(conn, "game.control"))
end

function T.select_asks_which_action_when_the_source_has_no_default()
  local client, conn = hub.client()
  local requests = sources.build(sources.SourceName.REQUESTS, hub.snapshot(client))
  with_installed({}, function()
    with_select_answers({ requests.format(hub.INCOMING), "decline" }, function(prompts)
      finder.open(requests)
      assert_eq({ "accept", "decline" }, prompts[2])
    end)
  end)
  assert_eq({ challenge_id = "c-in" }, hub.sent(conn, "challenge.decline"))
end

return T
