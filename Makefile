VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X github.com/ash0x0/csm/cmd.version=$(VERSION)"

.PHONY: build build-tui test vet lint install clean

build-tui:
	cd tui && npm install --frozen-lockfile && npm run build
	mkdir -p internal/tuibundle/dist
	cp tui/dist/index.js internal/tuibundle/dist/index.js

build: build-tui
	go build $(LDFLAGS) -o csm .

test: build-tui
	cd tui && npm test
	go test -race -cover ./...

vet:
	go vet ./...

lint: vet
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

install: build
	cp csm $(GOPATH)/bin/csm 2>/dev/null || cp csm ~/.local/bin/csm

clean:
	rm -f csm coverage.out
	rm -rf tui/dist tui/node_modules

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser"
