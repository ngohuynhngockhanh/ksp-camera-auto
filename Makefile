BINARY  := kspcam
PKG     := ./cmd/kspcam
DIST    := dist
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

GO ?= go
export CGO_ENABLED=0

.PHONY: all build run test tidy vet fmt clean build-all \
        build-amd64 build-arm32 build-arm64

all: build

build:
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BINARY) $(PKG)

run: build
	./$(BINARY) --config config.yaml

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

# Cross builds — all static (CGO disabled), single self-contained binary each.
build-all: build-amd64 build-arm32 build-arm64

build-amd64:
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-amd64 $(PKG)

build-arm32:
	GOOS=linux GOARCH=arm GOARM=7 $(GO) build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-armv7 $(PKG)

build-arm64:
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY)-linux-arm64 $(PKG)

# Optional Hikvision SDK (8000) build. Requires the HCNetSDK: set HIKSDK to the
# SDK dir containing incEn/ and lib/. Produces a cgo binary (not static, amd64).
# At runtime set KSPCAM_HIKSDK_PATH=$(HIKSDK)/lib. The SDK is never committed.
build-hiksdk:
	@test -n "$(HIKSDK)" || (echo "set HIKSDK=/path/to/HCNetSDK dir (with incEn/ and lib/)"; exit 1)
	CGO_ENABLED=1 \
	CGO_CPPFLAGS="-I$(HIKSDK)/incEn" \
	CGO_LDFLAGS="-L$(HIKSDK)/lib -lhcnetsdk -Wl,-rpath,$(HIKSDK)/lib" \
	$(GO) build -tags hiksdk -ldflags '$(LDFLAGS)' -o $(BINARY)-hiksdk $(PKG)

clean:
	rm -f $(BINARY) $(BINARY)-hiksdk
	rm -rf $(DIST)
