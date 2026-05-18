BINARY ?= countdownnow
GO ?= go
GOCACHE ?= /tmp/countdown-go-cache
PROJECT_URL ?= https://github.com/sukujgrg/countdownnow
VERSION ?= $(shell git describe --tags --exact-match HEAD 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || printf dev)

GOFLAGS := -trimpath -buildvcs=false
LDFLAGS := -s -w -X 'main.version=$(VERSION)' -X 'main.projectURL=$(PROJECT_URL)'

.PHONY: build test clean

build:
	GOCACHE="$(GOCACHE)" "$(GO)" build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o "$(BINARY)" .

test:
	GOCACHE="$(GOCACHE)" "$(GO)" test ./...

clean:
	rm -f "$(BINARY)"
