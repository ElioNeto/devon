BINARY   := devon
CMD      := ./cmd/devon
OUT      := ./bin/$(BINARY)
LDFLAGS  := -ldflags "-s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)"

.PHONY: build run test lint install clean build-all release-dry

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(OUT) $(CMD)

run:
	go run $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

install: build
	cp $(OUT) ~/.local/bin/$(BINARY)

clean:
	rm -rf bin/

build-all:
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 $(CMD)
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64 $(CMD)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 $(CMD)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 $(CMD)

release-dry:
	goreleaser release --clean --snapshot
