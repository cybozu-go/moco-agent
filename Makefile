MYSQL_VERSION = 8.4.4

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

PROTOC := protoc

.PHONY: all
all: build/moco-agent

.PHONY: aqua-install
aqua-install:
	aqua install

.PHONY: validate
validate: setup aqua-install
	test -z "$$(gofmt -s -l . | tee /dev/stderr)"
	staticcheck ./...
	test -z "$$(custom-checker -restrictpkg.packages=html/template,log $$(go list -tags='$(GOTAGS)' ./... ) 2>&1 | tee /dev/stderr)"
	go build ./...
	go vet ./...

.PHONY: check-generate
check-generate:
	$(MAKE) proto
	git diff --exit-code --name-only

# Run tests
.PHONY: test
test:
	MYSQL_VERSION=$(MYSQL_VERSION) go test -race -v -timeout 30m -coverprofile cover.out ./...

# Build moco-agent binary
build/moco-agent: $(GO_FILES)
	mkdir -p build
	go build -o $@ ./cmd/moco-agent

.PHONY: proto
proto: aqua-install proto/agentrpc.pb.go proto/agentrpc_grpc.pb.go docs/agentrpc.md

proto/agentrpc.pb.go: proto/agentrpc.proto
	$(PROTOC) --go_out=module=github.com/cybozu-go/moco-agent:. $<

proto/agentrpc_grpc.pb.go: proto/agentrpc.proto
	$(PROTOC) --go-grpc_out=module=github.com/cybozu-go/moco-agent:. $<

docs/agentrpc.md: proto/agentrpc.proto
	$(PROTOC) --doc_out=docs --doc_opt=markdown,$@ $<

.PHONY: setup
setup: custom-checker

.PHONY: custom-checker
custom-checker:
	if ! which custom-checker >/dev/null; then \
		env GOFLAGS= go install github.com/cybozu-go/golang-custom-analyzer/cmd/custom-checker@latest; \
	fi

.PHONY: clean
clean:
	rm -rf build
