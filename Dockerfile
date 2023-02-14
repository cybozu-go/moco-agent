# stage1: build the binary
FROM quay.io/cybozu/golang:1.19-jammy as builder

COPY ./ .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -a -o moco-agent ./cmd/moco-agent
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -a -o moco-init ./cmd/moco-init
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -a -o cp ./cmd/cp

# stage2: build the final image
FROM scratch
LABEL org.opencontainers.image.source https://github.com/cybozu-go/moco-agent

COPY --from=builder /work/moco-agent /
COPY --from=builder /work/moco-init /
COPY --from=builder /work/cp /bin/

USER 10000:10000

ENTRYPOINT ["/moco-agent"]
