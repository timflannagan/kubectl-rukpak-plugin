SHELL := /bin/bash

OUTPUT_BIN ?= bin/kubectl-bundle-evaluate

.PHONY: bin/evaluate
bin/evaluate:
	@go build -o $(OUTPUT_BIN) main.go

plugin: bin/evaluate
	@sudo cp $(OUTPUT_BIN) /usr/local/bin
	@kubectl bundle evaluate --help || (echo "failed to find the custom plugin in kubectl plugin path"; exit 1)
