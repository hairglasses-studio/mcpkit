# Streaming, Real-Time, and Interactive Patterns for AI Agent Systems

Research compiled March 2026. Covers developments from 2024-2026.

---

## 1. Streaming LLM Output

### Server-Sent Events (SSE)

SSE has become the dominant transport for streaming LLM responses in web applications. It is natively supported in all major browsers, easy to debug with standard browser developer tools, and simpler to build upon than WebSockets for unidirectional server-to-client streaming.

- **Vercel AI SDK** (v5/v6): Uses SSE as its standard streaming protocol. Text content streams using a start/delta/end pattern with unique IDs. Tool call inputs now stream by default, providing partial updates as the model generates them. Tool execution errors are scoped to the tool and can be resubmitted to the LLM.
- **LangGraph** (1.0, October 2025): Multiple streaming modes -- `stream_mode="values"` (complete state after each node), `stream_mode="updates"` (incremental diffs only), and `astream_events()` for real-time token streaming. All exposed via SSE in the LangGraph Platform API.

### Streaming Tool Calls

Different LLM providers handle streaming tool calls differently:

- **OpenAI**: Declares function calls up front in the stream; partial/delta structure differs from non-streaming responses. The Responses API format is the new canonical representation.
- **Anthropic**: Mixes content and tool calling in the stream. Fine-grained tool streaming (beta, `fine-grained-tool-streaming-2025-05-14`) enables streaming tool use parameters without buffering or JSON validation. Interleaved thinking (`interleaved-thinking-2025-05-14`) allows Claude to reason between tool calls.
- **OpenAI Agents SDK**: Implements a bridging mechanism -- `OpenAIChatCompletionsModel` converts Chat Completions chunks into Responses API events using a state tracker that accumulates deltas.

### Adaptive Streaming

Growing pattern: do not stream if there are tool calls (wait for structured output), but enable streaming when generating large text content. This avoids the complexity of partial JSON parsing for tool arguments while still providing responsive UX for text generation.

