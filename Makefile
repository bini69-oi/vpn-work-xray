GO ?= go
GOLANGCI_LINT ?= $(shell $(GO) env GOPATH)/bin/golangci-lint
MIN_COVERAGE ?= 55.0
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
ALL_GO_PKGS ?= ./...

.PHONY: test test-all bench lint cover verify ci

test:
	$(GO) test $(PKGS)

test-all:
	$(GO) test $(ALL_GO_PKGS)

bench:
	$(GO) test ./product/configgen ./product/storage/sqlite -bench=. -benchmem -run=^$

lint:
	$(GOLANGCI_LINT) run $(LINT_TARGETS)

cover:
	$(GO) test $(COVERPKGS) -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

verify:
	$(GO) test $(PKGS)
	$(GO) test $(ALL_GO_PKGS)
	$(GOLANGCI_LINT) run $(LINT_TARGETS)
	$(GO) test $(COVERPKGS) -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | awk -v min="$(MIN_COVERAGE)" '/^total:/ {gsub("%","",$$3); cov=$$3+0; if (cov < min) {printf("coverage %.2f%% is below minimum %.2f%%\n", cov, min); exit 1} else {printf("coverage %.2f%% (min %.2f%%)\n", cov, min)}}'

ci: verify
