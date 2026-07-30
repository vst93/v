# v - Gadgets under the terminal
#
# VERSION defaults to "dev", which is what a plain `go build .` also reports.
# Release builds inject the real version:
#
#   make build VERSION=0.0.6
#   make release VERSION=0.0.6
#
# CI passes the release tag with any leading "v" stripped.

BINARY  := v
PKG     := .
VERSION ?= dev
LDFLAGS := -w -s -X v/service.VVersion=$(VERSION)
GOFLAGS := -trimpath

OUTPUT  := output

# GOOS/GOARCH pairs built for a release (windows/arm64 excluded, matching CI).
PLATFORMS := \
	darwin/amd64 darwin/arm64 \
	linux/amd64 linux/arm64 \
	windows/amd64 \
	android/amd64 android/arm64

.PHONY: all build dev test lint fmt install release clean help

all: build

## build: build for the host platform (VERSION=dev unless overridden)
build:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)
	@echo "built $(BINARY) version=$(VERSION)"

## dev: build with no version injection at all (reports "dev")
dev:
	go build -o $(BINARY) $(PKG)

## test: vet + full test suite (the gate CI runs before publishing)
test:
	go vet ./...
	go test ./...

## lint: vet only
lint:
	go vet ./...

## fmt: gofmt the tree
fmt:
	gofmt -w .

## install: build and install into GOBIN (or ~/go/bin)
install:
	CGO_ENABLED=0 go install $(GOFLAGS) -ldflags "$(LDFLAGS)" $(PKG)

## release: cross-compile every platform into ./output, zip + sha256
##          Android needs the NDK: set ANDROID_NDK_LATEST_HOME.
release: clean
	@mkdir -p $(OUTPUT)
	@for p in $(PLATFORMS); do \
		goos=$${p%/*}; goarch=$${p#*/}; \
		bin="$(BINARY)"; \
		[ "$$goos" = "windows" ] && bin="$(BINARY).exe"; \
		cgo=0; cc=""; \
		if [ "$$goos" = "android" ]; then \
			cgo=1; \
			case "$$goarch" in \
				arm64) cc=aarch64-linux-android32-clang ;; \
				amd64) cc=x86_64-linux-android32-clang ;; \
			esac; \
			if [ -z "$$ANDROID_NDK_LATEST_HOME" ]; then \
				echo "skip android/$$goarch: ANDROID_NDK_LATEST_HOME not set"; continue; \
			fi; \
			cc="$$ANDROID_NDK_LATEST_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/$$cc"; \
		fi; \
		echo "building $$goos/$$goarch"; \
		GOOS=$$goos GOARCH=$$goarch CGO_ENABLED=$$cgo CC=$$cc \
			go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o "$(OUTPUT)/$$bin" $(PKG) || exit 1; \
		( cd $(OUTPUT) && \
		  zip -q "$(BINARY)-$$goos-$$goarch.zip" "$$bin" && \
		  rm -f "$$bin" && \
		  shasum -a 256 "$(BINARY)-$$goos-$$goarch.zip" | awk '{printf $$1}' \
		    > "$(BINARY)-$$goos-$$goarch.zip.sha256" ); \
	done
	@echo "release artifacts in $(OUTPUT)/ version=$(VERSION)"

## clean: remove build artifacts
clean:
	rm -rf $(OUTPUT)
	rm -f $(BINARY)

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
