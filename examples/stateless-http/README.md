# Stateless HTTP MCP Server

A horizontally scalable MCP server with Redis-backed sessions. This example
demonstrates how to run multiple MCP server instances behind a load balancer
while maintaining consistent session state.

## Architecture

```
                    ┌─────────┐
   Clients ────────>│  nginx  │ :80
                    └────┬────┘
                         │ round-robin
              ┌──────────┼──────────┐
              v                     v
        ┌───────────┐        ┌───────────┐
        │ mcp-srv-1 │        │ mcp-srv-2 │
        │   :8080   │        │   :8080   │
        └─────┬─────┘        └─────┬─────┘
              │                     │
              └──────────┬──────────┘
                         v
                    ┌─────────┐
                    │  Redis  │ :6379
                    └─────────┘
```

**Key insight**: Sessions are stored in Redis via `session.RedisStringStore`, not
in process memory. Any server instance can handle any request because session
state is always loaded from the shared Redis store. Nginx round-robins freely.

## What it demonstrates

| mcpkit component | Role in this example |
|---|---|
| `session.RedisStringStore` | Persists sessions as JSON in Redis via the `RedisClient` interface |
| `session.RedisClient` | Minimal Redis interface (Get/Set/Del) — bring your own client library |
| `transport.SessionExtractor` | Extracts session IDs from headers, cookies, or query params |
| `session.TokenMiddleware` | HTTP middleware that loads sessions from the store into context |
| `gateway.AffinityMiddleware` | (Available) Routes requests to consistent upstreams by session hash |

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- Or: Go 1.22+ and a local Redis for standalone mode

## Quick start with Docker Compose

```bash
cd examples/stateless-http
docker compose up --build
```

This starts:
- **Redis** on port 6379
- **mcp-server-1** and **mcp-server-2** (two identical MCP server instances)
- **nginx** on port 80 (round-robin load balancer)

## Testing

### 1. Create a session

```bash
curl -s http://localhost/session/new | jq .
```

Output:
```json
{
  "message": "session created",
  "server_id": "mcp-server-1",
  "session_id": "a1b2c3d4e5f6..."
}
```

Save the session ID:
```bash
SESSION_ID=$(curl -s http://localhost/session/new | jq -r .session_id)
```

### 2. Verify round-robin — same session, different servers

```bash
# Run several times — note server_id alternates between mcp-server-1 and mcp-server-2
curl -s -b "mcp_session=$SESSION_ID" http://localhost/session/info | jq .server_id
curl -s -b "mcp_session=$SESSION_ID" http://localhost/session/info | jq .server_id
curl -s -b "mcp_session=$SESSION_ID" http://localhost/session/info | jq .server_id
```

### 3. Increment counter across instances

```bash
# Each request may hit a different server, but the counter increments correctly
curl -s -b "mcp_session=$SESSION_ID" -X POST http://localhost/session/incr | jq .
curl -s -b "mcp_session=$SESSION_ID" -X POST http://localhost/session/incr | jq .
curl -s -b "mcp_session=$SESSION_ID" -X POST http://localhost/session/incr | jq .
```

Output (counter=3, served by different instances):
```json
{"counter": 3, "server_id": "mcp-server-2", "session_id": "a1b2c3..."}
```

### 4. Use session ID via header (for programmatic clients)

```bash
curl -s -H "X-Session-ID: $SESSION_ID" http://localhost/session/info | jq .
```

### 5. MCP protocol endpoint

The MCP StreamableHTTP endpoint is at `/mcp`. You can use any MCP client:

```bash
# Example: list tools via MCP protocol
curl -s -X POST http://localhost/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq .
```

## Standalone mode (no Docker)

```bash
# Terminal 1: start Redis
redis-server

# Terminal 2: start one server instance
cd /path/to/mcpkit
REDIS_URL=redis://localhost:6379 PORT=8080 SERVER_ID=local go run ./examples/stateless-http/

# Terminal 3: test
curl -s http://localhost:8080/session/new | jq .
```

## Adapting for production

### Real Redis client

The example includes a `MemRedisClient` for zero-dependency demonstration. In
production, implement `session.RedisClient` with your preferred Redis library:

```go
import "github.com/redis/go-redis/v9"

type GoRedisClient struct {
    rdb *redis.Client
}

func (c *GoRedisClient) Get(ctx context.Context, key string) (string, error) {
    val, err := c.rdb.Get(ctx, key).Result()
    if err == redis.Nil {
        return "", session.ErrNotFound
    }
    return val, err
}

func (c *GoRedisClient) Set(ctx context.Context, key, value string, ttl time.Duration) error {
    return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *GoRedisClient) Del(ctx context.Context, keys ...string) error {
    return c.rdb.Del(ctx, keys...).Err()
}
```

### Session affinity (optional)

If your tools have expensive warm-up or caching, you can use
`gateway.AffinityRouter` for consistent-hash routing that pins sessions to
specific upstreams while still allowing failover:

```go
router := gateway.NewAffinityRouter(gateway.AffinityConfig{
    Extractor: extractor,
    Store:     store,
    Upstreams: []string{"server-1:8080", "server-2:8080"},
})

// Route returns the same upstream for the same session ID
target := router.Route(sessionID)
```

### Scaling beyond two instances

Add more services to `docker-compose.yml` and update the nginx upstream block.
Because session state is in Redis, there is no coordination needed between
server instances.

## File inventory

| File | Purpose |
|---|---|
| `main.go` | Server with session store, tools, REST endpoints, MCP transport |
| `docker-compose.yml` | Redis + 2 MCP servers + nginx load balancer |
| `Dockerfile` | Multi-stage Go build |
| `nginx.conf` | Round-robin upstream with SSE support |
| `README.md` | This file |