**Sources:**
- [Vercel AI SDK 5](https://vercel.com/blog/ai-sdk-5)
- [Vercel AI SDK Stream Protocols](https://ai-sdk.dev/docs/ai-sdk-ui/stream-protocol)
- [LangGraph Streaming](https://docs.langchain.com/oss/python/langgraph/streaming)
- [Claude Fine-Grained Tool Streaming](https://docs.claude.com/en/docs/agents-and-tools/tool-use/fine-grained-tool-streaming)
- [OpenAI Agents SDK Streaming](https://openai.github.io/openai-agents-python/streaming/)
- [Agent Streaming Architecture in OpenAI Agents SDK](https://adalflow.sylph.ai/design/agent-streaming.html)

---

## 2. Real-Time Agent Interaction

### Voice Agents

#### OpenAI Realtime API

Persistent WebSocket connection to GPT-4o for low-latency bidirectional audio streaming. Key capabilities:

- **Phrase endpointing / turn detection**: Automatic detection of when the user has finished speaking.
- **Barge-in**: User can interrupt the model's output at any time.
- **Background tool calls**: Long-running function calls no longer disrupt the flow -- the model continues conversation while waiting on results (native in `gpt-realtime`).
- **MCP integration**: Pass a remote MCP server URL into session configuration; the API auto-handles tool calls.
- **Function calling improvements**: Better accuracy on calling relevant functions, at appropriate times, with correct arguments.

#### Google Gemini Live API

Processes continuous streams of audio, video, or text with Gemini 2.5 Flash Native Audio (single model, raw audio processing for dramatically reduced latency).

- **70 languages** supported natively.
- **Affective dialog**: Adapts response style and tone to match user expression.
- **Proactive audio**: Server controls when the model responds.
- **Transcriptions**: Built-in audio transcription support.
- Architecture: WebSocket-based, server-to-server or client-to-server-to-API patterns.

#### Google ADK Streaming

The Agent Development Kit (ADK) introduced at Cloud NEXT 2025 provides a full bidirectional streaming framework:

- **LiveRequestQueue**: Asyncio-based queue for continuous multimodal inputs (text, audio, video blobs). The agent's `run_live` consumes from this queue for near real-time processing.
- **Session persistence**: Sessions stored in databases (SQL, Vertex AI), surviving server restarts and disconnections.
- **Natural interruptibility**: Agent can instantly stop its current action to address new input.

The key architectural insight: the traditional request-response model is not suited for high-concurrency, low-latency interactions with continuous data streams. The "live" agent paradigm uses persistent, bidirectional streaming for asynchronous data flow in both directions.

**Sources:**
- [OpenAI Realtime API Guide](https://developers.openai.com/api/docs/guides/realtime/)
- [Introducing gpt-realtime](https://openai.com/index/introducing-gpt-realtime/)
- [Gemini Live API Overview](https://ai.google.dev/gemini-api/docs/live-api)
- [ADK Gemini Live API Toolkit](https://google.github.io/adk-docs/streaming/)
- [Beyond Request-Response: Real-time Bidirectional Streaming Multi-agent System](https://developers.googleblog.com/beyond-request-response-architecting-real-time-bidirectional-streaming-multi-agent-system/)

---

## 3. MCP Transport Evolution

### Streamable HTTP (MCP 2025-03-26)

Replaced HTTP+SSE transport from protocol version 2024-11-05. Single endpoint for all communication.

**Why SSE was deprecated:**
- Long-lived SSE connections complicate scaling (load balancers, horizontal scaling).
- Dropped connections lose responses during long-running operations.
- Separate endpoints for requests vs. responses add setup complexity.

**Streamable HTTP mechanics:**
- Server operates as independent process handling multiple client connections.
- Communication through HTTP POST (client-to-server requests) and GET (server-to-client SSE stream).
- Server MAY send JSON-RPC requests and notifications before sending the final response.
- Server can optionally use SSE to stream multiple messages on a single response.
- Bidirectional: servers can send notifications and requests back to clients on the same connection.

**Current limitations:**
- Stateful sessions conflict with load balancers.
- No standard way for registries/crawlers to discover server capabilities without connecting.
- Tool results must complete in full before returning -- you cannot stream partial tool output back (only progress notifications).

### Progress and Cancellation

- **Progress notifications**: `notifications/progress` with percentage, spinner, and optional text field. Part of the protocol specification.
- **Cancellation**: `notifications/cancelled` with request ID and optional reason string. Either side can send cancellation for in-progress requests.

### Streaming Tool Results -- The Gap

This is the most actively discussed limitation in the MCP community (Issues #117, #393, #661):

- Tool results are single self-contained JSON responses. You cannot stream partial results.
- Streaming happens between client and user, NOT between MCP tool and client.
- Progress notifications are the only mechanism for intermediate feedback, but they are informal (no structured partial data).
- This is a fundamental design constraint, not a bug.

### 2026 MCP Roadmap

The next specification release is tentatively slated for June 2026. Key planned work:

- **Scalable session handling**: Sessions that can be created, resumed, and migrated across server restarts and scale-out events.
- **Stateless Streamable HTTP**: Evolve transport to run statelessly across multiple server instances behind load balancers.
- **Bidirectional at scale**: Rethinking how Elicitations (human input) and Sampling (agentic LLM calls) work over Streamable HTTP.
- **WebSocket transport**: Proposed in SEP-1288 for long-lived bidirectional communication with session persistence across network interruptions. MCP is NOT adding more official transports this cycle -- keeping the set small is a deliberate design principle.

**Sources:**
- [MCP Transports Specification](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports)
- [Why MCP Deprecated SSE](https://blog.fka.dev/blog/2025-06-06-why-mcp-deprecated-sse-and-go-with-streamable-http/)
- [MCP Cancellation](https://modelcontextprotocol.io/specification/2025-03-26/basic/utilities/cancellation)
- [Streaming Tool Results Issue #117](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/117)
- [The 2026 MCP Roadmap](http://blog.modelcontextprotocol.io/posts/2026-mcp-roadmap/)
- [Exploring the Future of MCP Transports](http://blog.modelcontextprotocol.io/posts/2025-12-19-mcp-transport-future/)
- [MCP Streamable HTTP Security (Auth0)](https://auth0.com/blog/mcp-streamable-http/)

---

## 4. Streaming Tool Results

### Current State Across Frameworks

| Framework | Partial Tool Results | Progress | Cancellation |
|-----------|---------------------|----------|-------------|
| MCP | No (single JSON response) | Yes (notifications/progress) | Yes (notifications/cancelled) |
| OpenAI Agents SDK | No (tool completes, then returns) | Via streaming events | Via Runner cancellation |
| LangGraph | Yes (via streaming callbacks) | Via stream events | Via async cancellation |
| Vercel AI SDK | Partial inputs stream; results do not | Via stream protocol | Via AbortController |
| Google ADK | Yes (via LiveRequestQueue) | Built into streaming | Via barge-in |

### Patterns for Long-Running Tools

1. **Progress notifications**: Send periodic updates with percentage/status text (MCP pattern).
2. **Streaming callbacks**: Supply a callback function that receives intermediate content (LangGraph, Semantic Kernel pattern).
3. **Background execution**: Tool runs in background; model continues conversation; results arrive asynchronously (OpenAI Realtime pattern).
4. **Chunked results**: Tool emits result chunks that the framework reassembles (not yet standardized in MCP).

### The Streaming Tool Results Problem

The fundamental tension: MCP tools return structured JSON results that the LLM needs to parse. Streaming partial JSON is problematic because:
- The LLM cannot act on incomplete JSON.
- Partial results may not be semantically meaningful.
- Error handling becomes complex with partial state.

Practical workarounds in the ecosystem:
- Use progress notifications for status updates (MCP).
- Return early with a "handle" and poll for results (async pattern).
- Stream text-based results through notifications (informal, not standardized).

**Sources:**
- [MCP Advanced Server Capabilities](https://blog.fka.dev/blog/2025-06-11-diving-into-mcp-advanced-server-capabilities/)
- [Streaming HTTP Tool Responses Discussion](https://github.com/jlowin/fastmcp/discussions/429)
- [Microsoft Semantic Kernel Agent Streaming](https://learn.microsoft.com/en-us/semantic-kernel/frameworks/agent/agent-streaming)

---

## 5. Event Streaming Architectures

### Technology Comparison for Agent Event Buses

#### Apache Kafka
- **Strengths**: Append-only log is ideal for agent observability/forensics. Strong horizontal scaling via partitioning and replication. Production-proven at massive scale.
- **Agent use case**: Durable event sourcing for agent actions, audit trails, replay for debugging. "Kafka isn't just a bus; it's your forensic record of agent behavior."
- **Weakness**: Operational complexity, higher latency than alternatives.

#### NATS (with JetStream)
- **Strengths**: Extremely low latency, high throughput. Unified platform with JetStream Streams (actions/results), KV Store (sessions/state), Object Store (models/embeddings).
- **Agent use case**: Real-time agent coordination, session state management, lightweight pub/sub for agent events.
- **Maturity**: JetStream, KV Store, and Object Store now production-ready. NATS has evolved from lightweight pub/sub into a unified messaging platform.

#### Redis Streams
- **Strengths**: Message persistence, consumer groups for load balancing, message acknowledgment. Operational simplicity of Redis. Sweet spot for microservices architectures.
- **Agent use case**: Simple event-driven agent communication, message queuing between agent components.
- **Weakness**: Less suitable for very high-throughput or long-term event retention.

### Architecture Patterns

- **Agent event bus**: Agents publish actions/observations to a shared event stream. Other agents or orchestrators subscribe and react.
- **Event sourcing for agents**: All agent decisions recorded as immutable events. Enables replay, debugging, and auditing.
- **CQRS for agent state**: Separate read/write models for agent state. Event streams feed both real-time processing and analytics.

**Sources:**
- [How Kafka Improves Agentic AI (Red Hat)](https://developers.redhat.com/articles/2025/06/16/how-kafka-improves-agentic-ai)
- [NATS as Unified Cloud-Native Backbone](https://dev.to/thedonmon/beyond-kafka-and-redis-a-practical-guide-to-nats-as-your-unified-cloud-native-backbone-4g86)
- [Redis Streams Interservice Communication](https://redis.io/tutorials/howtos/solutions/microservices/interservice-communication/)

---

## 6. Multiplayer Agents

### Current State

The industry is entering the era of "multiplayer AI" where agents participate in team conversations, maintain context across interactions, and coordinate with both humans and other AI systems.

### Platform Capabilities

- **Amazon Bedrock Multi-Agent** (GA March 2025): Multiple specialized AI agents working together on complex multi-step tasks. Orchestrator pattern with agent delegation.
- **Microsoft AutoGen v0.4**: Asynchronous, event-driven architecture. Flexible collaboration patterns with reusable components. Stronger observability built in.
- **Google Agent2Agent (A2A) Protocol**: Common open language for agent collaboration, partnered with 50+ industry leaders. Complementary to MCP (A2A = agent-to-agent, MCP = agent-to-tool).

### Human-Agent Collaboration Patterns

- **Progressive autonomy**: Start with heavy human involvement, gradually reduce as the system proves reliability.
- **Context-aware teamwork**: Agents embedded in workflows understand codebase, track issues, observe decisions. Propose solutions using accumulated context rather than requiring re-explanation.
- **Human-in-the-loop streaming**: LangGraph and similar frameworks support real-time human approval gates during streaming execution. The agent pauses, presents its plan, and resumes after human confirmation.

### Open Challenges

- **Memory as bottleneck**: Memory is the bottleneck of multi-agent scale. Must be designed as a data architecture problem.
- **Conflict resolution**: When multiple agents or users modify shared state, no standard conflict resolution protocol exists.
- **Trust and evaluation**: Evaluation evolved from passive metric to active architectural component in 2025. Trust became the operational bottleneck.

**Sources:**
- [The Next Era of AI: From Single User to Team Collaboration](https://thenewstack.io/the-next-era-of-ai-from-single-user-to-team-collaboration/)
- [Amazon Bedrock Multi-Agent Collaboration](https://aws.amazon.com/blogs/aws/introducing-multi-agent-collaboration-capability-for-amazon-bedrock/)
- [AI Agent Architecture Patterns in 2025](https://nexaitech.com/multi-ai-agent-architecutre-patterns-for-scale/)
- [Lessons from 2025 on Agents and Trust (Google Cloud)](https://cloud.google.com/transform/ai-grew-up-and-got-a-job-lessons-from-2025-on-agents-and-trust)

---

## 7. Edge/Local Agents

### WebAssembly for Agent Execution

WebAssembly has moved beyond experimental use cases and actively powers production AI workloads (confirmed at WASM I/O 2025).

**Key advantages:**
- Cold start times under 1 millisecond (orders of magnitude faster than containers).
- Energy savings up to 75% and cost reductions exceeding 80% for hybrid edge-cloud agent workloads.
- **WASI-NN**: Standardized interface for Wasm modules to interact with neural network runtimes on host systems (cloud servers, IoT devices, edge gateways).

**Capabilities:**
- LLM inference, voice-to-text, image analysis at the edge.
- Portable across cloud, edge, and browser environments.
- Sandboxed execution provides security isolation for untrusted agent code.

### Edge AI Inference

The convergence of WebAssembly and AI enables edge-native AI applications:

- Run models locally for latency-sensitive operations.
- Hybrid edge-cloud: route simple tasks to edge, complex tasks to cloud.
- Offline-capable: agents can operate without network connectivity using local models.

### Streaming in Constrained Environments

- WebSocket and SSE work in browser/edge environments natively.
- WASI networking proposals enable Wasm agents to establish network connections.
- Local STDIO transport (MCP) works for on-device agent-to-tool communication.
- Bandwidth constraints favor incremental/delta streaming over full-state transfers.

**Sources:**
- [Running AI Workloads with WebAssembly (WASM I/O 2025)](https://www.fermyon.com/blog/ai-workloads-panel-discussion-wasm-io-2024)
- [Unleashing AI at the Edge with WebAssembly](https://dev.to/vaib/unleashing-ai-at-the-edge-and-in-the-browser-with-webassembly-4ld8)
- [WebAssembly's Edge Revolution](https://kawaldeepsingh.medium.com/webassemblys-edge-revolution-how-wasm-is-redefining-serverless-computing-in-2025-638e21751386)
- [Edge AI: The Future of AI Inference](https://www.infoworld.com/article/4117620/edge-ai-the-future-of-ai-inference-is-smarter-local-compute.html)

---

## 8. Observability in Real-Time

### OpenTelemetry for Agents

OpenTelemetry has emerged as the de facto standard for agent observability. Key developments in 2025:

- **Semantic conventions for AI**: Standardized attribute names for LLM calls, tool invocations, reasoning steps. Some frameworks (CrewAI) have built-in OTel instrumentation.
- **Sub-spans for tool invocations**: Each tool call and reasoning step gets its own span, enabling visualization of decision paths and bottleneck identification.
- **Automatic instrumentation**: Agents can capture traces, metrics, and logs without modifying source code.
- **200+ components**: OpenTelemetry Collector supports extensive processing, filtering, and export capabilities.

### Real-Time Monitoring Patterns

- **Streaming traces**: Traces emitted as spans complete, not batched. Enables live debugging of agent execution.
- **LangGraph streaming events**: All node transitions, tool calls, and state updates are observable through the streaming API. Transforms agent execution from batch processing to event-driven observable architecture.
- **Langfuse**: Tool call filtering and dashboard visualization added (December 2025). Filter observations by tool calls and add tool calls to dashboard widgets.

### Production Observability Stack

The "7 layers" pattern for production agent observability:

1. **Structured logging**: Captures reasoning process and tool calls.
2. **Metrics**: Success rates, latency, token usage, cost tracking.
3. **Distributed tracing**: Follow requests through multi-agent workflows.
4. **Live dashboards**: Real-time visibility into agent behavior.
5. **Alerting**: Anomaly detection on agent decision patterns.
6. **Evaluation as architecture**: Continuous evaluation of agent output quality, not just post-hoc analysis.
7. **Forensic replay**: Event-sourced agent actions enable replay and debugging.

### Key Insight

Observability in 2025 evolved beyond the traditional three pillars (metrics, logs, traces) to include AI-driven analysis and automated remediation. The gap between a compelling demo and a reliable production system is wider than expected, and observability is the primary tool for closing it.

**Sources:**
- [AI Agent Observability with OpenTelemetry](https://opentelemetry.io/blog/2025/ai-agent-observability/)
- [AI Agents Observability with OTel and VictoriaMetrics](https://victoriametrics.com/blog/ai-agents-observability/)
- [Langfuse Tool Calls Filtering](https://langfuse.com/changelog/2025-12-22-tool-calls-filtering-visualization)
- [7 Layers of Production-Grade Agentic AI](https://medium.com/aimonks/the-7-layers-of-a-production-grade-agentic-ai-system-an-architects-deep-dive-b00e78459fe6)
- [AI Agents in Production: What Actually Works](https://47billion.com/blog/ai-agents-in-production-frameworks-protocols-and-what-actually-works-in-2026/)

---

## Summary: Key Takeaways for mcpkit

### Immediate Relevance

1. **MCP cannot stream tool results today.** Progress notifications and cancellation are the only mechanisms. This is a fundamental design constraint, not a missing feature. Any Ralph streaming work must operate within this boundary.

2. **Streamable HTTP is the production transport.** SSE was deprecated. The single-endpoint model simplifies deployment but creates challenges for stateful sessions at scale.

3. **OpenTelemetry semantic conventions for AI agents are maturing.** The `observability` package should align with these conventions for tool invocation spans and reasoning step tracing.

4. **Event streaming (Kafka/NATS/Redis Streams) is how production multi-agent systems distribute events.** NATS is the strongest fit for low-latency agent coordination; Kafka for durable audit trails.

### Medium-Term Opportunities

5. **Bidirectional streaming is the future of agent interaction.** Google ADK's LiveRequestQueue pattern (asyncio queue + persistent sessions) is the leading architecture for real-time agents.

6. **Background tool execution** (OpenAI Realtime pattern) -- model continues conversation while tools complete -- is becoming standard. This maps well to Ralph's autonomous loop pattern.

7. **WebAssembly agents** are production-ready for edge execution with sub-millisecond cold starts. WASI-NN enables standardized ML inference at the edge.

### Architectural Patterns to Watch

8. **Adaptive streaming**: Stream text but buffer tool calls. Reduces complexity while maintaining responsive UX.

9. **Progressive autonomy**: Start with human-in-the-loop, reduce oversight as trust builds. LangGraph's streaming approval gates are the reference implementation.

10. **Memory as data architecture**: The bottleneck for multi-agent scale. Must be designed as a first-class data architecture concern, not an afterthought.
