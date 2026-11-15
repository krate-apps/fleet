APP        := fleet
MODULE     := github.com/krate-apps/fleet
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT_AT   ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.BuiltAt=$(BUILT_AT)

GO_PACKAGES := $(shell go list ./... | grep -v /node_modules/)

# Workspace extension checkouts (optional local helper).
EXT_ROOT ?= $(abspath ../extensions)

.PHONY: all build web embed test lint fmt tidy clean run ci ext-workers tools-json perf-cache e2e-smoke load-smoke seed-fixtures cleanup-fixtures rtprobe certify test-scripts pack

all: build

# Build workers in each extensions/fleet-ext-* checkout (bins stay in that repo).
ext-workers:
	@set -e; \
	for repo in "$(EXT_ROOT)"/fleet-ext-*; do \
	  [ -d "$$repo" ] || continue; \
	  slug=$${repo##*/fleet-ext-}; \
	  [ "$$slug" = "demo" ] && continue; \
	  if [ -f "$$repo/Makefile" ]; then \
	    echo "build $$slug (make build)"; \
	    $(MAKE) -C "$$repo" build; \
	    continue; \
	  fi; \
	  worker=""; \
	  for d in "$$repo"/cmd/*; do \
	    [ -d "$$d" ] || continue; \
	    base=$$(basename "$$d"); \
	    case "$$base" in *-worker) worker="$$base";; esac; \
	  done; \
	  if [ -z "$$worker" ]; then \
	    echo "skip $$slug (no cmd/*-worker)"; \
	    continue; \
	  fi; \
	  mkdir -p "$$repo/bin"; \
	  echo "build $$slug → bin/$$worker"; \
	  (cd "$$repo" && go build -trimpath -o "bin/$$worker" "./cmd/$$worker"); \
	done

# OPEN-11 reference JSON CLIs (Fleet-owned; not third-party classic tools).
tools-json:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/fleet-dumptorrent ./cmd/fleet-dumptorrent
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/fleet-mktorrent ./cmd/fleet-mktorrent

# Optional public UI base for path-mode reverse proxies, e.g. FLEET_UI_BASE=/trevor/fleet/
web:
	cd web && npm ci && FLEET_UI_BASE="$(FLEET_UI_BASE)" npm run build

embed: web
	rm -rf cmd/fleet/web
	mkdir -p cmd/fleet/web
	cp -a web/dist/. cmd/fleet/web/
	# Keep embed happy when dist is empty in CI stubs.
	touch cmd/fleet/web/.gitkeep

build: tools-json
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/fleet

test:
	go test $(GO_PACKAGES)
	cd web && npm ci && npm run typecheck
	$(MAKE) test-scripts

test-scripts:
	bash tests/scripts/install_uninstall_smoke.sh
	bash tests/scripts/pack_smoke.sh
	@if [ -d ../actions/fleet-bucket-action ]; then $(MAKE) -C ../actions/fleet-bucket-action test; fi

lint:
	go vet $(GO_PACKAGES)
	cd web && npm ci && npm run lint

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*' -not -path './web/node_modules/*' -not -path './extensions/*')

tidy:
	go mod tidy

clean:
	rm -rf bin web/dist cmd/fleet/web
	mkdir -p cmd/fleet/web
	touch cmd/fleet/web/.gitkeep

run: build
	./bin/fleet -config docs/user/examples/config.example.yaml

ci: tidy test lint build

perf-cache:
	go test ./internal/sync/ -bench=BenchmarkCache -benchmem -count=1

e2e-smoke:
	bash tests/e2e/smoke.sh

load-smoke:
	bash tests/load/http_smoke.sh

# Load fixtures (requires running Fleet + JSON mktorrent).
# Example: BASE_URL=http://127.0.0.1:8080 COUNT=1000 MKTORRENT=./bin/fleet-mktorrent make seed-fixtures
seed-fixtures:
	bash tests/load/seed_rtorrent_fixtures.sh

cleanup-fixtures:
	bash tests/load/cleanup_rtorrent_fixtures.sh

# Example: make rtprobe RTPROBE_ADDR=unix:///run/krate/user/$$USER.rtorrent.sock
RTPROBE_ADDR ?=
rtprobe:
	@if [ -z "$(RTPROBE_ADDR)" ]; then echo "Set RTPROBE_ADDR=unix:///path.sock"; exit 2; fi
	go run ./cmd/rtprobe -address "$(RTPROBE_ADDR)"

certify:
	go test $(GO_PACKAGES)
	go test ./internal/sync/ -bench=BenchmarkCacheApply1000 -benchmem -count=1
	go test ./tests/e2e/ -count=1
	@echo "certify: automated slice OK"

pack:
	bash scripts/pack.sh dist
