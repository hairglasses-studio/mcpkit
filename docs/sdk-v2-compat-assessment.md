# go-sdk v2.0 Compatibility Assessment

> **Superseded 2026-08-07**: the recheck trigger below never fired because
> upstream shipped the breaking spec rewrite as **`v1.7.0`** (2026-07-27),
> not a `v2.*` tag or a v2 migration guide — the exact event this doc waited
> for, under a different version number. `go list -m -versions
> github.com/modelcontextprotocol/go-sdk` confirms `v1.7.0` is published and
> latest as of 2026-08-07. The dual-SDK migration lane (ROADMAP "After
> P34-6" gate) is reopened; see the P52.6 work landing this note for the
> live compatibility recheck against v1.7.x. This file's original finding
> below is left unedited as the point-in-time record of the 2026-05-10
> assessment.

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
