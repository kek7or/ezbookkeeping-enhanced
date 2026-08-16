# ezBookkeeping development tasks.
#
# Windows + Git Bash. Run "make" with no arguments for the target list.
#
# The sqlite3 driver needs cgo, so the backend cannot be built without a C
# compiler. MINGW_BIN points at the mingw-w64 that winget installs; it is
# deliberately not on the global PATH, it is prepended per recipe instead.

SHELL := C:/Program Files/Git/bin/bash.exe
.SHELLFLAGS := -c

# GNU Make on Windows runs simple commands through CreateProcess directly instead
# of through SHELL, so mkdir/rm/tail/awk must be resolvable on PATH. They ship
# with Git for Windows but are only on PATH inside Git Bash - putting them there
# explicitly is what lets these targets work from PowerShell and cmd too.
GIT_USR_BIN := C:/Program Files/Git/usr/bin
export PATH := $(GIT_USR_BIN);$(PATH)

MINGW_BIN := C:/Users/Viktor/AppData/Local/Microsoft/WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe/mingw64/bin
OLLAMA    := C:/Users/Viktor/AppData/Local/Programs/Ollama/ollama.exe

BINARY       := ezbookkeeping.exe
API_PORT     := 4242
WEB_PORT     := 5173
OLLAMA_PORT  := 11434
OLLAMA_MODEL ?= qwen3-vl-16k

# CC is set explicitly rather than prepending to PATH: PATH is ";"-separated on Windows but
# ":"-separated inside bash, and mixing the two silently produces an unusable entry.
GO_BUILD := CGO_ENABLED=1 CC="$(MINGW_BIN)/gcc.exe" go build -tags timetzdata

# Stop whatever holds a TCP port. Killing by port rather than by pid on purpose:
# "npm run serve" exits without taking its vite child with it, orphaning the port.
kill_port = powershell -NoProfile -Command 'Get-NetTCPConnection -LocalPort $(1) -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $$_.OwningProcess -Force -ErrorAction SilentlyContinue }' >/dev/null 2>&1 || true

# Background services are started with Start-Process, not "nohup cmd &": the
# backgrounded job keeps make's stdout pipe open, so make never returns even
# though the service is up. Start-Process detaches and hands the terminal back.

.DEFAULT_GOAL := help

## ---------------------------------------------------------------- help

.PHONY: help
help: ## Show this help
	@echo "ezBookkeeping development tasks"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  App:    http://localhost:$(WEB_PORT)  (dev, hot reload)"
	@echo "  API:    http://localhost:$(API_PORT)"
	@echo "  Ollama: http://localhost:$(OLLAMA_PORT)   model: $(OLLAMA_MODEL)"

## ---------------------------------------------------------------- setup

.PHONY: setup
setup: node-deps dirs db ## First run: install deps, create runtime dirs, init database

.PHONY: node-deps
node-deps: ## Install frontend dependencies
	npm install --no-audit --no-fund

.PHONY: dirs
dirs: ## Create the runtime directories the server refuses to start without
	@mkdir -p storage data log
	@echo "created storage/ data/ log/"

.PHONY: db
db: $(BINARY) ## Create or migrate the database schema
	./$(BINARY) database update

## ---------------------------------------------------------------- build

.PHONY: build
build: build-backend build-frontend ## Build backend binary and frontend bundle

$(BINARY): $(shell find pkg cmd -name '*.go' 2>/dev/null) ezbookkeeping.go
	@$(MAKE) --no-print-directory build-backend

.PHONY: build-backend
build-backend: ## Build the backend binary (needs gcc for cgo/sqlite3)
	@test -x "$(MINGW_BIN)/gcc.exe" || { \
		echo "gcc not found at $(MINGW_BIN)"; \
		echo "install it with: winget install --id BrechtSanders.WinLibs.POSIX.UCRT"; \
		exit 1; }
	$(GO_BUILD) -o $(BINARY) ezbookkeeping.go
	@echo "built $(BINARY)"

.PHONY: build-frontend
build-frontend: ## Build the frontend bundle into dist/
	npm run build

