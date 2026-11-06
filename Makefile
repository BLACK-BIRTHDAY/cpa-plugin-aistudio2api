GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
SPIDERMONKEY_VERSION ?= v152.0.4-aistudio2api.1
SPIDERMONKEY_DIR := $(CURDIR)/spidermonkey
LIBWEBP_VERSION ?= v1.6.0-aistudio2api.1
LIBWEBP_DIR := $(CURDIR)/libwebp
ifeq ($(GOOS),darwin)
	EXT := .dylib
else ifeq ($(GOOS),windows)
	EXT := .dll
else
	EXT := .so
endif

OUT_DIR := dist/plugins/$(GOOS)/$(GOARCH)
OUT_FILE := $(OUT_DIR)/aistudio2api$(EXT)

.PHONY: all spidermonkey build test install clean

all: build

spidermonkey:
	@if [ ! -d "$(SPIDERMONKEY_DIR)/lib" ]; then \
		mkdir -p "$(SPIDERMONKEY_DIR)"; \
		curl -fsSL "https://github.com/Mag1cFall/firefox/releases/download/$(SPIDERMONKEY_VERSION)/spidermonkey-$(SPIDERMONKEY_VERSION)-$(GOOS)-$(GOARCH).tar.gz" | tar -xz -C "$(SPIDERMONKEY_DIR)"; \
	fi
	@if [ ! -d "$(LIBWEBP_DIR)/lib" ]; then \
		mkdir -p "$(LIBWEBP_DIR)"; \
		curl -fsSL "https://github.com/Mag1cFall/libwebp/releases/download/$(LIBWEBP_VERSION)/libwebp-$(LIBWEBP_VERSION)-$(GOOS)-$(GOARCH).tar.gz" | tar -xz -C "$(LIBWEBP_DIR)"; \
	fi

build: spidermonkey
	mkdir -p $(OUT_DIR)
	CGO_ENABLED=1 \
	CGO_CPPFLAGS="-I$(SPIDERMONKEY_DIR)/include -I$(LIBWEBP_DIR)/include" \
	CGO_LDFLAGS="-L$(SPIDERMONKEY_DIR)/lib -L$(LIBWEBP_DIR)/lib" \
	go build -trimpath -ldflags="-s -w" -buildmode=c-shared -o $(OUT_FILE) ./cmd/aistudio2api-plugin

test: spidermonkey
	export CGO_ENABLED=1 \
		CGO_CPPFLAGS="-I$(SPIDERMONKEY_DIR)/include -I$(LIBWEBP_DIR)/include" \
		CGO_LDFLAGS="-L$(SPIDERMONKEY_DIR)/lib -L$(LIBWEBP_DIR)/lib" && \
	go vet ./... && \
	go test ./internal/aistudio ./internal/api ./internal/config ./internal/cpa ./internal/imagecodec/... ./internal/proxydial ./internal/resample

install: build
	cp -f $(OUT_FILE) "/Users/jianli/Library/Application Support/com.cpa.gui/cpa-core/plugins/darwin/arm64/aistudio2api.dylib"
clean:
	rm -rf dist
