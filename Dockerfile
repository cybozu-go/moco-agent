# Build the moco-agent binary
FROM quay.io/cybozu/golang:1.15-focal as builder

WORKDIR /workspace

# Copy the go source
COPY ./ .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o moco-agent ./cmd/moco-agent/main.go

FROM quay.io/cybozu/ubuntu:20.04
LABEL org.opencontainers.image.source https://github.com/cybozu-go/moco-agent

WORKDIR /
COPY --from=builder /workspace/moco-agent /
COPY --from=builder /workspace/ping.sh /
USER 10000:10000

ENTRYPOINT ["/moco-agent"]
