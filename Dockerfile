# Build stage
FROM docker.io/library/golang:1.22-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -X main.version=dev" -o kerrigan-node ./cmd/node
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -X main.version=dev" -o kerrigan-cli ./cmd/cli

# Runtime stage
FROM docker.io/library/alpine:3.19

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

RUN addgroup -g 1000 -S kerrigan && \
    adduser -u 1000 -S kerrigan -G kerrigan

COPY --from=builder /build/kerrigan-node /app/
COPY --from=builder /build/kerrigan-cli /app/

RUN chown -R kerrigan:kerrigan /app

USER kerrigan

EXPOSE 38888 38889

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:38888/health || exit 1

ENTRYPOINT ["/app/kerrigan-node"]
CMD ["run"]