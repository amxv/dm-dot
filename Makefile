SHELL := /bin/bash

.PHONY: fmt test vet build-cli installers-check docs-install docs-check docs-build check

fmt:
	gofmt -w $$(find api pkg cmd internal -name '*.go')

build-cli:
	go build -o bin/dm-dot ./cmd/dm-dot

installers-check:
	sh -n docs/public/install.sh
	sh scripts/test-installer.sh

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

check: test vet installers-check docs-check docs-build
