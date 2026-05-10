# Docker Compose MCP Server

Container-ready StreamableHTTP MCP server with two app instances behind nginx. This example is intentionally smaller than `examples/stateless-http`: it focuses on Docker image structure, compose health checks, lifecycle-driven readiness, and simple smoke tests.

## Quick Start

```bash
cd examples/docker
docker compose up --build
```

The stack exposes nginx on `http://localhost:8080` and routes to two backend MCP servers.

## Services

| Service | Purpose |
|---|---|
| `mcp-primary` | First MCP HTTP server, built from this repo |
| `mcp-secondary` | Second MCP HTTP server, built from this repo |
| `nginx` | Round-robin reverse proxy for `/mcp`, health endpoints, identity, and server card |

## Smoke Tests

```bash
curl -s http://localhost:8080/health | jq .
curl -s http://localhost:8080/ready | jq .
curl -s http://localhost:8080/live | jq .
curl -s http://localhost:8080/identity | jq .
curl -s http://localhost:8080/.well-known/mcp.json | jq .
```

Call the MCP endpoint through nginx:

```bash
curl -s http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq .
```

Run the identity check repeatedly to see nginx alternate between `mcp-primary` and `mcp-secondary`:

```bash
for i in 1 2 3 4; do curl -s http://localhost:8080/identity | jq -r .server_id; done
```

## Lifecycle Behavior

Each app container uses `lifecycle.Manager`:

- startup sets health status to `healthy`
- SIGTERM switches readiness to `draining`
- the HTTP server shuts down through `http.Server.Shutdown`
- `/ready` returns 503 while the process is draining

This makes the same binary usable in Docker Compose, Kubernetes, Nomad, or systemd-managed container deployments.

## File Inventory

| File | Purpose |
|---|---|
| `main.go` | StreamableHTTP MCP server with health, readiness, lifecycle, identity, and server card endpoints |
| `Dockerfile` | Multi-stage static Go build with non-root runtime user |
| `docker-compose.yml` | Two MCP app containers plus nginx and health checks |
| `nginx.conf` | Reverse proxy config with SSE-friendly buffering and timeouts |
| `README.md` | This guide |
