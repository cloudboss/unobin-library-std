PROJECT := unobin-library-std
DIR_ROOT := $(realpath $(CURDIR))
DOCGEN ?= go run github.com/cloudboss/cloudboss-docs/unobin/cmd/docgen@main

.DEFAULT_GOAL := help

.PHONY: help docs test test-all

help:
	@echo 'Targets:'
	@echo '  docs      Generate the reference manual.'
	@echo '  test      Run tests without building a consumer.'
	@echo '  test-all  Run all tests, including compiled consumers.'

docs:
	@$(DOCGEN) --root $(DIR_ROOT) --out docs/reference

test:
	@go test -short ./...

test-all:
	@go test ./...
