UI_DIR := ui
SERVER_BIN := bin/curtain-server
CLI_BIN := bin/curtain

.PHONY: ui build build-server build-cli run dev-api dev-ui check test tidy clean

## ui: install dependencies if needed and build the SPA into ui/build
ui:
	cd $(UI_DIR) && npm install && npm run build

## build: build the SPA, server binary, and CLI binary
build: ui build-server build-cli

## build-server: build the server binary (API + embedded UI) -> bin/curtain-server
build-server:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(SERVER_BIN) ./cmd/api

## build-cli: build the CLI client -> bin/curtain (pure Go, no CGO)
build-cli:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(CLI_BIN) ./cmd/cli

## run: build and run the server
run: build
	./$(SERVER_BIN)

## dev-api: run only the Go API (serve the UI through the Vite dev server)
dev-api:
	go run ./cmd/api

## dev-ui: run the SvelteKit dev server with /api proxied to :8080
dev-ui:
	cd $(UI_DIR) && npm run dev

## check: run go vet and svelte-check
check:
	go vet ./...
	cd $(UI_DIR) && npm run check

## test: run Go unit and integration tests (including CLI crypto interoperability)
test:
	go test ./...

## tidy: tidy Go and Node dependencies
tidy:
	go mod tidy
	cd $(UI_DIR) && npm install

clean:
	rm -rf bin $(UI_DIR)/build
