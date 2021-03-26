include common.mk

# For Go
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GO_FILES := $(shell find . -name '*.go' -print)

PROTOC := $(PWD)/bin/protoc
PROTOC_VERSION := 3.14.0

.PHONY: all
all: build/moco-agent

.PHONY: validate
validate: setup
	test -z "$$(gofmt -s -l . | tee /dev/stderr)"
	staticcheck ./...
	test -z "$$(nilerr ./... 2>&1 | tee /dev/stderr)"
	test -z "$$(custom-checker -restrictpkg.packages=html/template,log $$(go list -tags='$(GOTAGS)' ./... ) 2>&1 | tee /dev/stderr)"
	go build ./...
	go vet ./...
	test -z "$$(go vet ./... | tee /dev/stderr)"

# Run tests
.PHONY: test
test:
	MYSQL_VERSION=$(MYSQL_VERSION) go test -race -v -timeout 30m -coverprofile cover.out ./...

# Build moco-agent binary
build/moco-agent: $(GO_FILES)
	mkdir -p build
	go build -o $@ ./cmd/moco-agent

.PHONY: agentrpc
agentrpc: $(PROTOC)
	$(PROTOC) --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative server/agentrpc/agentrpc.proto

$(PROTOC):
	mkdir -p bin
	curl -sfL -O https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-linux-x86_64.zip
	unzip -p protoc-$(PROTOC_VERSION)-linux-x86_64.zip bin/protoc > bin/protoc
	chmod +x bin/protoc
	rm protoc-$(PROTOC_VERSION)-linux-x86_64.zip

.PHONY: setup
setup: custom-checker staticcheck nilerr

.PHONY: custom-checker
custom-checker:
	if ! which custom-checker >/dev/null; then \
		env GOFLAGS= go install github.com/cybozu/neco-containers/golang/analyzer/cmd/custom-checker@latest; \
	fi

.PHONY: staticcheck
staticcheck:
	if ! which staticcheck >/dev/null; then \
		env GOFLAGS= go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi

.PHONY: nilerr
nilerr:
	if ! which nilerr >/dev/null; then \
		env GOFLAGS= go install github.com/gostaticanalysis/nilerr/cmd/nilerr@latest; \
	fi

.PHONY: clean
clean:
	rm -rf build