.PHONY: package
package: build ## Build, then stage dist/ into public/ so the binary serves the UI itself
	@rm -rf public/css public/js public/fonts public/*.html public/manifest.json
	@cp -r dist/* public/
	@echo "staged dist/ into public/ - the binary now serves the UI on :$(API_PORT)"

## ---------------------------------------------------------------- run

.PHONY: server
server: $(BINARY) dirs ## Run the backend in the foreground (:4242)
	./$(BINARY) server run

.PHONY: web
web: ## Run the Vite dev server in the foreground (:5173)
	npm run serve

# Everything the app needs is started and stopped together, because receipt
# recognition is not optional to how this is used and an app whose Ollama is
# down fails at the moment it is being relied on. The halves are still
# available on their own as app-up / app-down and ollama-up / ollama-down.

.PHONY: up
up: ollama-up app-up ## Start Ollama, backend and frontend in the background
	@$(MAKE) --no-print-directory status
	@echo ""
	@echo "open http://localhost:$(WEB_PORT)   (logs: make logs)"

.PHONY: down
down: app-down ollama-down ## Stop Ollama, backend and frontend

.PHONY: app-up
app-up: $(BINARY) dirs ## Start backend and frontend only
	@$(call kill_port,$(API_PORT))
	@$(call kill_port,$(WEB_PORT))
	@powershell -NoProfile -Command "Start-Process -FilePath './$(BINARY)' -ArgumentList 'server','run' -RedirectStandardOutput 'log/server.out' -RedirectStandardError 'log/server.err' -WindowStyle Hidden"
	@powershell -NoProfile -Command "Start-Process -FilePath 'npm.cmd' -ArgumentList 'run','serve' -RedirectStandardOutput 'log/web.out' -RedirectStandardError 'log/web.err' -WindowStyle Hidden"
	@$(MAKE) --no-print-directory wait

.PHONY: app-down
app-down: ## Stop backend and frontend only
	@$(call kill_port,$(API_PORT))
	@$(call kill_port,$(WEB_PORT))
	@echo "stopped :$(API_PORT) and :$(WEB_PORT)"

.PHONY: restart
restart: down up ## Restart everything

# Waiting for the port to answer rather than sleeping a fixed six seconds: vite
# is usually up in two and the backend takes longer after a rebuild, so a fixed
# wait is either too slow or a lie about being ready.
.PHONY: wait
wait: ## Block until the web server answers (up to 40s)
	@for i in $$(seq 1 40); do \
		curl -s -m 1 -o /dev/null http://127.0.0.1:$(WEB_PORT)/ 2>/dev/null && exit 0; \
		sleep 1; \
	done; \
	echo "web server did not come up within 40s - see log/web.err"

.PHONY: status
status: ## Show which services are listening
	@printf "  backend :%s  %s\n" "$(API_PORT)" "$$(curl -s -m 2 -o /dev/null -w '%{http_code}' http://127.0.0.1:$(API_PORT)/server_settings.js 2>/dev/null | grep -q 200 && echo up || echo down)"
	@printf "  web     :%s  %s\n" "$(WEB_PORT)" "$$(curl -s -m 2 -o /dev/null -w '%{http_code}' http://127.0.0.1:$(WEB_PORT)/ 2>/dev/null | grep -q 200 && echo up || echo down)"
	@printf "  ollama  :%s %s\n" "$(OLLAMA_PORT)" "$$(curl -s -m 2 -o /dev/null -w '%{http_code}' http://127.0.0.1:$(OLLAMA_PORT)/api/tags 2>/dev/null | grep -q 200 && echo up || echo down)"

.PHONY: url
url: ## Print the app url, so a launcher never has to repeat the port
	@echo "http://localhost:$(WEB_PORT)"

.PHONY: logs
logs: ## Tail the backend log
	@tail -f log/server.out

## ---------------------------------------------------------------- ollama

.PHONY: ollama-up
ollama-up: ## Start the Ollama server in the background
	@mkdir -p log
	@curl -s -m 2 -o /dev/null http://127.0.0.1:$(OLLAMA_PORT)/api/tags \
		&& echo "ollama already running on :$(OLLAMA_PORT)" \
		|| { powershell -NoProfile -Command "Start-Process -FilePath '$(OLLAMA)' -ArgumentList 'serve' -RedirectStandardOutput 'log/ollama.out' -RedirectStandardError 'log/ollama.err' -WindowStyle Hidden"; \
		     sleep 4; echo "started ollama on :$(OLLAMA_PORT)"; }

.PHONY: ollama-down
ollama-down: ## Stop the Ollama server
	@$(call kill_port,$(OLLAMA_PORT))
	@echo "stopped ollama on :$(OLLAMA_PORT)"

.PHONY: ollama-models
ollama-models: ## List locally available models
	@"$(OLLAMA)" list

.PHONY: ollama-pull
ollama-pull: ## Pull OLLAMA_MODEL (override: make ollama-pull OLLAMA_MODEL=...)
	@"$(OLLAMA)" pull $(OLLAMA_MODEL)

.PHONY: ollama-logs
ollama-logs: ## Tail the Ollama log
	@tail -f log/ollama.out

## ---------------------------------------------------------------- checks

.PHONY: test
test: test-go test-web ## Run backend and frontend tests

.PHONY: test-go
test-go: ## Run the Go test suite
	go test ./...

.PHONY: test-web
test-web: ## Run the frontend test suite
	npm test

.PHONY: lint
lint: ## Typecheck and lint the frontend
	npm run lint

.PHONY: vet
vet: ## Run go vet
	go vet ./...

## ---------------------------------------------------------------- clean

.PHONY: clean
clean: ## Remove build output
	@rm -rf dist $(BINARY)
	@echo "removed dist/ and $(BINARY)"

.PHONY: clean-data
clean-data: down ## Delete the database, uploads and logs (DESTRUCTIVE)
	@rm -rf data log storage
	@echo "removed data/ log/ storage/ - run 'make setup' to start over"
