# Beekeeper Makefile
#
# SFDF-01 (Reproducible builds): the `verify-release` target performs two
# independent clean builds of the same source and asserts byte-for-byte
# identical sha256 hashes. A safety harness that cannot prove its own build
# integrity is not trustworthy, so this gate ships from v0.1.0.
#
# Reproducibility requirements (RESEARCH Open Question 2 / Pitfall 3):
#   - The SAME Go toolchain version MUST be used for both builds. The toolchain
#     is pinned via the `toolchain` directive in go.mod (go1.25.0). Building
#     with a different toolchain WILL produce a different hash even from
#     identical source — that is expected, not a reproducibility failure.
#   - ldflags inject ONLY the commit DATE (`git show -s --format=%cI HEAD`),
#     never a wall-clock shell timestamp, which would differ on every build and
#     break reproducibility.
#   - `-trimpath` strips local filesystem paths; `-buildvcs=false` suppresses
#     Go 1.18+ embedded VCS stamping; `-mod=readonly` forbids implicit go.mod
#     mutation during the build.

# Module path + version variable locations (see internal/version/version.go).
MODULE     := github.com/mzansi-agentive/beekeeper
VERSION    ?= dev
COMMIT     := $(shell git rev-parse HEAD)
# Commit DATE (RFC-3339), NOT the wall-clock build time — required for
# reproducibility. Never replace this with a shell wall-clock timestamp.
DATE       := $(shell git show -s --format=%cI HEAD)

# Reproducible build flags (CLAUDE.md Build Constraints).
GOFLAGS    := -trimpath -buildvcs=false -mod=readonly
LDFLAGS    := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

.PHONY: build test vet verify-release

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/beekeeper ./cmd/beekeeper

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

# verify-release (SFDF-01): prove the release build is byte-for-byte
# reproducible. Builds the same source TWICE into separate output paths with
# identical flags, computes sha256 of each artifact, and exits non-zero if the
# hashes differ. Requires VERSION (e.g. `make verify-release VERSION=0.1.0`).
#
# sha256sum is used where available (Linux, macOS via coreutils, Git Bash on
# Windows). Portable fallback: `shasum -a 256` (BSD/macOS default) — uncomment
# the fallback line below if sha256sum is absent on your platform.
verify-release:
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "ERROR: verify-release requires an explicit VERSION (e.g. make verify-release VERSION=0.1.0)"; \
		exit 1; \
	fi
	@echo ">> Reproducibility check for VERSION=$(VERSION)"
	@echo ">> Go toolchain in use (must be identical for both builds):"
	@go version
	@rm -rf dist/verify-a dist/verify-b
	@mkdir -p dist/verify-a dist/verify-b
	go build $(GOFLAGS) -ldflags "-s -w \
		-X $(MODULE)/internal/version.Version=$(VERSION) \
		-X $(MODULE)/internal/version.Commit=$(COMMIT) \
		-X $(MODULE)/internal/version.Date=$(DATE)" \
		-o dist/verify-a/beekeeper ./cmd/beekeeper
	go build $(GOFLAGS) -ldflags "-s -w \
		-X $(MODULE)/internal/version.Version=$(VERSION) \
		-X $(MODULE)/internal/version.Commit=$(COMMIT) \
		-X $(MODULE)/internal/version.Date=$(DATE)" \
		-o dist/verify-b/beekeeper ./cmd/beekeeper
	@HASH_A=$$(sha256sum dist/verify-a/beekeeper | cut -d' ' -f1); \
	HASH_B=$$(sha256sum dist/verify-b/beekeeper | cut -d' ' -f1); \
	echo "build A: $$HASH_A"; \
	echo "build B: $$HASH_B"; \
	if [ "$$HASH_A" != "$$HASH_B" ]; then \
		echo "FAIL: builds are NOT byte-for-byte reproducible (hash mismatch)"; \
		exit 1; \
	fi; \
	echo "OK: builds are byte-for-byte reproducible (SFDF-01)"
