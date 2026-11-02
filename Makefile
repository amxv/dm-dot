SHELL := /bin/bash

.PHONY: fmt test vet docs-check check

fmt:
	gofmt -w $$(find api pkg -name '*.go')

test:
	go test ./...

vet:
	go vet ./...

docs-check:
	npm run check

check: test vet docs-check
