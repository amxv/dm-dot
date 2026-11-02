SHELL := /bin/bash

.PHONY: fmt test vet check

fmt:
	gofmt -w $$(find api pkg -name '*.go')

test:
	go test ./...

vet:
	go vet ./...

check: test vet
