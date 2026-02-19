SHELL := /bin/bash
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: build build-daemon test test-unit test-integration test-vm vm-create vm-shell vm-bootstrap vm-delete clean

build:
	GOCACHE=$(GOCACHE) go build ./cmd/atomic

test: test-unit test-integration

test-unit:
	GOCACHE=$(GOCACHE) go test ./internal/... ./cmd/...

test-integration:
	@if [ "$$(uname -s)" != "Linux" ]; then \
		echo "integration tests require Linux"; \
	else \
		GOCACHE=$(GOCACHE) ./integration/e2e/run_all.sh; \
	fi

test-vm:
	limactl shell atomic-ubuntu -- bash -lc 'cd /Users/karanhanda/atomic && GOCACHE=/tmp/atomic-gocache make test'

vm-create:
	limactl start --name=atomic-ubuntu template://ubuntu

vm-shell:
	limactl shell atomic-ubuntu

vm-bootstrap:
	limactl shell atomic-ubuntu -- bash /Users/karanhanda/atomic/scripts/vm/bootstrap-ubuntu.sh

vm-delete:
	limactl delete --force atomic-ubuntu

clean:
	rm -rf .gocache
