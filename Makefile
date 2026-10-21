LUALS ?= $(HOME)/.local/share/nvim/mason/bin/lua-language-server
LUA_DIRS := $(wildcard lua tests/lua tests/smoke)
HAS_GO_PACKAGES := $(shell cd server && go list ./... 2>/dev/null)

SCHEMA_DIR := $(CURDIR)/docs/schema
PICKERS_PACK ?= $(CURDIR)/.pickers
IMAGE ?= games-server:latest

.PHONY: fmt lint schema schema-check test-go build docker test-lua check smoke-finders

fmt:
	cd server && gofmt -w .
	$(if $(LUA_DIRS),stylua $(LUA_DIRS))

lint:
	@test -z "$$(cd server && gofmt -l .)" || { cd server && gofmt -l .; exit 1; }
	$(if $(HAS_GO_PACKAGES),cd server && go vet ./...)
	$(if $(LUA_DIRS),stylua --check $(LUA_DIRS))
	$(if $(wildcard lua),VIMRUNTIME="$$(nvim --headless -u NONE -c 'echo $$VIMRUNTIME' -c q 2>&1)" $(LUALS) --check lua --configpath $(CURDIR)/.luarc.json --checklevel=Warning)

schema:
	cd server && go run ./cmd/schemagen -out $(SCHEMA_DIR)

schema-check:
	cd server && go run ./cmd/schemagen -check $(SCHEMA_DIR)

test-go:
	$(if $(HAS_GO_PACKAGES),cd server && go test -race -short ./...)
	$(if $(wildcard server/games/chess),cd server && go test -run TestPerft ./games/chess/)

build:
	cd server && go build -o ../bin/games-server ./cmd/games-server

docker:
	docker build -t $(IMAGE) .

test-lua: $(if $(wildcard server/cmd/games-server),build)
	$(if $(wildcard tests/lua),nvim --headless -u NONE -l tests/lua/run.lua)

check: lint schema-check test-go test-lua

smoke-finders:
	@echo "fzf $$(fzf --version)"
	tests/smoke/fetch-pickers.sh $(PICKERS_PACK)
	PICKERS_PACK=$(PICKERS_PACK) nvim --headless -u NONE -l tests/smoke/finders.lua
