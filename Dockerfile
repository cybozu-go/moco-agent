# stage1: build the binary
FROM quay.io/cybozu/golang:1.16-focal as builder

COPY ./ .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -a -o moco-agent ./cmd/moco-agent

# stage2: build the final image
FROM scratch
LABEL org.opencontainers.image.source https://github.com/cybozu-go/moco-agent

COPY --from=builder /work/moco-agent /
USER 10000:10000

ENTRYPOINT ["/moco-agent"]
