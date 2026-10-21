local uv = vim.uv

local DIR_MODE = tonumber("755", 8)
local FILE_MODE = tonumber("644", 8)
local CHUNK_BYTES = 64 * 1024
---What vim.uv.fs_mkstemp replaces with a name no other temp file has.
local TEMP_SUFFIX = ".XXXXXX"

local M = {}

---@param err string|nil
---@param code string e.g. "ENOENT"
---@return boolean
local function is_error(err, code)
  return err ~= nil and vim.startswith(err, code .. ":")
end

---@param err string|nil
local function raise(err)
  if err then
    error(err, 0)
  end
end

---@return boolean
local function in_task()
  local co, is_main = coroutine.running()
  return co ~= nil and not is_main
end

---Calls the vim.uv fs function fn with args: inside fs.run asynchronously, resuming the task
---when it is done, and outside it synchronously.
---@param fn function
---@param ... any
---@return string|nil err
---@return any ... fn's results
local function call(fn, ...)
  local n, args = select("#", ...), { ... }
  if not in_task() then
    local results = { fn(...) }
    if results[1] == nil and results[2] ~= nil then
      return results[2]
    end
    return nil, unpack(results)
  end
  return coroutine.yield(function(resume)
    args[n + 1] = resume
    fn(unpack(args, 1, n + 1))
  end)
end

---Runs task in a coroutine in which the functions below run asynchronously, resuming it on the main
---loop after each libuv callback. on_done gets the task's return value, or its error message.
---@param task fun(): any
---@param on_done fun(ok: boolean, result: any)
function M.run(task, on_done)
  local co = coroutine.create(task)
  local function step(...)
    local resumed, yielded = coroutine.resume(co, ...)
    if not resumed then
      on_done(false, yielded)
      return
    end
    if coroutine.status(co) == "dead" then
      on_done(true, yielded)
      return
    end
    local started, start_err = pcall(yielded, vim.schedule_wrap(step))
    if not started then
      on_done(false, start_err)
    end
  end
  step()
end

---Creates path and its missing parents.
---@param path string absolute
function M.mkdir_p(path)
  local err = call(uv.fs_mkdir, path, DIR_MODE)
  if is_error(err, "ENOENT") then
    M.mkdir_p(vim.fs.dirname(path))
    err = call(uv.fs_mkdir, path, DIR_MODE)
  end
  if not is_error(err, "EEXIST") then
    raise(err)
  end
end

---@param fd integer
---@param data string
local function write_all(fd, data)
  raise(call(uv.fs_write, fd, data, -1))
end

---The file's bytes, or nil when it does not exist.
---@param path string
---@return string|nil
function M.read(path)
  local err, fd = call(uv.fs_open, path, "r", FILE_MODE)
  if is_error(err, "ENOENT") then
    return nil
  end
  raise(err)
  local stat_err, stat = call(uv.fs_fstat, fd)
  local read_err, data
  if stat then
    read_err, data = call(uv.fs_read, fd, stat.size, 0)
  end
  raise(call(uv.fs_close, fd))
  raise(stat_err or read_err)
  return data
end

---The lines of the file at path, without their line breaks, read a chunk at a time, and a close
---for a caller that stops early; nil when it does not exist. The file is closed once the last
---line is read.
---@param path string
---@return (fun(): string|nil)|nil
---@return fun() close
function M.lines(path)
  local err, fd = call(uv.fs_open, path, "r", FILE_MODE)
  if is_error(err, "ENOENT") then
    return nil, function() end
  end
  raise(err)
  local lines, index, partial, at_end = {}, 0, "", false
  local function close()
    if not at_end then
      at_end = true
      raise(call(uv.fs_close, fd))
    end
  end
  local function fill()
    local read_err, chunk = call(uv.fs_read, fd, CHUNK_BYTES, -1)
    if read_err then
      at_end = true
      call(uv.fs_close, fd)
      error(read_err, 0)
    end
    if chunk == "" then
      close()
      lines, index = partial ~= "" and { partial } or {}, 0
      return
    end
    lines, index = vim.split(partial .. chunk, "\n", { plain = true }), 0
    partial = table.remove(lines)
  end
  return function()
    while index == #lines do
      if at_end then
        return nil
      end
      fill()
    end
    index = index + 1
    return (lines[index]:gsub("\r$", ""))
  end,
    close
end

---Replaces path with data through a temp file of its own in the same directory, so readers never
---see a partial file.
---@param path string
---@param data string
function M.write_atomic(path, data)
  local temp_err, fd, temp = call(uv.fs_mkstemp, path .. TEMP_SUFFIX)
  raise(temp_err)
  local _, err = pcall(function()
    raise(call(uv.fs_fchmod, fd, FILE_MODE))
    write_all(fd, data)
    raise(call(uv.fs_fsync, fd))
  end)
  local close_err = call(uv.fs_close, fd)
  err = err or close_err or call(uv.fs_rename, temp, path)
  if err then
    call(uv.fs_unlink, temp)
    error(err, 0)
  end
end

---@param path string
---@param data string
function M.append(path, data)
  local open_err, fd = call(uv.fs_open, path, "a", FILE_MODE)
  raise(open_err)
  local ok, err = pcall(write_all, fd, data)
  raise(call(uv.fs_close, fd))
  if not ok then
    error(err, 0)
  end
end

return M
