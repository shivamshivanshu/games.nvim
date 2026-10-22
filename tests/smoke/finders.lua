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

---@param keys string vim key notation, typed into whatever has focus
local function type_keys(keys)
  vim.api.nvim_feedkeys(vim.keycode(keys), "tx!", false)
end

---@class (private) games.smoke.Driver
---@field ready fun(preview_line: string): any the open picker, once its first preview shows
---@field confirm fun(picker: any) confirms the item under the cursor
---@field choose_second fun(picker: any) moves to the second item and presses the choose key

---@type table<games.PickerName, games.smoke.Driver>
local DRIVERS = {
  snacks = {
    ready = function(preview_line)
      local picker = wait_for(function()
        local active = require("snacks").picker.get()[1]
        return active and active:count() > 0 and active.preview.win.buf and active
      end, "the snacks picker")
      wait_for(function()
        return shows(picker.preview.win.buf, preview_line)
      end, "the snacks preview")
      return picker
    end,
    confirm = function(picker)
      picker:action("confirm")
    end,
    choose_second = function(picker)
      picker:action("list_down")
      wait_for(function()
        return picker:current().idx == 2
      end, "snacks to move to the second item")
      type_keys(finder.CHOOSE_KEY)
    end,
  },
  telescope = {
    ready = function(preview_line)
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
      return picker
    end,
    confirm = function(picker)
      require("telescope.actions").select_default(picker.prompt_bufnr)
    end,
    choose_second = function(picker)
      local action_state = require("telescope.actions.state")
      local first = action_state.get_selected_entry()
      require("telescope.actions").move_selection_next(picker.prompt_bufnr)
      wait_for(function()
        return action_state.get_selected_entry() ~= first
      end, "telescope to move to the second item")
      type_keys(finder.CHOOSE_KEY)
    end,
  },
  fzf = {
    ready = function(preview_line)
      local buf = wait_for(function()
        local current = vim.api.nvim_get_current_buf()
        return vim.bo[current].buftype == "terminal" and shows(current, "2/2") and current
      end, "fzf to list the items")
      wait_for(function()
        return shows(buf, preview_line)
      end, "the fzf preview")
      return buf
    end,
    confirm = function(buf)
      vim.api.nvim_chan_send(vim.bo[buf].channel, "\r")
    end,
    choose_second = function(buf)
      local down, alt_enter = "\27[B", "\27\r"
      vim.api.nvim_chan_send(vim.bo[buf].channel, down)
      vim.wait(200)
      vim.api.nvim_chan_send(vim.bo[buf].channel, alt_enter)
    end,
  },
  select = {
    ready = function() end,
    confirm = function() end,
    choose_second = function() end,
  },
}

---@class (private) games.smoke.Case
---@field item games.Challenge the item the case picks
---@field action games.SourceActionName what the action choice is answered with
---@field message_type string the request the action sends
---@field run fun(driver: games.smoke.Driver, picker: any)

---@type games.smoke.Case[]
local CASES = {
  {
    item = hub.INCOMING,
    action = "accept",
    message_type = "challenge.accept",
    run = function(driver, picker)
      driver.confirm(picker)
    end,
  },
  {
    item = hub.OUTGOING,
    action = "cancel",
    message_type = "challenge.cancel",
    run = function(driver, picker)
      driver.choose_second(picker)
    end,
  },
}

---Opens the requests finder in picker, with the outgoing challenge previewing nothing, and runs
---case; the action choice and select's own list are answered with the case's item and action.
---@param name games.PickerName
---@param case games.smoke.Case
local function smoke(name, case)
  local client, conn = hub.client()
  local requests = sources.build(sources.SourceName.REQUESTS, hub.snapshot(client))
  local source = vim.tbl_extend("force", requests, {
    preview = function(item)
      return item ~= hub.OUTGOING and requests.preview(item) or nil
    end,
  })
  helpers.with_overrides({
    {
      vim.ui,
      "select",
      function(items, opts, on_choice)
        local labels = vim.tbl_map(opts.format_item, items)
        local index = vim.fn.index(labels, source.format(case.item))
        if index < 0 then
          index = vim.fn.index(labels, case.action)
        end
        on_choice(items[index + 1])
      end,
    },
  }, function()
    assert(finder.picker_name(name) == name, name .. " is not installed")
    finder.open(source, name)
    local driver = DRIVERS[name]
    case.run(driver, driver.ready(source.preview(hub.INCOMING)[1]))
    wait_for(function()
      return conn:sent(case.message_type)
    end, ("%s to %s the challenge"):format(name, case.action))
  end)
  helpers.assert_eq({ challenge_id = case.item.challenge_id }, conn:sent(case.message_type))
end

vim.o.columns, vim.o.lines, vim.o.showmode = COLUMNS, LINES, false
local failed = 0
for _, name in ipairs(finder.PICKER_NAMES) do
  for _, case in ipairs(CASES) do
    local ok, err = xpcall(smoke, debug.traceback, name, case)
    io.stdout:write(("%s %s %s\n"):format(ok and "ok" or "FAIL", name, case.action))
    if not ok then
      failed = failed + 1
      io.stderr:write(err .. "\n")
    end
  end
end
os.exit(failed == 0 and 0 or 1)
