.PHONY: help dev dev-down test test-race build lint clean install icons \
        build-% test-% lint-%

# ---------------------------------------------------------------------------
# Binaries
# ---------------------------------------------------------------------------
INTEROP_BIN := ./bin/tpt-health-interop

# Modules that have a cmd/<binary> entrypoint (verified by listing cmd/).
MODULES_WITH_CMD := \
	doctor \
	pharmacy \
	pathology \
	acupuncture \
	chiropractic \
	osteopathy \
	massage \
	counselling \
	naturopathy \
	tcm \
	nutrition \
	vision \
	allied-health \
	hospital \
	dental \
	blood-bank

MODULE_BINS := $(foreach m,$(MODULES_WITH_CMD),./bin/tpt-$(shell echo $(m) | tr '_' '-'))

# All Go modules under modules/ (kept in sync with go.work). golangci-lint
# cannot resolve the ./modules/... glob directly because "modules" is not
# itself a Go module boundary under go.work — each subdirectory must be
# listed explicitly.
ALL_MODULES := \
	tpt-acupuncture tpt-addiction tpt-aged-care tpt-allied-health \
	tpt-blood-bank tpt-cardiology tpt-chiropractic tpt-clinical-trials \
	tpt-community-health tpt-counselling tpt-dental tpt-disability \
	tpt-doctor tpt-epidemiology tpt-health-billing tpt-hospital \
	tpt-immunisation tpt-massage tpt-maternal-child-health tpt-mental-health \
	tpt-naturopathy tpt-nutrition tpt-oncology tpt-osteopathy tpt-palliative \
	tpt-pathology tpt-pharmacy tpt-practice tpt-radiology tpt-rehabilitation \
	tpt-renal tpt-screening tpt-tcm tpt-telehealth tpt-vision

help:
	@echo "tpt-healthcare"
	@echo ""
	@echo "  make dev                Start full dev stack (PostgreSQL + Redis + interop)"
	@echo "  make dev-down           Stop the dev stack"
	@echo "  make test               Run all Go tests (core + interop + modules)"
	@echo "  make test-race          Run all Go tests with race detector"
	@echo "  make lint               Run golangci-lint across all workspace modules"
	@echo "  make build              Build all Go binaries (interop + modules)"
	@echo "  make build-<module>     Build a single module binary (e.g. make build-vision)"
	@echo "  make test-<module>      Test a single module (e.g. make test-core)"
	@echo "  make clean              Remove build artifacts"
	@echo "  make install            Install interop binary to /usr/local/bin"
	@echo "  make icons              Generate PWA icons for all frontend apps"

# ---------------------------------------------------------------------------
# Development stack
# ---------------------------------------------------------------------------
dev:
	docker compose -f deploy/docker-compose.dev.yml up -d
	@echo "Dev stack running. Interop: http://localhost:8080"

dev-down:
	docker compose -f deploy/docker-compose.dev.yml down

# ---------------------------------------------------------------------------
# Testing
# ---------------------------------------------------------------------------
test:
	go test ./core/... ./interop/... ./modules/...

test-race:
	go test -race ./core/... ./interop/... ./modules/...

# Per-module test target (accepts any package path).
test-%:
	go test ./$(subst -,,$*)/...

# ---------------------------------------------------------------------------
# Linting
# ---------------------------------------------------------------------------
lint:
	golangci-lint run --timeout=10m ./core/... ./interop/... $(foreach m,$(ALL_MODULES),./modules/$(m)/...)

lint-%:
	golangci-lint run ./$(subst -,,$*)/...

# ---------------------------------------------------------------------------
# Building
# ---------------------------------------------------------------------------
build: build-interop build-modules

build-interop:
	@mkdir -p bin
	go build -o $(INTEROP_BIN) ./interop/cmd/tpt-health-interop/

build-modules:
	@mkdir -p bin
	@for module in $(MODULES_WITH_CMD); do \
		binname="tpt-$$(echo $$module | tr '_' '-')"; \
		go build -o ./bin/$$binname ./modules/tpt-$$module/cmd/tpt-$$module/; \
	done

# Build a single module by name, e.g. make build-vision.
build-%:
	@mkdir -p bin
	@module=$(subst build-,,$@); \
	binname="tpt-$$(echo $$module | tr '_' '-')"; \
	go build -o ./bin/$$binname ./modules/tpt-$$module/cmd/tpt-$$module/

# ---------------------------------------------------------------------------
# Housekeeping
# ---------------------------------------------------------------------------
clean:
	rm -rf bin/

install: build-interop
	cp $(INTEROP_BIN) /usr/local/bin/

icons:
	cd tools/gen-icons && npm install --silent && node gen-icons.mjs
