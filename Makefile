###########################
# Configuration Variables #
###########################
# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL := /usr/bin/env bash -o pipefail
.SHELLFLAGS := -ec
export ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

GOLANG_VERSION := $(shell sed -En 's/^go (.*)$$/\1/p' "go.mod")

# bingo manages consistent tooling versions for things like kind, kustomize, etc.
include .bingo/Variables.mk

# Disable -j flag for make
.NOTPARALLEL:

.DEFAULT_GOAL := build

#SECTION General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '#SECTION' and the
# target descriptions by '#HELP' or '#EXHELP'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: #HELP something, and then pretty-format the target and help. Then,
# if there's a line with #SECTION something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php
# The extended-help target uses '#EXHELP' as the delineator.

.PHONY: help
help: #HELP Display essential help.
	@awk 'BEGIN {FS = ":[^#]*#HELP"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\n"} /^[a-zA-Z_0-9-]+:.*#HELP / { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } ' $(MAKEFILE_LIST)

.PHONY: help-extended
help-extended: #HELP Display extended help.
	@awk 'BEGIN {FS = ":.*#(EX)?HELP"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*#(EX)?HELP / { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^#SECTION / { printf "\n\033[1m%s\033[0m\n", substr($$0, 10) } ' $(MAKEFILE_LIST)

#SECTION Development

.PHONY: lint
lint: $(GOLANGCI_LINT) #HELP Run golangci linter.
	$(GOLANGCI_LINT) run $(GOLANGCI_LINT_ARGS)

.PHONY: tidy
tidy: #HELP Update dependencies.
	# Force tidy to use the version already in go.mod
	$(Q)go mod tidy -go=$(GOLANG_VERSION)

.PHONY: verify
verify: tidy fmt vet #HELP Verify all generated code is up-to-date.
	git diff --exit-code

.PHONY: fix-lint
fix-lint: $(GOLANGCI_LINT) #EXHELP Fix lint issues
fix-lint: GOLANGCI_LINT_ARGS += --fix
fix-lint: lint

.PHONY: fmt
fmt: #EXHELP Formats code
	go fmt ./...

.PHONY: vet
vet: #EXHELP Run go vet against code.
	go vet -tags '$(GO_BUILD_TAGS)' ./...

.PHONY: bingo-upgrade
bingo-upgrade: $(BINGO) #EXHELP Upgrade tools
	@for pkg in $$($(BINGO) list | awk '{ print $$1 }' | tail -n +3); do \
		echo "Upgrading $$pkg to latest..."; \
		$(BINGO) get "$$pkg@latest"; \
	done

.PHONY: test
test: test-unit #HELP Run all tests.

.PHONY: test-unit
UNIT_TEST_DIRS := $(shell go list ./... | grep -v /test/)
COVERAGE_UNIT_DIR := $(ROOT_DIR)/coverage/unit
test-unit: #HELP Run the unit tests
	rm -rf $(COVERAGE_UNIT_DIR) && mkdir -p $(COVERAGE_UNIT_DIR)
	CGO_ENABLED=1 go test \
		-tags '$(GO_BUILD_TAGS)' \
		-cover -coverprofile ${ROOT_DIR}/coverage/unit.out \
		-count=1 -race -short \
		$(UNIT_TEST_DIRS) \
		-test.gocoverdir=$(ROOT_DIR)/coverage/unit

#SECTION Build

ifeq ($(origin VERSION), undefined)
VERSION := $(shell git describe --tags --always --dirty)
endif
export VERSION

ifeq ($(origin CGO_ENABLED), undefined)
CGO_ENABLED := 0
endif
export CGO_ENABLED

export GIT_REPO := $(shell go list -m)
export VERSION_PATH := ${GIT_REPO}/internal/version
export GO_BUILD_ASMFLAGS := all=-trimpath=$(PWD)
export GO_BUILD_GCFLAGS := all=-trimpath=$(PWD)
export GO_BUILD_FLAGS :=
export GO_BUILD_TAGS :=
export GO_BUILD_LDFLAGS := -s -w \
    -X '$(VERSION_PATH).version=$(VERSION)'

.PHONY: build
build: #HELP build h5d binary
	go build $(GO_BUILD_FLAGS) -tags '$(GO_BUILD_TAGS)' -ldflags '$(GO_BUILD_LDFLAGS)' -gcflags '$(GO_BUILD_GCFLAGS)' -asmflags '$(GO_BUILD_ASMFLAGS)' -o bin/h5d main.go

#SECTION Release
ifeq ($(origin ENABLE_RELEASE_PIPELINE), undefined)
ENABLE_RELEASE_PIPELINE := false
endif
ifeq ($(origin GORELEASER_ARGS), undefined)
GORELEASER_ARGS := --snapshot --clean
endif

export ENABLE_RELEASE_PIPELINE
export GORELEASER_ARGS

.PHONY: release
release: $(GORELEASER) #EXHELP Runs goreleaser for the operator-controller. By default, this will run only as a snapshot and will not publish any artifacts unless it is run with different arguments. To override the arguments, run with "GORELEASER_ARGS=...". When run as a github action from a tag, this target will publish a full release.
	$(GORELEASER) $(GORELEASER_ARGS)