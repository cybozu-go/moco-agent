include common.mk

# For Go
GO111MODULE = on
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

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
	MYSQL_VERSION=$(MYSQL_VERSION) go test -race -v -coverprofile cover.out ./...
	docker build -t mysql-with-go:latest ./initialize/ --build-arg MYSQL_VERSION=$(MYSQL_VERSION)
	docker run -v $(PWD):/go/src/github.com/cybozu-go/moco -e GOPATH=/tmp --rm mysql-with-go:latest sh -c "CGO_ENABLED=0 go test -v ./initialize"

# Build entrypoint binary
build/moco-agent:
	mkdir -p build
	GO111MODULE=on go build -o $@ ./cmd/moco-agent

.PHONY: mod
mod:
	go mod tidy
	git add go.mod

.PHONY: setup
setup: custom-checker staticcheck nilerr ineffassign

.PHONY: custom-checker
custom-checker:
	if ! which custom-checker >/dev/null; then \
		cd /tmp; env GOFLAGS= GO111MODULE=on go get github.com/cybozu/neco-containers/golang/analyzer/cmd/custom-checker; \
	fi

.PHONY: staticcheck
staticcheck:
	if ! which staticcheck >/dev/null; then \
		cd /tmp; env GOFLAGS= GO111MODULE=on go get honnef.co/go/tools/cmd/staticcheck; \
	fi

.PHONY: nilerr
nilerr:
	if ! which nilerr >/dev/null; then \
		cd /tmp; env GOFLAGS= GO111MODULE=on go get github.com/gostaticanalysis/nilerr/cmd/nilerr; \
	fi

.PHONY: clean
clean:
	rm -rf build
