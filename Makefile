# ShadowLink build.
#
# Build tags are REQUIRED, not optional:
#   with_utls      -> REALITY needs uTLS; without it the core refuses to start
#                     ("uTLS, which is required by reality is not included in this build")
#   with_gvisor    -> TUN gVisor/mixed network stack
#   with_clash_api -> runtime control (health probes, server switching)
TAGS := with_utls with_gvisor with_clash_api

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
LDFLAGS := -X github.com/shadowlink/shadowlink/internal/version.Version=$(VERSION)

.PHONY: build test vet tidy clean

build:
	go build -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o bin/shadowlink ./cmd/shadowlink

test:
	go test -tags "$(TAGS)" ./...

vet:
	go vet -tags "$(TAGS)" ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
