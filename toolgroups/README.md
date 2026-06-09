# toolgroups

Package `toolgroups` provides a reusable **ToolGroupRegistry** pattern for
deferred, on-demand loading of MCP tool groups.

It is extracted from `ralphglasses/internal/mcpserver/registry.go` and
generalized so that `runmylife`, `hg-android`, `hg-pi`, and other MCP servers
can adopt the same registration model without importing ralphglasses internals.

## Core concepts

| Type | Role |
|---|---|
| `Builder` | Interface: `Name() string` + `Build(BuildContext) Group` |
| `FuncBuilder` | Convenience adapter — wraps a plain `func(BuildContext) Group` |
| `Group` | Output of a Builder: `Name`, `Description`, optional `LoadFn` |
| `BuildContext` | Server-supplied interface; carries config/logger into builders |
| `Registry` | Collects builders; produces groups on demand |

## Quickstart

```go
import "github.com/hairglasses-studio/mcpkit/toolgroups"

// 1. Create a registry.
reg := toolgroups.NewRegistry()

// 2. Register groups (last write wins on duplicate names).
reg.Register(toolgroups.NewFuncBuilder("audio", func(ctx toolgroups.BuildContext) toolgroups.Group {
    return toolgroups.Group{
        Name:        "audio",
        Description: "Audio device management",
        LoadFn: func() error {
            // Register tools into the live server here.
            return nil
        },
    }
}))

reg.Register(toolgroups.NewFuncBuilder("gpu", func(ctx toolgroups.BuildContext) toolgroups.Group {
    return toolgroups.Group{Name: "gpu", Description: "GPU status tools"}
}))

// 3. Build a single group on demand (e.g. when a client requests it).
g, err := reg.Build(myServerCtx, "audio")
if err != nil {
    // toolgroups.ErrUnknownGroup if not registered
}
if g.LoadFn != nil {
    if err := g.LoadFn(); err != nil { ... }
}

// 4. Or build all at startup.
groups := reg.BuildAll(myServerCtx)   // map[string]Group
ordered := reg.BuildAllOrdered(myServerCtx)  // []Group, registration order
```

## Implementing BuildContext

```go
type MyServer struct { cfg *Config; log *slog.Logger }

func (s *MyServer) ServerName() string { return s.cfg.Name }

// Pass *MyServer as the BuildContext — builders can type-assert if needed.
reg.Build(myServer, "audio")
```

## Adoption pattern (from ralphglasses)

In `ralphglasses/internal/mcpserver`, the registry is created once at server
init, builders are registered per-feature, and groups are loaded lazily via
`LoadToolGroup` when the client calls the corresponding meta-tool.

```
┌─────────────────────────────────────────────────┐
│  Server init                                    │
│   reg.Register(audio)                           │
│   reg.Register(gpu)                             │
│   reg.Register(network)                         │
└──────────────┬──────────────────────────────────┘
               │  client calls load_tool_group("audio")
               ▼
┌─────────────────────────────────────────────────┐
│  reg.Build(ctx, "audio")  →  Group.LoadFn()     │
│  → tools registered into live server            │
└─────────────────────────────────────────────────┘
```

## Verification

```bash
cd /home/hg/hairglasses-studio/mcpkit
GOWORK=off go build ./toolgroups
GOWORK=off go test ./toolgroups
```
