local repo_root = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")
vim.opt.runtimepath:prepend(repo_root)
package.path = repo_root .. "/tests/lua/?.lua;" .. package.path

local spec_files = vim.fn.glob(repo_root .. "/tests/lua/*_spec.lua", false, true)
table.sort(spec_files)

local failures = {}
local passed = 0

for _, file in ipairs(spec_files) do
  local spec_name = vim.fn.fnamemodify(file, ":t:r")
  local cases = dofile(file)
  local names = vim.tbl_keys(cases)
  table.sort(names)
  for _, name in ipairs(names) do
    local ok, err = xpcall(cases[name], debug.traceback)
    if ok then
      passed = passed + 1
    else
      table.insert(failures, ("%s.%s\n%s"):format(spec_name, name, err))
    end
  end
end

for _, failure in ipairs(failures) do
  io.stderr:write("FAIL " .. failure .. "\n\n")
end
io.stdout:write(("%d passed, %d failed\n"):format(passed, #failures))
os.exit(#failures == 0 and 0 or 1)
