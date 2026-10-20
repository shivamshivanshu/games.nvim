LUALS ?= $(HOME)/.local/share/nvim/mason/bin/lua-language-server
LUA_DIRS := $(wildcard lua tests/lua)
HAS_GO_PACKAGES := $(shell cd server && go list ./... 2>/dev/null)

.PHONY: fmt lint test-go build test-lua check

fmt:
	cd server && gofmt -w .
	$(if $(LUA_DIRS),stylua $(LUA_DIRS))

lint:
	@test -z "$$(cd server && gofmt -l .)" || { cd server && gofmt -l .; exit 1; }
	$(if $(HAS_GO_PACKAGES),cd server && go vet ./...)
	$(if $(LUA_DIRS),stylua --check $(LUA_DIRS))
	$(if $(wildcard lua),VIMRUNTIME="$$(nvim --headless -u NONE -c 'echo $$VIMRUNTIME' -c q 2>&1)" $(LUALS) --check lua --configpath $(CURDIR)/.luarc.json --checklevel=Warning)

test-go:
	$(if $(HAS_GO_PACKAGES),cd server && go test -race -short ./...)
	$(if $(wildcard server/games/chess),cd server && go test -run TestPerft ./games/chess/)

build:
	cd server && go build -o ../bin/games-server ./cmd/games-server

test-lua: $(if $(wildcard server/cmd/games-server),build)
	$(if $(wildcard tests/lua),nvim --headless -u NONE -l tests/lua/run.lua)

check: lint test-go test-lua
