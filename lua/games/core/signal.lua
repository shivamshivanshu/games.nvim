---@class games.Signal
---@field connect fun(fn: function): games.Unsubscribe
---@field emit fun(...): string[] calls every handler with ..., even past one that raises, and returns what they raised
---@field handlers fun(): function[] those connected now, in connect order; never edited afterwards

local Signal = {}

---A list of handlers that emit calls in connect order. Connecting or unsubscribing replaces the
---list rather than editing it, so an emit in progress calls the handlers it started with.
---@return games.Signal
function Signal.new()
  local handlers = {}
  return {
    connect = function(fn)
      handlers = vim.list_extend(vim.list_slice(handlers), { fn })
      return function()
        handlers = vim.tbl_filter(function(handler)
          return handler ~= fn
        end, handlers)
      end
    end,
    emit = function(...)
      local errors = {}
      for _, fn in ipairs(handlers) do
        local ok, err = pcall(fn, ...)
        if not ok then
          errors[#errors + 1] = tostring(err)
        end
      end
      return errors
    end,
    handlers = function()
      return handlers
    end,
  }
end

return Signal
