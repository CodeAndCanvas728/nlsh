BINARY := nlsh
PKG := ./cmd/nlsh
VERSION := $(shell sed -n 's/^const Version = "\(.*\)"/\1/p' internal/cli/usage.go)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test race vet fmt fmt-check lint check install clean release-dry-run

all: check build

## build: compile the binary for this machine
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

## test: run the test suite
test:
	go test ./...

## race: run the test suite with the race detector
race:
	go test -race ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: format the source
fmt:
	gofmt -w .

## fmt-check: fail when the source is not formatted
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not formatted:"; echo "$$unformatted"; exit 1; \
	fi

## check: everything CI runs
check: fmt-check vet race test

## install: install the binary into GOBIN
install:
	go install -trimpath -ldflags "$(LDFLAGS)" $(PKG)

## clean: remove build output
clean:
	rm -f $(BINARY)
	go clean -testcache
