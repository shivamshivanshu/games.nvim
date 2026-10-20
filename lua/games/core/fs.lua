local uv = vim.uv

local DIR_MODE = tonumber("755", 8)
local FILE_MODE = tonumber("644", 8)

local M = {}

---Starts an async libuv call; the driver passes the callback that resumes the task.
---@alias games.fs.Start fun(resume: fun(err: string|nil, result: any))

---@param start games.fs.Start
---@return string|nil err
---@return any result
local function await(start)
  assert(coroutine.running(), "games.fs: call from inside fs.run")
  return coroutine.yield(start)
end

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

---Runs task in a coroutine in which the functions below may be called, resuming it on the main loop
---after each libuv callback. on_done gets the task's return value, or its error message.
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

---@param path string
---@return string|nil err
local function mkdir(path)
  return await(function(resume)
    uv.fs_mkdir(path, DIR_MODE, resume)
  end)
end

---Creates path and its missing parents.
---@param path string absolute
function M.mkdir_p(path)
  local err = mkdir(path)
  if is_error(err, "ENOENT") then
    M.mkdir_p(vim.fs.dirname(path))
    err = mkdir(path)
  end
  if not is_error(err, "EEXIST") then
    raise(err)
  end
end

---@param path string
---@param flags string
---@return integer fd
local function open(path, flags)
  local err, fd = await(function(resume)
    uv.fs_open(path, flags, FILE_MODE, resume)
  end)
  raise(err)
  return fd
end

---@param fd integer
local function close(fd)
  raise(await(function(resume)
    uv.fs_close(fd, resume)
  end))
end

---@param fd integer
---@param data string
local function write_all(fd, data)
  raise(await(function(resume)
    uv.fs_write(fd, data, -1, resume)
  end))
end

---The file's bytes, or nil when it does not exist.
---@param path string
---@return string|nil
function M.read(path)
  local err, fd = await(function(resume)
    uv.fs_open(path, "r", FILE_MODE, resume)
  end)
  if is_error(err, "ENOENT") then
    return nil
  end
  raise(err)
  local stat_err, stat = await(function(resume)
    uv.fs_fstat(fd, resume)
  end)
  local read_err, data
  if stat then
    read_err, data = await(function(resume)
      uv.fs_read(fd, stat.size, 0, resume)
    end)
  end
  close(fd)
  raise(stat_err or read_err)
  return data
end

---Replaces path with data through a temp file in the same directory, so readers never see a
---partial file.
---@param path string
---@param data string
function M.write_atomic(path, data)
  local temp = ("%s.%d.tmp"):format(path, uv.os_getpid())
  local fd = open(temp, "w")
  local _, err = pcall(function()
    write_all(fd, data)
    raise(await(function(resume)
      uv.fs_fsync(fd, resume)
    end))
  end)
  local close_err = await(function(resume)
    uv.fs_close(fd, resume)
  end)
  err = err or close_err
  if not err then
    err = await(function(resume)
      uv.fs_rename(temp, path, resume)
    end)
  end
  if err then
    await(function(resume)
      uv.fs_unlink(temp, resume)
    end)
    error(err, 0)
  end
end

---@param path string
---@param data string
function M.append(path, data)
  local fd = open(path, "a")
  local ok, err = pcall(write_all, fd, data)
  close(fd)
  if not ok then
    error(err, 0)
  end
end

return M
