# stage1: build the binary
FROM --platform=$BUILDPLATFORM ghcr.io/cybozu/golang:1.23-jammy as builder

ARG TARGETARCH

RUN apt-get update && apt-get install -y mysql-server

COPY ./ .

# Generate timezone.sql file before building
RUN mysql_tzinfo_to_sql /usr/share/zoneinfo > cmd/moco-init/timezone.sql
RUN GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -ldflags="-w -s" -a -o moco-agent ./cmd/moco-agent
RUN GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -ldflags="-w -s" -a -o moco-init ./cmd/moco-init
RUN GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -ldflags="-w -s" -a -o cp ./cmd/cp

# stage2: build the final image
FROM --platform=$TARGETPLATFORM scratch
LABEL org.opencontainers.image.source https://github.com/cybozu-go/moco-agent

COPY --from=builder /work/moco-agent /
COPY --from=builder /work/moco-init /
COPY --from=builder /work/cp /bin/

USER 10000:10000

ENTRYPOINT ["/moco-agent"]
