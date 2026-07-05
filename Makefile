BINARY := msl
PKG := ./cmd/msl

.PHONY: all build test fmt fmt-check e2e clean

all: test build

build:
	go build -trimpath -ldflags "-s -w" -o bin/$(BINARY) $(PKG)

test:
	go test ./...

fmt:
	gofmt -w cmd internal

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)

e2e: build
	scripts/e2e.sh ./bin/$(BINARY)

clean:
	rm -rf bin dist coverage.out
