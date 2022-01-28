MYSQL_VERSION = 8.0.28

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

PROTOC := PATH=$(PWD)/bin:$(PATH) $(PWD)/bin/protoc -I=$(PWD)/include:.
PROTOC_BIN := $(PWD)/bin/protoc
PROTOC_GEN_GO := $(PWD)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(PWD)/bin/protoc-gen-go-grpc
PROTOC_GEN_DOC := $(PWD)/bin/protoc-gen-doc
PROTOC_VERSION := 3.19.1
PROTOC_GEN_GO_VERSION := $(shell awk '/google.golang.org\/protobuf/ {print substr($$2, 2)}' go.mod)
PROTOC_GEN_GO_GRPC_VERSON=1.1.0
PROTOC_GEN_DOC_VERSION=1.5.0

.PHONY: all
all: build/moco-agent

.PHONY: validate
validate: setup
	test -z "$$(gofmt -s -l . | tee /dev/stderr)"
	staticcheck ./...
	nilerr ./...
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
proto: proto/agentrpc.pb.go proto/agentrpc_grpc.pb.go docs/agentrpc.md

proto/agentrpc.pb.go: proto/agentrpc.proto $(PROTOC_BIN) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)
	$(PROTOC) --go_out=module=github.com/cybozu-go/moco-agent:. $<

proto/agentrpc_grpc.pb.go: proto/agentrpc.proto $(PROTOC_BIN) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)
	$(PROTOC) --go-grpc_out=module=github.com/cybozu-go/moco-agent:. $<

docs/agentrpc.md: proto/agentrpc.proto $(PROTOC_BIN) $(PROTOC_GEN_DOC)
	$(PROTOC) --doc_out=docs --doc_opt=markdown,$@ $<

$(PROTOC_BIN):
	mkdir -p bin
	curl -sfL -o protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-linux-x86_64.zip
	unzip -o protoc.zip bin/protoc 'include/*'
	rm -f protoc.zip

$(PROTOC_GEN_GO):
	mkdir -p bin
	GOBIN=$(PWD)/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@v$(PROTOC_GEN_GO_VERSION)

$(PROTOC_GEN_GO_GRPC):
	mkdir -p bin
	GOBIN=$(PWD)/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$(PROTOC_GEN_GO_GRPC_VERSON)

$(PROTOC_GEN_DOC):
	mkdir -p bin
	GOBIN=$(PWD)/bin go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@v$(PROTOC_GEN_DOC_VERSION)

.PHONY: setup
setup: custom-checker staticcheck nilerr $(PROTOC_BIN) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) $(PROTOC_GEN_DOC)

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
