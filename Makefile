GO ?= go
GOLANGCI_LINT ?= $(shell $(GO) env GOPATH)/bin/golangci-lint
CMD_PKGS :=
ifneq ($(wildcard cmd/vpn-productd),)
CMD_PKGS += ./cmd/vpn-productd
endif
ifneq ($(wildcard cmd/vpn-productctl),)
CMD_PKGS += ./cmd/vpn-productctl
endif
PKGS ?= ./product/... $(CMD_PKGS)
COVERPKGS ?= ./product/configgen ./product/api ./product/storage/sqlite ./product/connection
LINT_TARGETS ?= ./product/... $(CMD_PKGS)

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
