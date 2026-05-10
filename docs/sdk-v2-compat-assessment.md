# go-sdk v2.0 Compatibility Assessment

Date: 2026-05-10

Package: `github.com/modelcontextprotocol/go-sdk`

Current mcpkit pin: `v1.6.0`

## Finding

No official `v2.0.0` module or release is available yet. A live module query on
2026-05-10 returned versions through `v1.6.0`, and the upstream GitHub releases
page lists `v1.6.0` as latest.

The `v1.6.0` release notes mention work toward a future `2026-06-30` release,
but not a v2 module path or v2 compatibility break.

## Impact

No mcpkit code migration is required for P34-6 today. The existing
`official_sdk` compatibility lane remains pinned to `v1.6.0`, and current
supported official-SDK package gates should stay scoped through the Makefile's
`OFFICIAL_SDK_*_PACKAGES` lists.

## v1.6.0 Behavior Changes Reviewed

- `CallToolResult.SetError` preserves pre-populated content unless
  `seterroroverwrite=1` is set. mcpkit does not call the upstream `SetError`
  API directly.
- `StreamableHTTPOptions.CrossOriginProtection` no longer enables origin
  verification by default when nil. mcpkit does not set the upstream option
  directly in its compatibility layer.
- `jsonescaping` was removed upstream. mcpkit does not reference it.

## Recheck Trigger

Reopen compatibility work when either condition is true:

- `go list -m -versions github.com/modelcontextprotocol/go-sdk` returns a
  `v2.*` tag.
- Upstream publishes an official v2 migration guide or v2 module path.

Useful verification commands:

```sh
go list -m -versions github.com/modelcontextprotocol/go-sdk
go list -m -json github.com/modelcontextprotocol/go-sdk
go test -tags official_sdk ./registry ./handler ./mcptest ./transport ./session ./gateway ./health ./sampling ./feedback -count=1
```
