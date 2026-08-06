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

.PHONY: lint
lint: aqua-install
	@golangci-lint run --timeout 5m

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

.PHONY: clean
clean:
	rm -rf build
