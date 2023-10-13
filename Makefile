SHELL := /bin/bash

.PHONY: \
	help \
	coverage \
	vet \
	lint \
	fmt \
	version

help: ## Show this help screen.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make  \033[36m<OPTIONS>\033[0m ... \033[36m<TARGETS>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


all: imports fmt lint vet errors build

print-%:
	@echo $* = $($*)

deps: 
	go get golang.org/x/lint/golint

coverage: ## Report code tests coverage.
	go test $(go list ./... | grep -v examples) -coverprofile coverage.txt ./...

vet: ## Run go vet.
	go vet ./...

lint: deps
	golint ./...

fmt: ## Run go fmt.
	go install mvdan.cc/gofumpt@latest
	gofumpt -l -w -extra .

pre-dev:
	make pre-commit

pre-commit:
	bash script/pre-commit-hook

release: package-release sign-release

version: ## Display Go version.
	@go version
