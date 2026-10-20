#!/usr/bin/env bash
# Fetches the picker plugins the finder smoke test runs against, at the commits it passed on.
set -euo pipefail

dest="${1:?usage: fetch-pickers.sh <packpath dir>}"
opt="$dest/pack/pickers/opt"
mkdir -p "$opt"

while read -r name repo commit; do
  dir="$opt/$name"
  if [ ! -d "$dir/.git" ]; then
    git init -q "$dir"
    git -C "$dir" remote add origin "https://github.com/$repo"
  fi
  if [ "$(git -C "$dir" rev-parse -q --verify HEAD || true)" != "$commit" ]; then
    git -C "$dir" fetch -q --depth 1 origin "$commit"
    git -C "$dir" checkout -q --detach FETCH_HEAD
  fi
done <<'PINS'
snacks.nvim folke/snacks.nvim 882c996cf28183f4d63640de0b4c02ec886d01f2
telescope.nvim nvim-telescope/telescope.nvim 40aedd8a68c78a656a10a8d62d80c54af59420fb
plenary.nvim nvim-lua/plenary.nvim 74b06c6c75e4eeb3108ec01852001636d85a932b
fzf-lua ibhagwan/fzf-lua a306c293b37bd7c8c1e8e1e6c0d7d3d9c3de3f21
PINS
