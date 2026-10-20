---@type games.FinderAdapter
return {
  available = function()
    return (pcall(require, "telescope"))
  end,
  open = function(source, handlers)
    local actions = require("telescope.actions")
    local action_state = require("telescope.actions.state")
    local finders = require("telescope.finders")
    local pickers = require("telescope.pickers")
    local previewers = require("telescope.previewers")
    local telescope_config = require("telescope.config").values

    ---@param picked fun(item: any)
    ---@param prompt_bufnr integer
    ---@return fun()
    local function closing(picked, prompt_bufnr)
      return function()
        local entry = action_state.get_selected_entry()
        actions.close(prompt_bufnr)
        if entry then
          vim.schedule(function()
            picked(entry.value)
          end)
        end
      end
    end

    pickers
      .new({}, {
        prompt_title = source.title,
        finder = finders.new_table({
          results = source.items,
          entry_maker = function(item)
            local line = source.format(item)
            return { value = item, display = line, ordinal = line }
          end,
        }),
        sorter = telescope_config.generic_sorter({}),
        previewer = previewers.new_buffer_previewer({
          title = source.title,
          define_preview = function(self, entry)
            local lines = entry and source.preview(entry.value) or {}
            vim.api.nvim_buf_set_lines(self.state.bufnr, 0, -1, false, lines)
          end,
        }),
        attach_mappings = function(prompt_bufnr, map)
          actions.select_default:replace(closing(handlers.confirm, prompt_bufnr))
          map({ "i", "n" }, handlers.choose_key, closing(handlers.choose, prompt_bufnr))
          return true
        end,
      })
      :find()
  end,
}
