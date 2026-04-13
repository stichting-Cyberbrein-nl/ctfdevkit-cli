BINARY   := devkit
MODULE   := github.com/stichting-Cyberbrein-nl/ctfdevkit-cli
VERSION  ?= dev
LDFLAGS  := -ldflags "-X main.version=$(VERSION) -s -w"
OUT      := dist

.PHONY: all build build-linux build-darwin build-windows clean test lint

all: build

## build — cross-compile for all supported platforms
build: build-linux build-darwin build-windows

build-linux:
	@mkdir -p $(OUT)
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(OUT)/$(BINARY)-linux-amd64   ./cmd/devkit
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(OUT)/$(BINARY)-linux-arm64   ./cmd/devkit

build-darwin:
	@mkdir -p $(OUT)
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(OUT)/$(BINARY)-darwin-amd64  ./cmd/devkit
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(OUT)/$(BINARY)-darwin-arm64  ./cmd/devkit

build-windows:
	@mkdir -p $(OUT)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(OUT)/$(BINARY)-windows-amd64.exe ./cmd/devkit

## test — run all tests (forces amd64 so platform detection works locally)
test:
	GOARCH=amd64 go test ./...

## clean — remove build artefacts
clean:
	rm -rf $(OUT)

## lint — run go vet
lint:
	go vet ./...
