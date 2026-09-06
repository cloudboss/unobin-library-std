PROJECT := unobin-library-std
DIR_ROOT := $(realpath $(CURDIR))
DOCGEN ?= go run github.com/cloudboss/cloudboss-docs/unobin/cmd/docgen@v0.2.1

.DEFAULT_GOAL := help

.PHONY: help docs test test-all test-release

help:
	@echo 'Targets:'
	@echo '  docs          Generate the reference manual.'
	@echo '  test          Run tests without building a consumer.'
	@echo '  test-all      Run all tests, including compiled consumers.'
	@echo '  test-release  Check packaging with an empty module cache.'

docs:
	@$(DOCGEN) --root $(DIR_ROOT) --out docs/reference

test:
	@go test -short ./...

test-all:
	@go test ./...

test-release:
	@go test -tags release -run '^TestConsumerWithoutModuleReplacements$$' -count=1 .
