# Edge Computing, Embedded Agents, and Local-First AI

Research compiled March 2026. Covers developments from 2024-2026.

---

## 1. Local LLMs

### Inference Engines

Four primary engines dominate local LLM inference:

| Engine | Language | Primary Target | Key Strength |
|--------|----------|---------------|--------------|
| **Ollama** | Go | Desktop/Server | Developer ergonomics, OpenAI-compatible API |
| **llama.cpp** | C++ | Universal | Broad hardware support, minimal dependencies |
| **MLX** | Python/C++ | Apple Silicon | Highest throughput on M-series chips |
| **vLLM** | Python | GPU servers | Continuous batching, multi-user serving |

### Performance Benchmarks (Apple Silicon M2 Ultra)

From a [production-grade evaluation](https://arxiv.org/abs/2511.05502) (October 2025):
- MLX: ~230 tokens/sec (short context)
- MLC-LLM: ~190 tokens/sec
- llama.cpp: ~150 tokens/sec
- Ollama: Lower throughput but best developer experience

A January 2026 study found [vllm-mlx exceeds llama.cpp throughput by 21-87%](https://arxiv.org/html/2601.19139v2) across 0.6B-30B parameter models. For multi-user serving, continuous batching provides up to 3.7x throughput scaling.

### Agent Framework Integrations

Ollama has become the de facto local model server. In January 2026, it added [Anthropic Messages API support](https://github.com/ollama/ollama), enabling Claude Code to connect directly to any Ollama model. Major agent frameworks with Ollama support:

- **CrewAI** -- Direct Ollama model integration
- **LangGraph / LangChain** -- Via OpenAI-compatible API
- **OpenAI Agents SDK** -- [Integration guide](https://www.danielkliewer.com/blog/2025-03-12-openai-agents-sdk-ollama-integration) for local agent development
- **Langflow** -- [Full agentic capability](https://www.langflow.org/blog/local-ai-using-ollama-with-agents) via API

### Recommended Models for Edge Agents

- **Qwen 3.5 35B-A3B** -- Balances agentic capability, 256K context, tool calling, hardware efficiency
- **EXAONE 4.0 1.2B** -- Designed for [agentic behavior in tiny footprint](https://dextralabs.com/blog/tiny-ai-models-for-raspberry-pi-edge-ai/)
- **Phi-4 Mini** -- Strong reasoning at small scale
- **Granite 4.0 Micro** -- Enterprise-focused, runs on Raspberry Pi

---

## 2. Edge Agent Architectures

### Hardware Landscape

Edge AI hardware in 2025 spans a wide range:

| Device Class | RAM | AI Acceleration | Use Case |
|-------------|-----|-----------------|----------|
| **ESP32-S3** | 512KB-8MB | None (CPU only) | MCP tool host, sensor gateway |
| **Raspberry Pi 5** | 4-8GB | Hailo-8 hat (26 TOPS) | Local inference, vision analytics |
| **Apple M-series** | 16-192GB | Neural Engine + GPU | Full model serving |
| **NVIDIA Jetson** | 8-64GB | CUDA cores | Industrial vision, robotics |

### Constraints That Matter

**Memory**: Quantized models (INT4/INT8) are essential. A 7B parameter model needs ~4GB in 4-bit quantization. Raspberry Pi 5 with 8GB can run models up to ~4B parameters comfortably.

**Latency**: On-device inference eliminates network round-trips (typically 50-200ms). Local inference on Pi 5 with Hailo-8 achieves [real-time multi-stream vision analytics](https://wiki.seeedstudio.com/raspberry-pi-devices/).

**Connectivity**: Edge agents must handle intermittent or absent connectivity. The [MCP server running on ESP32](https://www.bitfabrik.io/blog/index.php?id=261) demonstrates tool execution without cloud dependency.

**Power**: Battery-powered edge devices need efficient inference. TinyGo binaries and quantized models minimize power draw.

### MCP on Edge Devices

Significant development in deploying [MCP servers directly on edge hardware](https://glama.ai/blog/2025-08-20-implementing-mcp-on-edge-devices):

- **Raspberry Pi**: Full MCP server with HTTP/SSE transport, local tool execution
- **ESP32**: Custom C++ MCP SDK with [automatic JSON schema generation](https://www.bitfabrik.io/blog/index.php?id=261), registry-based tool discovery, memory-safe execution
- **ESP RainMaker**: [Official MCP support](https://developer.espressif.com/blog/2025/07/esp-rainmaker-mcp-server/) for natural language IoT control
- **Espressif Private AI Agents Platform**: [MCP-compatible agent platform](https://developer.espressif.com/blog/2025/12/annoucing_esp_private_agents_platform/) for device makers

### Browser-Based Agents

[WebLLM](https://webllm.mlc.ai/) enables in-browser LLM inference via WebGPU, achieving ~7 tokens/sec prompt evaluation and ~4.27 tokens/sec generation. Combined with WebAssembly for agent logic and Web Workers for background execution, [fully offline browser agents](https://blog.mozilla.ai/3w-for-in-browser-ai-webllm-wasm-webworkers/) are now practical.

[picoLLM](https://picovoice.ai/blog/cross-browser-local-llm-inference-using-webassembly/) runs cross-browser on Chrome, Safari, Edge, Firefox, and Opera, plus Raspberry Pi, Android, and iOS.

---

## 3. Hybrid Cloud-Edge

### Routing Architecture

The hybrid pattern uses local models for simple/fast tasks and cloud APIs for complex reasoning:

```
Request --> Router --> [Complexity Check] --> Local Model (simple)
                                         --> Cloud API (complex)
                                         --> Fallback (if local fails)
```

Policy-based routing evaluates:
- **Bandwidth**: Current network availability
- **Task complexity**: Token count, reasoning depth needed
- **Privacy requirements**: Sensitive data stays local
- **Cost**: Cloud API pricing vs local compute cost
- **Latency SLA**: Time-critical tasks route locally

### Implementation Patterns

[Microsoft Azure AI Foundry](https://spknowledge.com/2025/11/03/mastering-hybrid-ai-workflows-connecting-foundry-local-with-azure-ai-foundry-cloud/) demonstrates the hybrid workflow: local model handles baseline inference, cloud endpoints handle peak load or large models, with [automatic failover](https://digitalthoughtdisruption.com/2025/07/31/deploying-agentic-ai-edge-onprem-hybrid-cloud/) when connectivity drops.

AWS published guidance for [multi-provider AI gateway architectures](https://aws.amazon.com/blogs/opensource/building-intelligent-physical-ai-from-edge-to-cloud-with-strands-agents-bedrock-agentcore-claude-4-5-nvidia-gr00t-and-hugging-face-lerobot/) that normalize access to multiple LLMs behind a consistent API.

### Real-World Example: Robotics

Humanoid robots use cloud-based reasoning to plan multi-step tasks while executing precise physical movements with edge-based vision-language-action models. Cloud agent plans high-level objectives; edge model handles millisecond-level control.

### Relevance to mcpkit

The `gateway/` package already provides multi-server aggregation with namespaced tool routing. A hybrid edge-cloud pattern could extend this to route between local and remote MCP servers based on connectivity, latency, and cost policies. The `finops/` package's budget tracking could inform routing decisions.

---

## 4. WASM for Agents

### Wassette (Microsoft, August 2025)

[Wassette](https://opensource.microsoft.com/blog/2025/08/06/introducing-wassette-webassembly-based-tools-for-ai-agents) is the most significant development in WASM+MCP integration. Key properties:

- **MCP-native**: Exposes WASM Component typed interfaces as MCP tools
- **OCI registry**: Agents autonomously fetch WASM components from container registries
- **Security**: Deny-by-default permission system, per-domain network access control
- **Built on Wasmtime**: Secure sandboxing on par with modern browser engines
- **Zero dependencies**: Single Rust binary, works with Claude Code, Cursor, GitHub Copilot, Gemini CLI

This validates the concept of MCP tools as WASM modules. The architecture:
```
Agent --> MCP Client --> Wassette (MCP Server) --> WASM Component (tool)
                                               --> WASM Component (tool)
                                               --> OCI Registry (fetch new tools)
```

### WASI Progress

[WASI Preview 2](https://eunomia.dev/blog/2025/02/16/wasi-and-the-webassembly-component-model-current-status/) (2025) added:
- Async operations and better resource management
- TCP/UDP sockets, HTTP client/server support
- **WASI AI (Preview)**: Wasm modules interact with ML models and hardware accelerators

### Extism

[Extism](https://github.com/extism/extism) provides the plugin framework layer:
- Host SDK in Go (and 15+ other languages)
- Persistent memory / module-scope variables
- Secure HTTP without WASI
- Runtime limiters and timers

### TinyGo + WASM

[TinyGo v0.39.0+](https://tinygo.org/docs/guides/webassembly/wasi/) compiles Go to WebAssembly Components. The workflow:
1. Define tool interface in WIT (WebAssembly Interface Types)
2. Generate Go bindings with `wit-bindgen-go`
3. Compile with TinyGo to WASM component
4. Load in Extism, Spin, wasmCloud, or Wassette

This means MCP tools written in Go could be compiled to WASM and run anywhere -- browser, edge device, cloud -- with sandboxed isolation.

### Could MCP Tools Be WASM Modules?

**Yes, and this is already happening.** Wassette proves the concept. The path for mcpkit:
1. Define tool interfaces in WIT format
2. Compile handler functions to WASM via TinyGo
3. Host WASM tools in an MCP server using Extism or Wasmtime
4. Distribute tools via OCI registries
5. Benefit: sandboxed execution, portable across architectures, hot-loadable

---

## 5. Offline-Capable Agents

### Agent Caching Patterns

[Agentic Plan Caching (APC)](https://openreview.net/forum?id=n4V3MSqK77) extracts, stores, and reuses structured plan templates across semantically similar tasks. This reduces both cost and latency by avoiding redundant LLM calls.

[Hierarchical caching](https://www.mdpi.com/2504-4990/8/2/30) operates at workflow and tool levels:
- **Workflow level**: Cache entire tool-calling sequences for repeated patterns
- **Tool level**: Cache individual tool results with dependency-aware invalidation
- **Semantic level**: Match queries by embedding similarity, not exact match

### Sync Patterns

For agents that operate offline and sync when connected:

| Pattern | Description | Trade-off |
|---------|-------------|-----------|
| **Event sourcing** | Record all actions as events, replay on sync | Complete history, larger storage |
| **CRDT-based** | Conflict-free replicated data types | Automatic merge, limited data types |
| **Last-write-wins** | Timestamp-based conflict resolution | Simple, potential data loss |
| **Operational transform** | Transform concurrent operations | Complex, precise merging |

### Local-First Architecture

The actor model maps well to offline agents: each agent maintains its own message queue and internal state, processes events independently, and reconciles on reconnection. This aligns with how the `ralph/` loop runner works -- self-contained execution with state tracking.

### Relevance to mcpkit

The `memory/` package with pluggable storage backends is positioned for offline support. Adding CRDT-based or event-sourced storage backends would enable agents that accumulate memory offline and merge when reconnected.

---

## 6. Go Advantages for Edge AI

### Why Go Excels at Edge Deployment

**Single binary deployment**: No runtime, no dependency management, no JIT warm-up. Copy a binary to a Raspberry Pi and run it.

**Cross-compilation**: Build for any target from any host:
```bash
GOOS=linux GOARCH=arm64 go build -o agent-arm64 .
GOOS=linux GOARCH=arm GOARM=7 go build -o agent-armv7 .
GOOS=wasip1 GOARCH=wasm go build -o agent.wasm .
```

**Small memory footprint**: Go's runtime is lightweight compared to Python/Node.js. A typical MCP server binary is 10-20MB with all dependencies statically linked.

**Concurrency model**: Goroutines are ideal for edge agents that need to handle sensor streams, HTTP requests, and inference calls concurrently with minimal overhead.

**WebAssembly target**: Both standard Go and TinyGo compile to WASM. TinyGo produces smaller binaries (often under 1MB) suitable for embedded contexts.

### Go in Production Edge AI

IoT companies deploy Go-based computer vision pipelines on edge devices where the [single binary and low memory footprint](https://medium.com/@nisarg.bhavsar/why-choose-go-for-edge-application-development-496f5415a012) enable efficient operation on resource-constrained hardware.

Go's compiled nature ensures [minimal latency for real-time sensor data processing](https://medium.com/codex/golang-in-2025-927148df4235) and edge inference coordination.

### mcpkit's Position

mcpkit is already well-positioned for edge deployment:
- Pure Go, single binary output
- No CGO dependencies in core packages
- `registry` and `handler` packages have zero external dependencies beyond mcp-go
- Cross-compilation works out of the box for ARM/ARM64
- The middleware chain model is lightweight enough for constrained devices

---

## 7. IoT and Agents

### MQTT + MCP Integration

[EMQX](https://www.emqx.com/en/blog/mcp-over-mqtt) pioneered MCP-over-MQTT, creating a bridge between AI agents and IoT devices:

```
LLM Agent --> MCP Client --> MQTT Broker --> Device (sensors/actuators)
                                          --> Device
                                          --> Device
```

The architecture uses MQTT for device communication (pub/sub, QoS, retained messages) and MCP for AI tool discovery and invocation. The [ESP32 + MCP over MQTT tutorial](https://www.emqx.com/en/blog/esp32-and-mcp-over-mqtt) shows end-to-end implementation.

### IoT-Edge-MCP-Server

The [poly-mcp/IoT-Edge-MCP-Server](https://github.com/poly-mcp/IoT-Edge-MCP-Server) project unifies MQTT sensors, Modbus devices, and industrial equipment into a single AI-orchestrable API. Features:
- Real-time monitoring and alarms
- Time-series storage
- Actuator control
- SCADA and PLC system integration

### Digital Twins

Device models represent physical entities in the cloud with three dimensions:
- **Attributes**: Real-time status (temperature, humidity, state)
- **Functions**: Callable actions (turn on/off, set threshold)
- **Events**: Asynchronous notifications (alarm triggered, battery low)

This maps directly to MCP's tool/resource/prompt model.

### Voice Agents on Microcontrollers

[ElatoAI](https://developers.openai.com/cookbook/examples/voice_solutions/running_realtime_api_speech_on_esp32_arduino_edge_runtime_elatoai/) runs real-time speech AI agents on ESP32 with Arduino, connecting to OpenAI's Realtime API for voice interaction with physical devices.

### Relevance to mcpkit

An MQTT transport for MCP would enable mcpkit servers to communicate with IoT devices natively. The `resources/` package could expose sensor readings as MCP resources, while `registry/` tools could map to actuator commands. The digital twin model aligns with MCP's tool/resource/prompt triple.

---

## 8. Privacy-First

### On-Device Architecture

72% of enterprises prioritize private or hybrid LLM deployments in 2025. The privacy-first stack:

1. **Local model**: Ollama/llama.cpp serving quantized models
2. **Local embeddings**: On-device embedding generation (e5-small, nomic-embed)
3. **Local vector store**: FAISS, Qdrant, or SQLite-based vector search
4. **Local RAG pipeline**: Query embedding -> vector search -> context injection -> local inference
5. **No outbound API calls**: Complete data sovereignty

### Tools and Platforms

[AnythingLLM](https://skywork.ai/blog/anythingllm-review-2025-local-ai-rag-agents-setup/) provides a complete local AI + RAG + agents setup with support for Ollama, multiple vector databases, and workspace-isolated agent memory.

[Espressif's Private AI Agents Platform](https://developer.espressif.com/blog/2025/12/annoucing_esp_private_agents_platform/) brings MCP-compatible private agents to ESP32 devices, ensuring data never leaves the device network.

### Agent Memory Privacy

As agents accumulate session memory, the challenge expands to a "context architecture" managing memory, retrieval, instructions, and guardrails together. Key requirements:
- Memory encrypted at rest
- No telemetry or usage data exfiltration
- Embeddings stored locally, never sent to cloud
- Tool execution audited locally

### Relevance to mcpkit

The `memory/` package with pluggable storage backends can support encrypted local storage. The `secrets/` package already provides secret management patterns. The `security/` package's audit logging could track all agent actions locally for compliance. A local-only deployment mode using Ollama + mcpkit + local vector store would provide a complete privacy-first agent stack.

---

## Key Takeaways for mcpkit

### High-Value Opportunities

1. **WASM tool runtime**: Compile mcpkit tools to WASM via TinyGo. Distribute via OCI registries. Use Extism for sandboxed execution. Wassette has proven this works with MCP.

2. **Hybrid routing in gateway**: Extend `gateway/` to route between local (Ollama) and cloud (Claude/GPT) based on task complexity, connectivity, and `finops/` budget policies.

3. **MQTT transport**: Add MQTT as an MCP transport alongside HTTP/SSE and stdio. Enables IoT device integration without new protocol bridges.

4. **Offline memory sync**: Add CRDT or event-sourcing storage backends to `memory/` for agents that work offline and sync when reconnected.

5. **Edge-optimized build**: Document and test ARM/ARM64 cross-compilation. Ensure no CGO dependencies leak into core packages. Target Raspberry Pi 5 as reference platform.

### Architecture Alignment

mcpkit's existing design maps well to edge patterns:

| mcpkit Package | Edge Application |
|---------------|-----------------|
| `gateway/` | Hybrid cloud-edge routing |
| `memory/` | Offline agent state, local RAG |
| `finops/` | Cost-based routing decisions |
| `ralph/` | Autonomous edge agent loops |
| `registry/` | Tool discovery on constrained devices |
| `dispatcher/` | Priority-based task execution under resource limits |
| `secrets/` | On-device credential management |
| `security/` | Local audit logging, RBAC |

### What Not to Build

- A custom inference engine (Ollama/llama.cpp are mature)
- A vector database (use FAISS/Qdrant/SQLite-vec)
- An MQTT broker (use EMQX/Mosquitto)
- A WASM runtime (use Extism/Wasmtime)

Focus on the integration layer: making mcpkit tools run anywhere (WASM), talk to anything (MQTT transport), and work offline (sync-capable memory).

---

## Sources

### Local LLMs
- [Ollama GitHub](https://github.com/ollama/ollama)
- [Production-Grade Local LLM Inference on Apple Silicon](https://arxiv.org/abs/2511.05502)
- [Native LLM and MLLM Inference at Scale on Apple Silicon](https://arxiv.org/html/2601.19139v2)
- [OpenAI Agents SDK + Ollama Integration](https://www.danielkliewer.com/blog/2025-03-12-openai-agents-sdk-ollama-integration)
- [Local AI: Using Ollama with Agents (Langflow)](https://www.langflow.org/blog/local-ai-using-ollama-with-agents)
- [vLLM or llama.cpp: Choosing the Right Engine](https://developers.redhat.com/articles/2025/09/30/vllm-or-llamacpp-choosing-right-llm-inference-engine-your-use-case)

### Edge Agents
- [7 Tiny AI Models That Run on Raspberry Pi](https://dextralabs.com/blog/tiny-ai-models-for-raspberry-pi-edge-ai/)
- [Edge AI in 2025: Running LLMs on Laptop & Raspberry Pi](https://www.lktechacademy.com/2025/09/edge-ai-llms-laptop-raspberrypi-2025.html)
- [Implementing MCP on Edge Devices](https://glama.ai/blog/2025-08-20-implementing-mcp-on-edge-devices)
- [MCP Edge Deployment Guide 2025](https://www.byteplus.com/en/topic/541258)
- [Seeed Studio Edge Devices Powered By Raspberry Pi](https://wiki.seeedstudio.com/raspberry-pi-devices/)

### Hybrid Cloud-Edge
- [Mastering Hybrid AI Workflows: Foundry Local + Azure Cloud](https://spknowledge.com/2025/11/03/mastering-hybrid-ai-workflows-connecting-foundry-local-with-azure-ai-foundry-cloud/)
- [Deploying Agentic AI in Edge, On-Prem, and Hybrid Cloud](https://digitalthoughtdisruption.com/2025/07/31/deploying-agentic-ai-edge-onprem-hybrid-cloud/)
- [Building Intelligent Physical AI: Edge to Cloud (AWS)](https://aws.amazon.com/blogs/opensource/building-intelligent-physical-ai-from-edge-to-cloud-with-strands-agents-bedrock-agentcore-claude-4-5-nvidia-gr00t-and-hugging-face-lerobot/)

### WASM for Agents
- [Wassette: WebAssembly-based Tools for AI Agents (Microsoft)](https://opensource.microsoft.com/blog/2025/08/06/introducing-wassette-webassembly-based-tools-for-ai-agents)
- [Wassette: Rust-Powered Bridge Between Wasm and MCP](https://thenewstack.io/wassette-microsofts-rust-powered-bridge-between-wasm-and-mcp/)
- [Wassette GitHub](https://github.com/microsoft/wassette)
- [WASI and the WebAssembly Component Model: Current Status](https://eunomia.dev/blog/2025/02/16/wasi-and-the-webassembly-component-model-current-status/)
- [Extism GitHub](https://github.com/extism/extism)
- [TinyGo WebAssembly Guide](https://tinygo.org/docs/guides/webassembly/wasi/)
- [Sandboxing Agentic Developers with WebAssembly (Cosmonic)](https://blog.cosmonic.com/engineering/2025-03-25-sandboxing-agentic-developers-with-webassembly/)

### Browser Inference
- [WebLLM: High-Performance In-Browser LLM Inference](https://github.com/mlc-ai/web-llm)
- [3W for In-Browser AI: WebLLM + WASM + WebWorkers (Mozilla)](https://blog.mozilla.ai/3w-for-in-browser-ai-webllm-wasm-webworkers/)
- [Cross-Browser Local LLM Inference Using WebAssembly (Picovoice)](https://picovoice.ai/blog/cross-browser-local-llm-inference-using-webassembly/)

### Caching and Offline
- [Agentic Plan Caching: Test-Time Memory for LLM Agents](https://openreview.net/forum?id=n4V3MSqK77)
- [Hierarchical Caching for Agentic Workflows](https://www.mdpi.com/2504-4990/8/2/30)
- [AI Agents 2025: Architectures, State Durability, and Caching](https://bytewax.io/blog/real-time-ai-agents-streaming-and-caching/)

### Go for Edge
- [Why Choose Go for Edge Application Development](https://medium.com/@nisarg.bhavsar/why-choose-go-for-edge-application-development-496f5415a012)
- [Golang in 2025: The Future and Its Boundless Potential](https://medium.com/codex/golang-in-2025-927148df4235)
- [Go in the AI/ML Landscape: A Practical Guide](https://medium.com/@vladimirvivien/go-in-the-ai-ml-landscape-a-practical-guide-d36d44f360d2)

### IoT and MQTT
- [MCP over MQTT: Empowering Agentic IoT (EMQX)](https://www.emqx.com/en/blog/mcp-over-mqtt)
- [IoT-Edge-MCP-Server GitHub](https://github.com/poly-mcp/IoT-Edge-MCP-Server)
- [ESP RainMaker MCP Server](https://developer.espressif.com/blog/2025/07/esp-rainmaker-mcp-server/)
- [Building MCP Server on ESP32](https://www.bitfabrik.io/blog/index.php?id=261)
- [ESP32 + MCP over MQTT Tutorial (EMQX)](https://www.emqx.com/en/blog/esp32-and-mcp-over-mqtt)
- [ElatoAI: Realtime Speech AI Agents for ESP32](https://developers.openai.com/cookbook/examples/voice_solutions/running_realtime_api_speech_on_esp32_arduino_edge_runtime_elatoai/)

### Privacy-First
- [Espressif Private AI Agents Platform](https://developer.espressif.com/blog/2025/12/annoucing_esp_private_agents_platform/)
- [AnythingLLM Review 2025: Local AI, RAG, Agents](https://skywork.ai/blog/anythingllm-review-2025-local-ai-rag-agents-setup/)
- [Building Private RAG Systems on Dedicated GPUs](https://www.servermania.com/kb/articles/private-rag-dedicated-gpu-infrastructure)
- [How to Build AI Agent on On-Prem Data with RAG & Private LLM](https://www.intuz.com/blog/how-to-build-ai-agent-on-prem-data-with-rag-llm)
