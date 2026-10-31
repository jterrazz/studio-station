.PHONY: help install build build-linux lint fix test check provision deprovision skills clean

BINARY       := station
BUILD_PATH   := .artifacts/go/$(BINARY)
LINUX_PATH   := .artifacts/go/$(BINARY)-linux-arm64
BIN_DIR      := $(HOME)/.local/bin
INSTALL_PATH := $(BIN_DIR)/$(BINARY)
LDFLAGS      := -X github.com/jterrazz/studio-station/src/app.version=$(shell git describe --tags --always --dirty)

# hazard: the spine (L1) ships the system adapters, their fakes and the answer path
# before any domain calls them. Each pattern names a function shipped ahead of its
# first caller; the lot that brings the caller deletes its pattern, and L-INT
# deletes this variable, which must then be empty.
DEADCODE_AHEAD := ^src/system/host/|^src/system/systemtest/|unreachable func: (IO\.Answer|Refuse|For|jsonRenderer\.Render|pipeRenderer\.Render|pipeBlock|pipeLine|ttyRenderer\.Render|Writer\.Answer|MarkOf)$$

CYAN  := \033[36m
GREEN := \033[32m
DIM   := \033[2m
RESET := \033[0m

help: ## Show available targets
	@printf "$(CYAN)studio-station$(RESET)\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "$(CYAN)%-12s$(RESET) %s\n", $$1, $$2}'

node_modules/.install: package.json package-lock.json
	@npm ci --silent
	@touch node_modules/.install

install: node_modules/.install ## Install the dependencies of both halves
	@go mod download

build: ## Build the binary, stamped with the version git describes
	@go build -ldflags "$(LDFLAGS)" -o $(BUILD_PATH) ./src/cmd/station
	@printf "$(GREEN)●$(RESET) Built $(BUILD_PATH)\n"

build-linux: ## Build the linux/arm64 binary a VM image carries
	@GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(LINUX_PATH) ./src/cmd/station
	@printf "$(GREEN)●$(RESET) Built $(LINUX_PATH)\n"

lint: node_modules/.install ## Both halves: gofmt, golangci-lint and deadcode over Go, typescript check over the rest
	@unformatted=$$(gofmt -l ./src ./specs); \
		if [ -n "$$unformatted" ]; then printf "gofmt would rewrite:\n%s\n" "$$unformatted"; exit 1; fi
	@go tool golangci-lint run ./src/... ./specs/...
	@dead=$$(go tool deadcode -test ./src/... | grep -v -E '$(DEADCODE_AHEAD)'); \
		if [ -n "$$dead" ]; then printf "deadcode found unreachable functions:\n%s\n" "$$dead"; exit 1; fi
	@printf "$(GREEN)●$(RESET) Go lint passed\n"
	@npm run --silent lint

fix: node_modules/.install ## Every fixer: gofmt over Go, then typescript fix
	@status=0; \
	gofmt -w ./src ./specs || status=1; \
	npx typescript fix || status=1; \
	exit $$status

test: node_modules/.install ## The Go tests (modules and the boundary), then the CLI specs against a fresh build of station
	@go test ./src/... ./specs/...
	@STATION_FORCE_REBUILD=1 npx typescript test

check: lint test build-linux ## Every gate a change passes

provision: build ## Put this build on the machine (~/.local/bin/station), then run station setup
	@mkdir -p $(BIN_DIR)
	@cp $(BUILD_PATH) $(INSTALL_PATH)
	@chmod +x $(INSTALL_PATH)
	@rm -f $(HOME)/.studio/bin/workstation $(HOME)/.studio/bin/station
	@printf "$(GREEN)●$(RESET) Installed $(INSTALL_PATH)\n"
	@$(INSTALL_PATH) setup

deprovision: ## Remove the binary from ~/.local/bin; the user data stays
	@if [ -f "$(INSTALL_PATH)" ]; then \
		rm "$(INSTALL_PATH)"; \
		printf "$(GREEN)●$(RESET) Removed $(INSTALL_PATH)\n"; \
	else \
		printf "$(DIM)$(BINARY) not found at $(INSTALL_PATH)$(RESET)\n"; \
	fi

skills: ## Regenerate the toolbelt skill rosters
	@go run ./src/cmd/skillsgen
	@printf "$(GREEN)●$(RESET) Regenerated skills/studio-toolbelt/SKILL.md\n"

clean: ## Remove build artifacts
	@rm -rf .artifacts
	@printf "$(GREEN)●$(RESET) Cleaned\n"
