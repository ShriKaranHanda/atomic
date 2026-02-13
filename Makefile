SHELL := /bin/bash
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: build test test-unit test-integration test-vm vm-shell vm-bootstrap clean

build:
	GOCACHE=$(GOCACHE) go build ./cmd/atomic

test: test-unit test-integration

test-unit:
	GOCACHE=$(GOCACHE) go test ./internal/... ./cmd/atomic

test-integration:
	@if [ "$$(uname -s)" != "Linux" ]; then \
		echo "integration tests require Linux"; \
	elif [ "$$(id -u)" -ne 0 ]; then \
		sudo -E GOCACHE=$(GOCACHE) ATOMIC_INTEGRATION=1 go test ./integration -count=1 -v; \
	else \
		GOCACHE=$(GOCACHE) ATOMIC_INTEGRATION=1 go test ./integration -count=1 -v; \
	fi

test-vm:
	limactl shell atomic-ubuntu -- bash -lc 'cd /Users/karanhanda/atomic && make test'

vm-shell:
	limactl shell atomic-ubuntu

vm-bootstrap:
	limactl shell atomic-ubuntu -- bash /Users/karanhanda/atomic/scripts/vm/bootstrap-ubuntu.sh

clean:
	rm -rf .gocache
