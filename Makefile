GO ?= go
GOLANGCI_LINT ?= $(shell $(GO) env GOPATH)/bin/golangci-lint
MIN_COVERAGE ?= 80.0
XRAY_LOCATION_ASSET ?= var/vpn-product-predeploy3/assets
CMD_PKGS :=
ifneq ($(wildcard cmd/vpn-productd),)
CMD_PKGS += ./cmd/vpn-productd
endif
ifneq ($(wildcard cmd/vpn-productctl),)
CMD_PKGS += ./cmd/vpn-productctl
endif
PKGS ?= ./internal/... $(CMD_PKGS)
COVERPKGS ?= ./internal/configgen ./internal/connection
LINT_TARGETS ?= ./internal/... $(CMD_PKGS)
ALL_GO_PKGS ?= ./...

.PHONY: test test-all bench build lint cover verify verify-quick secret-scan ci

build:
	$(GO) build -trimpath -ldflags="-s -w" -o vpn-productd ./cmd/vpn-productd
	$(GO) build -trimpath -ldflags="-s -w" -o vpn-productctl ./cmd/vpn-productctl

test:
	$(GO) test $(PKGS)

test-all:
	XRAY_LOCATION_ASSET="$(XRAY_LOCATION_ASSET)" bash scripts/prepare_test_assets.sh
	$(GO) test $(ALL_GO_PKGS)

bench:
	$(GO) test ./internal/configgen ./internal/storage/sqlite -bench=. -benchmem -run=^$

lint:
	$(GOLANGCI_LINT) run $(LINT_TARGETS)

cover:
	$(GO) test $(COVERPKGS) -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

secret-scan:
	python3 scripts/secret_scan.py

verify-quick:
	$(GO) test $(PKGS)
	$(GOLANGCI_LINT) run $(LINT_TARGETS)
	python3 scripts/secret_scan.py

verify:
	python3 scripts/secret_scan.py
	$(GO) test $(PKGS)
	XRAY_LOCATION_ASSET="$(XRAY_LOCATION_ASSET)" bash scripts/prepare_test_assets.sh
	$(GO) test $(ALL_GO_PKGS)
	$(GOLANGCI_LINT) run $(LINT_TARGETS)
	$(GO) test $(COVERPKGS) -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | awk -v min="$(MIN_COVERAGE)" '/^total:/ {gsub("%","",$$3); cov=$$3+0; if (cov < min) {printf("coverage %.2f%% is below minimum %.2f%%\n", cov, min); exit 1} else {printf("coverage %.2f%% (min %.2f%%)\n", cov, min)}}'

ci: verify
