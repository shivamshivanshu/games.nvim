-- Opens the finder in each real picker plugin, headless, and confirms its first item.
-- Run with `make smoke-finders PICKERS_PACK=<dir>`; tests/smoke/fetch-pickers.sh fills <dir>.
local repo = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")
vim.opt.runtimepath:prepend(repo)
package.path = repo .. "/tests/lua/?.lua;" .. package.path
vim.opt.packpath:prepend(assert(vim.env.PICKERS_PACK, "PICKERS_PACK is not set"))
for _, plugin in ipairs({ "plenary.nvim", "telescope.nvim", "snacks.nvim", "fzf-lua" }) do
  vim.cmd.packadd(plugin)
end

local finder = require("games.ui.finder")
local helpers = require("helpers")
local hub = require("fixtures.hub")
local sources = require("games.core.sources")

local WAIT_MS = 5000
---Pickers hide their preview in a narrow editor.
local COLUMNS, LINES = 200, 50
local ACCEPT = "accept"

---@param predicate fun(): any
---@param what string
---@return any
local function wait_for(predicate, what)
  local value
  local ok = vim.wait(WAIT_MS, function()
    value = predicate()
    return value
  end, 10)
  assert(ok, "timed out waiting for " .. what)
  return value
end

---@param buf integer
---@param line string
---@return boolean
local function shows(buf, line)
  return vim.iter(vim.api.nvim_buf_get_lines(buf, 0, -1, false)):any(function(shown)
    return shown:find(line, 1, true) ~= nil
  end)
end

---@type table<games.PickerName, fun(preview_line: string)>
local CONFIRM_FIRST = {
  snacks = function(preview_line)
    local picker = wait_for(function()
      local active = require("snacks").picker.get()[1]
      return active and active:count() > 0 and active.preview.win.buf and active
    end, "the snacks picker")
    wait_for(function()
      return shows(picker.preview.win.buf, preview_line)
    end, "the snacks preview")
    picker:action("confirm")
  end,
  telescope = function(preview_line)
    local action_state = require("telescope.actions.state")
    local prompt_bufnr = vim.api.nvim_get_current_buf()
    local picker = wait_for(function()
      local current = action_state.get_current_picker(prompt_bufnr)
      return current and action_state.get_selected_entry() and current
    end, "the telescope picker")
    wait_for(function()
      local state = picker.previewer and picker.previewer.state
      return state and state.bufnr and shows(state.bufnr, preview_line)
    end, "the telescope preview")
    require("telescope.actions").select_default(prompt_bufnr)
  end,
  fzf = function(preview_line)
    local buf = wait_for(function()
      local current = vim.api.nvim_get_current_buf()
      return vim.bo[current].buftype == "terminal" and shows(current, "2/2") and current
    end, "fzf to list the items")
    wait_for(function()
      return shows(buf, preview_line)
    end, "the fzf preview")
    vim.api.nvim_chan_send(vim.bo[buf].channel, "\r")
  end,
  select = function() end,
}

---Confirms the incoming challenge in picker; requests have no default action, so the select
---that then asks which to run is answered with accept, as is select's own list.
---@param name games.PickerName
local function smoke(name)
  local client, conn = hub.client()
  local requests = sources.build(sources.SourceName.REQUESTS, hub.snapshot(client))
  local preview_line = requests.preview(hub.INCOMING)[1]
  helpers.with_overrides({
    {
      vim.ui,
      "select",
      function(items, opts, on_choice)
        local labels = vim.tbl_map(opts.format_item, items)
        local index = vim.fn.index(labels, requests.format(hub.INCOMING))
        if index < 0 then
          index = vim.fn.index(labels, ACCEPT)
        end
        on_choice(items[index + 1])
      end,
    },
  }, function()
    assert(finder.picker_name(name) == name, name .. " is not installed")
    finder.open(requests, name)
    CONFIRM_FIRST[name](preview_line)
    wait_for(function()
      return hub.sent(conn, "challenge.accept")
    end, name .. " to accept the challenge")
  end)
  helpers.assert_eq(
    { challenge_id = hub.INCOMING.challenge_id },
    hub.sent(conn, "challenge.accept")
  )
end

vim.o.columns, vim.o.lines, vim.o.showmode = COLUMNS, LINES, false
local failed = 0
for _, name in ipairs(finder.PICKER_NAMES) do
  local ok, err = xpcall(smoke, debug.traceback, name)
  io.stdout:write(("%s %s\n"):format(ok and "ok" or "FAIL", name))
  if not ok then
    failed = failed + 1
    io.stderr:write(err .. "\n")
  end
end
os.exit(failed == 0 and 0 or 1)
