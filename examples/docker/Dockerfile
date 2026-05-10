# Multi-stage build for the Docker Compose MCP server example.
# Build context must be the mcpkit repository root.

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -tags '!official_sdk' -trimpath -ldflags="-s -w" -o /out/docker-example ./examples/docker

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget \
    && adduser -D -H -u 10001 mcpkit
USER mcpkit
ENV PORT=8080
EXPOSE 8080
COPY --from=build /out/docker-example /usr/local/bin/docker-example
ENTRYPOINT ["/usr/local/bin/docker-example"]
