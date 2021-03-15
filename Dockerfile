# Build the moco-agent binary
FROM quay.io/cybozu/golang:1.16-focal as builder

ARG GRPC_HEALTH_PROBE_VERSION=0.3.6

WORKDIR /workspace

RUN curl -sSLf -o grpc-health-probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64
RUN chmod +x grpc-health-probe

# Copy the go source
COPY ./ .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o moco-agent ./cmd/moco-agent/main.go

FROM quay.io/cybozu/ubuntu:20.04
LABEL org.opencontainers.image.source https://github.com/cybozu-go/moco-agent

WORKDIR /
COPY --from=builder /workspace/grpc-health-probe /
COPY --from=builder /workspace/moco-agent /
USER 10000:10000

ENTRYPOINT ["/moco-agent"]
