local controls = require("games.core.controls")

local M = {}

---Reply callback that reports request errors and hands successful results on, if asked to.
---@generic R
---@param on_result? fun(result: R)
---@return games.Reply<R>
function M.on_ok(on_result)
  return function(err, result)
    if err then
      controls.report_message(err)
      return
    end
    if on_result then
      on_result(result)
    end
  end
end

return M
