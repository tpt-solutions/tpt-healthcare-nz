.PHONY: dev test build lint clean install help

INTEROP_BIN=./bin/tpt-health-interop
DOCTOR_BIN=./bin/tpt-doctor

help:
	@echo "tpt-healthcare"
	@echo ""
	@echo "  make dev       Start full dev stack (PostgreSQL + Redis + interop)"
	@echo "  make test      Run all Go tests"
	@echo "  make build     Build all Go binaries"
	@echo "  make lint      Run golangci-lint across all modules"
	@echo "  make clean     Remove build artifacts"
	@echo "  make install   Install binaries to /usr/local/bin"

dev:
	docker compose -f deploy/docker-compose.dev.yml up -d
	@echo "Dev stack running. Interop: http://localhost:8080"

dev-down:
	docker compose -f deploy/docker-compose.dev.yml down

test:
	go test ./core/... ./interop/...

test-race:
	go test -race ./core/... ./interop/...

build:
	mkdir -p bin
	go build -o $(INTEROP_BIN) ./interop/cmd/tpt-health-interop/

lint:
	golangci-lint run ./core/... ./interop/...

clean:
	rm -rf bin/

migrate:
	$(INTEROP_BIN) migrate

install: build
	cp $(INTEROP_BIN) /usr/local/bin/
