// Package discovery provides MCP Registry integration for server discovery,
// publishing, metadata extraction, and server card serving.
//
// [Client] queries the MCP Registry API for [ServerMetadata], caches results
// with a configurable TTL, and maps HTTP status codes to typed sentinel
// errors. [Publisher] registers or updates a server's metadata in the
// registry using an API token. [MetadataFromConfig] introspects a local
// [registry.ToolRegistry] (and optionally resources/prompts registries) to
// build a [ServerMetadata] value without a live registry round-trip.
// [ServerCardHandler] and [StaticServerCardHandler] serve the
// .well-known/mcp.json server card over HTTP.
//
// # Serving a live server card
//
//	mux.Handle("/.well-known/mcp.json", discovery.ServerCardHandler(discovery.MetadataConfig{
//	    Name:       "my-server",
//	    Version:    "1.2.0",
//	    License:    "MIT",
//	    Homepage:   "https://github.com/example/my-server",
//	    Categories: []string{"developer-tools"},
//	    Install:    &discovery.InstallInfo{Go: "go install github.com/example/my-server@latest"},
//	    Tools:      reg,
//	}))
//
// # Writing a static .well-known/mcp.json file
//
//	card := discovery.ServerCard{
//	    ServerMetadata: discovery.MetadataFromConfig(cfg),
//	    GeneratedAt:    time.Now().UTC(),
//	}
//	if err := discovery.WriteFile(".well-known/mcp.json", card); err != nil {
//	    log.Fatal(err)
//	}
//
// # --contract-write flag convention
//
// Server binaries can expose a --contract-write flag that writes the manifest
// to disk and exits cleanly, making it straightforward for CI pipelines to
// generate .well-known/mcp.json without starting a server transport:
//
//	contractWrite := flag.String(discovery.ContractWriteFlag, "", "write server card and exit")
//	flag.Parse()
//	if err := discovery.HandleContractWrite(*contractWrite, cfg); err != nil {
//	    if errors.Is(err, discovery.ErrContractWritten) {
//	        os.Exit(0)
//	    }
//	    log.Fatal(err)
//	}
package discovery
