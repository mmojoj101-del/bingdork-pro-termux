# Stage 1: Build
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 go build -ldflags "\
    -X github.com/bingdork/bingdork/cli.Version=${VERSION} \
    -X github.com/bingdork/bingdork/cli.Commit=${COMMIT} \
    -X github.com/bingdork/bingdork/cli.Date=${DATE} \
    -X github.com/bingdork/bingdork/cli.GoVersion=$(go version | awk '{print $3}')" \
    -o /bingdork ./cmd/bingdork/

# Stage 2: Runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata libcap

# Create non-root user
RUN adduser -D -g '' bingdork

# Copy binary
COPY --from=builder /bingdork /usr/local/bin/bingdork

# Create data directories
RUN mkdir -p /home/bingdork/.bingdork/data \
    /home/bingdork/.bingdork/cache \
    /home/bingdork/.bingdork/plugins

# Set capabilities
RUN setcap 'cap_net_bind_service=+ep' /usr/local/bin/bingdork

USER bingdork
WORKDIR /home/bingdork

ENTRYPOINT ["/usr/local/bin/bingdork"]
CMD ["--help"]

LABEL org.opencontainers.image.title="BingDork Pro" \
      org.opencontainers.image.description="Advanced Search Automation Framework" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.licenses="MIT"
