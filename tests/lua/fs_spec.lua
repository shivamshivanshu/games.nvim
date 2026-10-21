local fs = require("games.core.fs")
local helpers = require("helpers")

local assert_eq = helpers.assert_eq

---Runs each task in its own fs.run at once and waits for all of them.
---@param tasks (fun(): any)[]
---@return {ok: boolean, result: any}[]
local function run_together(tasks)
  local outcomes, finished = {}, 0
  for i, task in ipairs(tasks) do
    fs.run(task, function(ok, result)
      outcomes[i] = { ok = ok, result = result }
      finished = finished + 1
    end)
  end
  helpers.wait_for(function()
    return finished == #tasks
  end, "the tasks")
  return outcomes
end

local T = {}

function T.writes_to_one_path_at_once_each_use_their_own_temp_file()
  helpers.with_temp_dir(function(dir)
    vim.fn.mkdir(dir, "p")
    local path = dir .. "/set.json"
    local first, second = ("a"):rep(100000), ("b"):rep(100000)
    local outcomes = run_together({
      function()
        fs.write_atomic(path, first)
      end,
      function()
        fs.write_atomic(path, second)
      end,
    })
    assert_eq({ { ok = true }, { ok = true } }, outcomes)
    local written = helpers.read_file(path)
    assert(written == first or written == second, "a write was torn")
    assert_eq({ "set.json" }, vim.fn.readdir(dir))
  end)
end

function T.lines_reads_a_file_a_chunk_at_a_time_without_its_line_breaks()
  helpers.with_temp_dir(function(dir)
    vim.fn.mkdir(dir, "p")
    local path = dir .. "/big.csv"
    local want = {}
    for n = 1, 20000 do
      want[n] = ("row %d,%s"):format(n, ("x"):rep(n % 7))
    end
    local file = assert(io.open(path, "wb"))
    file:write(table.concat(want, "\r\n"))
    file:close()
    local outcomes = run_together({
      function()
        local got = {}
        for line in assert(fs.lines(path)) do
          got[#got + 1] = line
        end
        return got
      end,
    })
    assert_eq({ { ok = true, result = want } }, outcomes)
    assert_eq(nil, fs.lines(dir .. "/missing.csv"))
  end)
end

function T.outside_fs_run_the_calls_finish_before_they_return()
  helpers.with_temp_dir(function(dir)
    local path = dir .. "/sub/value"
    fs.mkdir_p(dir .. "/sub")
    fs.write_atomic(path, "one")
    assert_eq("one", fs.read(path))
    assert_eq(nil, fs.read(dir .. "/missing"))
  end)
end

return T
