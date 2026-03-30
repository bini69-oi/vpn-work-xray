GO ?= go
GOLANGCI_LINT ?= $(shell $(GO) env GOPATH)/bin/golangci-lint
PKGS ?= ./product/... ./cmd/vpn-productd ./cmd/vpn-productctl
COVERPKGS ?= ./product/configgen ./product/api ./product/storage/sqlite ./product/connection
LINT_TARGETS ?= ./product/... ./cmd/vpn-productd ./cmd/vpn-productctl

.PHONY: test bench lint cover

test:
	$(GO) test $(PKGS)

bench:
	$(GO) test ./product/configgen ./product/storage/sqlite -bench=. -benchmem -run=^$

lint:
	$(GOLANGCI_LINT) run $(LINT_TARGETS)

cover:
	$(GO) test $(COVERPKGS) -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
