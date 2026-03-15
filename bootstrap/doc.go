//go:build !official_sdk

// Package bootstrap provides agent workspace initialization and capability reporting.
//
// It generates a [ContextReport] summarizing all registered tools, resources,
// prompts, and active extensions, so agents can self-describe their capabilities
// at startup or on demand. Reports can be rendered as human-readable text or
// serialized as JSON for downstream consumers.
//
// Key types: [ContextReport], [Config], [GenerateReport].
//
// Basic usage:
//
//	report := bootstrap.GenerateReport(bootstrap.Config{
//	    ServerName: "my-server",
//	    Tools:      toolRegistry,
//	    Resources:  resourceRegistry,
//	    Prompts:    promptRegistry,
//	    Extensions: extRegistry,
//	})
//	fmt.Print(report.FormatText())
package bootstrap
