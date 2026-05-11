# Fleet Script Conversion Graph

```mermaid
flowchart TD
  A[Discover scripts] --> B[Analyze target + complexity]
  B --> C[Plan conversion shape]
  C --> D1[cmd/hg-sync (Go)]
  C --> D2[cmd/hg-pipeline (Go)]
  C --> D3[cmd/cr8 (Go)]
  C --> D4[tools/launchers-rs (Rust)]
  D1 --> E[Dry-run validation]
  D2 --> E
  D3 --> E
  D4 --> E
  E --> F[go test / cargo test]
  F --> G[Archive migrated legacy scripts]
```

## Implemented CLI Structure

1. `cmd/hg-sync`: Go-version synchronization across manifest-backed repos.
2. `cmd/hg-pipeline`: language-aware build/vet/test pipeline runner.
3. `cmd/cr8`: wrapper lane for running `hg-pipeline` against `cr8-cli`.
4. `tools/launchers-rs`: zero-dependency Rust launcher primitives (`wofi-geometry`, `truthy`).
