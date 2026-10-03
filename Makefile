SHELL      := $(shell which bash)

## BOF define block

BINARIES   := glaze
BINARY     = $(word 1, $@)

# tmux has no native Windows build, so neither does glaze.
PLATFORMS  := linux darwin
PLATFORM   = $(word 1, $@)
GOARCHES   := amd64 arm64

ROOT_DIR   := $(shell git rev-parse --show-toplevel)
BIN_DIR    := $(ROOT_DIR)/bin
REL_DIR    := $(ROOT_DIR)/release
SRC_DIR    := $(ROOT_DIR)/cmd

VERSION    := $(shell git describe --tags 2>/dev/null || echo dev)
# Linux packages take the version without the leading v.
PKG_VERSION = $(patsubst v%,%,$(VERSION))
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE       := $(shell date "+%FT%T%z")
STAGE      ?= development

# The main package's linker symbols are addressed as `main.X`, not by the full
# import path (a Go quirk for package main), so -X targets use the `main` prefix.
LDBASE     := main
LDFLAGS    := -ldflags "-w -s \
	-X $(LDBASE).Version=$(VERSION) \
	-X $(LDBASE).Commit=$(COMMIT) \
	-X $(LDBASE).Date=$(DATE) \
	-X $(LDBASE).Stage=$(STAGE)"

GOARCH     ?= $(shell go env GOARCH)
GOOS       ?= $(shell go env GOOS)

# Tooling is installed into BIN_DIR via `go run <tool>@<version>` so versions are
# pinned without network-piped install scripts.
LINTER       := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
TESTRUNNER   := go run gotest.tools/gotestsum@v1.13.0
VULNCHECKER  := go run golang.org/x/vuln/cmd/govulncheck@v1.3.0
PACKAGER     := go run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0
COVER_FLOOR  := 80
FUZZTIME     ?= 10s

# The end-to-end module (e2e/) is a separate Go module. RUN is a go test -run
# pattern, TARGETS a comma-separated list of targets and BASE a git revision to
# compare with.
E2E_DIR      := $(ROOT_DIR)/e2e
E2E_GLAZE    := $(BIN_DIR)/$(GOOS)-$(GOARCH)/glaze

