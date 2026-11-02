SHELL := /bin/bash

.PHONY: fmt test vet docs-install docs-check docs-build check

fmt:
	gofmt -w $$(find api pkg -name '*.go')

test:
	go test ./...

vet:
	go vet ./...

docs-install:
	cd docs && bun install --frozen-lockfile

docs-check:
	cd docs && bun run check

docs-build:
	cd docs && bun run build

check: test vet docs-check docs-build
