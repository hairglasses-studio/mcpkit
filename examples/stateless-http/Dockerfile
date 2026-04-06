# Multi-stage build for the stateless HTTP MCP server example.
# Build context should be the mcpkit repo root (../../ from this file)
# so that all Go module dependencies are available.

FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -tags '!official_sdk' -o /server ./examples/stateless-http/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget
COPY --from=build /server /server
ENTRYPOINT ["/server"]
