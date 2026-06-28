PROJECT := unobin-library-std
DIR_ROOT := $(realpath $(CURDIR))
DOCGEN ?= go run github.com/cloudboss/cloudboss-docs/unobin/cmd/docgen@main

.DEFAULT_GOAL := help

.PHONY: help docs test

help:
	@echo 'Targets:'
	@echo '  docs    Generate the reference manual.'
	@echo '  test    Run unit tests on the host.'

docs:
	@$(DOCGEN) --root $(DIR_ROOT) --out docs/reference

test:
	@go test ./...
