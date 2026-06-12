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

GOARCH     ?= amd64
GOOS       ?= $(shell go env GOOS)

# Tooling is installed into BIN_DIR via `go run <tool>@<version>` so versions are
# pinned without network-piped install scripts.
LINTER       := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
TESTRUNNER   := go run gotest.tools/gotestsum@v1.13.0
VULNCHECKER  := go run golang.org/x/vuln/cmd/govulncheck@v1.3.0
COVER_FLOOR  := 80
FUZZTIME     ?= 10s

NO_COLOR   :=\033[0m
ATTN_COLOR :=\033[33;01m

## EOF define block

.PHONY: all
all: deps build test race lint cover vuln

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
	@zip -j $(T)/$(P)-$(GOARCH)/$(B)-$(P)-$(GOARCH).zip $(T)/$(P)-$(GOARCH)/$(B)$(if $(findstring $(P),windows),".exe","") >/dev/null

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
	@$(MAKE) checksums

# One SHA256SUMS over every zip, keyed by bare filename so a user who
# downloads a single asset can verify it with `shasum -a 256 -c SHA256SUMS
# --ignore-missing`. shasum (perl) rather than sha256sum: it exists on both
# the linux runners and macOS.
.PHONY: checksums
checksums:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@cd $(REL_DIR) && rm -f SHA256SUMS && for f in */*.zip; do \
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

.PHONY: clean
clean:
	@echo -e "$(ATTN_COLOR)==> $@ $(NO_COLOR)"
	@rm -rf $(BIN_DIR)
	@rm -rf $(REL_DIR)
	@rm -f coverage.*
	@go clean

$(REL_DIR):
	@echo -e "$(ATTN_COLOR)==> create REL_DIR $(REL_DIR) $(NO_COLOR)"
	@mkdir -p $(REL_DIR)

$(BIN_DIR):
	@echo -e "$(ATTN_COLOR)==> create BIN_DIR $(BIN_DIR) $(NO_COLOR)"
	@mkdir -p $(BIN_DIR)
