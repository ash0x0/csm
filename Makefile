VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X github.com/ash0x0/csm/cmd.version=$(VERSION)"

.PHONY: build test vet lint install clean

build:
	go build $(LDFLAGS) -o csm .

test:
	go test -race -cover ./...

vet:
	go vet ./...

lint: vet
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

install: build
	cp csm $(GOPATH)/bin/csm 2>/dev/null || cp csm ~/.local/bin/csm

clean:
	rm -f csm coverage.out
	rm -rf dist/

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser"
