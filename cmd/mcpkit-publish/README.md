# mcpkit-publish

CI helper for validating and publishing mcpkit server cards to an MCP Registry API.

```bash
go run ./cmd/mcpkit-publish \
  -card .well-known/mcp.json \
  -registry-url https://registry.modelcontextprotocol.io \
  -token "$MCP_REGISTRY_TOKEN"
```

Useful modes:

- `-validate-only`: validate metadata and print the workflow result without calling the registry.
- `-mode update -server-id <id>`: update an existing registry entry.
- `-token-url -client-id -client-secret`: fetch an OAuth2 client-credentials token instead of passing a static bearer token.
- `-json=false`: print a compact text summary instead of the JSON workflow result.