NO_COLOR   :=\033[0m
ATTN_COLOR :=\033[33;01m

## EOF define block

.PHONY: all
all: deps build test race lint cover vuln e2e-check

.PHONY: deps
deps:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@go mod download

.PHONY: tidy
tidy:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@go mod tidy

.PHONY: fmt
fmt:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@gofmt -w $(ROOT_DIR)

.PHONY: dobuild
dobuild:
	@echo -e "$(ATTN_COLOR)==> $@ $(B) GOOS=$(P) GOARCH=$(GOARCH) VERSION=$(VERSION) COMMIT=$(COMMIT) DATE=$(DATE) $(NO_COLOR)"
	@GOOS=$(P) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(T)/$(P)-$(GOARCH)/$(B)$(if $(findstring $(P),windows),".exe","") $(SRC_DIR)/$(B)
ifneq ($(P),windows)
	@chmod +x $(T)/$(P)-$(GOARCH)/$(B)
endif

.PHONY: build
build: $(BIN_DIR) deps
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@for b in ${BINARIES}; \
	do \
		$(MAKE) dobuild B=$${b} P=${GOOS} T=${BIN_DIR}; \
	done

.PHONY: doinstall
doinstall:
	@echo -e "$(ATTN_COLOR)==> $@ $(B) GOOS=$(P) GOARCH=$(GOARCH) VERSION=$(VERSION) COMMIT=$(COMMIT) DATE=$(DATE) $(NO_COLOR)"
	@GOOS=$(P) GOARCH=$(GOARCH) go install $(LDFLAGS) $(SRC_DIR)/$(B)

.PHONY: install
install:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@for b in ${BINARIES}; \
	do \
		$(MAKE) doinstall B=$${b} P=${GOOS}; \
	done

.PHONY: dorelease
dorelease:
	@echo -e "$(ATTN_COLOR)==> $@ build GOOS=$(P) GOARCH=$(GOARCH) VERSION=$(VERSION) COMMIT=$(COMMIT) DATE=$(DATE) $(NO_COLOR)"
	@GOOS=$(P) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(T)/$(P)-$(GOARCH)/$(B)$(if $(findstring $(P),windows),".exe","") $(SRC_DIR)/$(B)
ifneq ($(P),windows)
	@chmod +x $(T)/$(P)-$(GOARCH)/$(B)
endif
	@echo -e "$(ATTN_COLOR)==> $@ zip $(B)-$(P)-$(GOARCH).zip $(NO_COLOR)"
	@zip -j $(T)/$(P)-$(GOARCH)/$(B)-$(P)-$(GOARCH).zip $(T)/$(P)-$(GOARCH)/$(B)$(if $(findstring $(P),windows),".exe","") $(ROOT_DIR)/LICENSE.md >/dev/null

.PHONY: release
release: $(REL_DIR)
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@for b in ${BINARIES}; \
	do \
		for p in ${PLATFORMS}; \
		do \
			for a in ${GOARCHES}; \
			do \
				$(MAKE) dorelease B=$${b} P=$${p} GOARCH=$${a} T=${REL_DIR}; \
			done; \
		done; \
	done
	@$(MAKE) packages
	@$(MAKE) checksums

# A .deb, an .rpm and an unsigned .apk for each Linux architecture, from the binaries that `release` built.
# PKG_ARCH, not GOARCH: `go run` would build nfpm itself for the target architecture, which the host cannot run.
.PHONY: packages
packages:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@for a in ${GOARCHES}; \
	do \
		for k in deb rpm apk; \
		do \
			PKG_ARCH=$${a} PKG_VERSION=$(PKG_VERSION) $(PACKAGER) package --config $(ROOT_DIR)/packaging/nfpm.yaml \
				--packager $${k} --target $(REL_DIR)/linux-$${a}/glazier-linux-$${a}.$${k} || exit 1; \
		done; \
	done

# One SHA256SUMS over every zip and package, keyed by bare filename so a user who
# downloads a single asset can verify it with `shasum -a 256 -c SHA256SUMS
# --ignore-missing`. shasum (perl) rather than sha256sum: it exists on both
# the linux runners and macOS.
.PHONY: checksums
checksums:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@cd $(REL_DIR) && rm -f SHA256SUMS && for f in */*.zip */*.deb */*.rpm */*.apk; do \
		(cd $$(dirname $$f) && shasum -a 256 $$(basename $$f)); \
	done > SHA256SUMS
	@cat $(REL_DIR)/SHA256SUMS

.PHONY: test
test:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@CGO_ENABLED=0 $(TESTRUNNER) --format short-verbose -- -count=1 ./...

.PHONY: race
race:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@CGO_ENABLED=1 go test -race -count=1 ./...

.PHONY: cover
cover:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@CGO_ENABLED=0 go test -count=1 -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | awk -v floor=$(COVER_FLOOR) \
		'/^total:/ { sub(/%/, "", $$3); printf "total coverage: %s%% (floor: %s%%)\n", $$3, floor; exit ($$3 + 0 < floor) ? 1 : 0 }'

.PHONY: vet
vet:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@go vet ./...

.PHONY: lint
lint:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@CGO_ENABLED=0 $(LINTER) run ./...

# The vulnerability database is fetched live on every run; the pin above only
# fixes the scanner binary itself.
.PHONY: vuln
vuln:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@$(VULNCHECKER) ./...

# Go only fuzzes one target per invocation, so discover every Fuzz* target
# and run them one at a time. Packages without fuzz targets are skipped. The
# seed corpora also run as plain tests under `make test`.
.PHONY: fuzz
fuzz:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@set -e; for pkg in $$(go list ./...); do \
		for target in $$(CGO_ENABLED=0 go test -list '^Fuzz' $$pkg | grep '^Fuzz' || true); do \
			CGO_ENABLED=0 go test -run='^$$' -fuzz="^$$target\$$" -fuzztime=$(FUZZTIME) $$pkg; \
		done; \
	done

# Runs every end-to-end case on the host, against the tmux in PATH.
.PHONY: e2e
e2e: build
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@cd $(E2E_DIR) && GLAZE_BIN=$(E2E_GLAZE) E2E_REQUIRE=1 go test -count=1 $(if $(RUN),-run '$(RUN)') ./cases/

# Runs every end-to-end case on each tmux target in Docker, and compares with
# BASE when it is set.
.PHONY: e2e-matrix
e2e-matrix:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@cd $(E2E_DIR) && go run ./cmd/matrix $(if $(TARGETS),-targets '$(TARGETS)') $(if $(BASE),-base '$(BASE)') $(if $(RUN),-run '$(RUN)') $(MATRIX_FLAGS)

# Vets, lints and unit-tests the e2e module itself. It needs no tmux.
.PHONY: e2e-check
e2e-check:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@cd $(E2E_DIR) && go vet ./...
	@cd $(E2E_DIR) && CGO_ENABLED=0 $(LINTER) run --config $(ROOT_DIR)/.golangci.yml ./...
	@cd $(E2E_DIR) && CGO_ENABLED=0 go test -count=1 ./harness/ ./report/ ./cmd/...
	@cd $(E2E_DIR) && $(VULNCHECKER) ./...

# Formats every fixture in place, except the malformed ones.
.PHONY: e2e-fmt
e2e-fmt: build
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@find $(E2E_DIR)/fixtures -name '*.glaze' -not -path '*/malformed/*' -exec $(E2E_GLAZE) format --profile-path {} \;

.PHONY: clean
clean:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@rm -rf $(BIN_DIR)
	@rm -rf $(REL_DIR)
	@rm -f coverage.*
	@rm -rf $(E2E_DIR)/results
	@go clean

$(REL_DIR):
	@echo -e "$(ATTN_COLOR)==> create REL_DIR $(REL_DIR) $(NO_COLOR)"
	@mkdir -p $(REL_DIR)

$(BIN_DIR):
	@echo -e "$(ATTN_COLOR)==> create BIN_DIR $(BIN_DIR) $(NO_COLOR)"
	@mkdir -p $(BIN_DIR)
