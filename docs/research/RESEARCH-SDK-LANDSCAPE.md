# AI Agent SDK & Framework Landscape Research

Last updated: 2026-03-14

---

## 1. Framework Survey (2024-2025)

### Anthropic Claude Agent SDK

- **Core abstractions**: Agentic loop (Gather Context -> Take Action -> Verify Work -> Repeat). Agents have access to Read, Write, Edit, Bash, and custom tools. Custom tools are implemented as in-process MCP servers.
- **Language support**: Python (primary), TypeScript (npm package). No Go SDK.
- **Maturity**: v0.2.74 (pre-1.0). Renamed from Claude Code SDK in late 2025 to reflect broader scope.
- **Key features**: Structured outputs (JSON schema validation), file checkpointing/rewind, 1M context window beta, permission allowlists, hooks for lifecycle customization.
- **Adoption**: Powers Claude Code internally. Growing external adoption for autonomous coding and workflow agents.
- **Differentiator**: Same infrastructure that powers Claude Code -- battle-tested agentic loop. Deep MCP integration since Anthropic created MCP.

### OpenAI Agents SDK

- **Core abstractions**: Agents (LLMs + instructions + tools), Handoffs (agent-to-agent delegation), Guardrails (input/output validation).
- **Language support**: Python and TypeScript with feature parity.
- **Maturity**: Released March 2025 as production-ready successor to Swarm. Provider-agnostic (supports 100+ LLMs).
- **Key features**: Built-in tracing, MCP tool calling, Sessions (persistent memory), Human-in-the-loop, Realtime Agents (voice with interruption detection), function tools with automatic schema generation.
- **Adoption**: Strong. Backed by OpenAI's developer ecosystem.
- **Differentiator**: Minimal primitives (3 core concepts). Dual-language parity. Voice/realtime support.

### Google Agent Development Kit (ADK)

- **Core abstractions**: Code-first agent definitions, multi-agent hierarchies, rich tool ecosystem.
- **Language support**: Python, TypeScript, and **Go** (added November 2025).
- **Maturity**: Active development. Integrated with Vertex AI and Google Cloud. A2A protocol support built in.
- **Key features**: Model-agnostic (Gemini, Claude, Mistral via LiteLLM), bidirectional audio/video streaming, built-in evaluation framework, MCP tool support, Agent2Agent protocol, multi-agent orchestration.
- **Adoption**: Growing. Backed by Google Cloud ecosystem.
- **Differentiator**: Only major vendor SDK with Go support. A2A protocol native. Streaming audio/video. Built-in eval.

### LangChain / LangGraph

- **Core abstractions**: LangChain = agent loop + middleware + provider connectors. LangGraph = stateful graph-based orchestration with explicit state management.
- **Language support**: Python (primary), JavaScript/TypeScript.
- **Maturity**: **Both reached v1.0 in October 2025.** LangGraph GA since May 2025, powering production agents at ~400 companies.
- **Key features**: Durable execution (survives failures), human-in-the-loop interrupts, time-travel debugging, checkpointing, multimodal support, short/long-term memory management.
- **Adoption**: Highest in the ecosystem. De facto standard for production agent deployments.
- **Differentiator**: Most mature. Graph-based state machines. LangSmith observability. Large community and ecosystem. But: criticized for abstraction complexity and churn.

### CrewAI

- **Core abstractions**: Role-based agents with defined personas, tasks with expected outputs, crews as orchestration units.
- **Language support**: Python only.
- **Maturity**: Popular for prototyping. Known limitation: teams frequently hit walls and face 50-80% rewrites to migrate to LangGraph after 3-6 months.
- **Key features**: Simple role/task model, crew orchestration, built-in RAG.
- **Adoption**: High for prototyping, lower for production.
- **Differentiator**: Lowest learning curve. Best for quick multi-agent prototypes. Not recommended for production-scale systems.

### AutoGen v0.4 / Microsoft Agent Framework

- **Core abstractions**: Async event-driven architecture. Message flows and policies. Merged with Semantic Kernel into unified "Microsoft Agent Framework" (October 2025).
- **Language support**: Python and .NET.
- **Maturity**: Preview (GA expected Q1 2026). MIT licensed.
- **Key features**: Session-based state management, type safety, filters, telemetry, graph-based workflows, MCP support, A2A support, OpenAPI-first design. Integrates with Claude Agent SDK.
- **Adoption**: Enterprise-focused. Strong Azure integration.
- **Differentiator**: Enterprise-grade (.NET + Azure). Cross-runtime portability. Merges two battle-tested projects.

### Semantic Kernel

- **Core abstractions**: Now subsumed into Microsoft Agent Framework. Previously: plugins, planners, memory, connectors.
- **Language support**: C#/.NET (primary), Python, Java.
- **Maturity**: GA. Well-established in enterprise .NET shops.
- **Differentiator**: Best option for .NET/C# teams. Enterprise security and identity patterns.

### Haystack

- **Core abstractions**: Pipeline-based architecture. Components connected in DAGs.
- **Language support**: Python only.
- **Maturity**: Stable. Focused on RAG and document processing rather than general agent orchestration.
- **Key features**: Lowest token usage (~1.57k per query). Strong document processing. Pipeline composability.
- **Adoption**: Niche but loyal. Popular in RAG-heavy deployments.
- **Differentiator**: Best for RAG/knowledge retrieval workloads. Low overhead.

### DSPy

- **Core abstractions**: Declarative program synthesis. Modules, signatures, optimizers. Compiles AI programs into effective prompts/weights.
- **Language support**: Python only.
- **Maturity**: Research-oriented, moving toward production. Lowest framework overhead (~3.53ms).
- **Key features**: Automatic prompt optimization, modular AI programs, supports GPT-4/Claude/Gemini/Llama/Mistral.
- **Adoption**: Growing in ML research community.
- **Differentiator**: Unique "optimizer framework" -- optimizes prompts programmatically rather than hand-crafting them. Complementary to orchestration frameworks.

---

## 2. Go Ecosystem Gaps

### What Exists

| Project | Description | Maturity |
|---------|-------------|----------|
| **Google ADK for Go** | Multi-agent, A2A, MCP support | Active (Nov 2025) |
| **Eino (CloudWeGo)** | LLM app framework, LangChain-inspired | Active |
| **Jetify AI SDK** | Unified provider interface (inspired by Vercel AI SDK) | Early |
| **LangChainGo** | Community port of LangChain | Community-maintained |
| **Firebase Genkit for Go** | AI app framework by Google | Active |
| **agent-sdk-go** | Open-source agent framework with handoffs | Early |
| **mcp-go (mark3labs)** | MCP protocol implementation | Stable |
| **mcpkit** | Production-grade MCP toolkit | Active |

### What's Missing in Go (vs Python/TypeScript)

1. **No production-grade agent orchestrator** -- Python has LangGraph (400+ production deployments), Go has nothing equivalent.
2. **No standard agent memory** -- Python has LangChain's memory modules, Mem0, etc. Go has nothing comparable.
3. **No built-in observability for agents** -- Python has LangSmith, Langfuse, Phoenix. Go has generic OpenTelemetry but no agent-specific tracing.
4. **No FinOps/cost control** -- No Go framework tracks token usage, enforces budgets, or exports cost metrics.
5. **No agent evaluation framework** -- Python has deepeval, ragas, LangSmith evals. Go has nothing.
6. **No DAG/workflow engine for agents** -- Python has LangGraph's graph engine, Prefect, Temporal adapters. Go has Temporal but no agent-specific wrapper.
7. **No guardrails/validation framework** -- OpenAI Agents SDK has guardrails built in. Go has nothing.
8. **Limited handoff patterns** -- No Go framework implements OpenAI-style handoffs or Google A2A.

### Where mcpkit Is Positioned

mcpkit is the most feature-complete Go toolkit for MCP-based agent infrastructure. It fills gaps 2 (memory/), 3 (observability/), 4 (finops/), and partially 7 (handler validation). With its roadmap items (orchestrator/, workflow/, handoff/, a2a/), it would address gaps 1, 6, and 8 -- making it the only comprehensive Go agent framework.

---

## 3. Emerging Patterns

### Converging Across All Frameworks

1. **MCP as universal tool protocol** -- Every major framework now supports MCP. Anthropic donated MCP to the Linux Foundation's AAIF in December 2025. Ecosystem projected to grow to $4.5B.

2. **Structured outputs over free-form text** -- All frameworks moving toward JSON schema-validated outputs. Reduces token consumption and improves reliability.

3. **Agent handoffs as first-class concept** -- OpenAI's Handoffs, Google's A2A, LangGraph's multi-agent graphs. The pattern: specialized agents delegate to each other with context transfer.

4. **Context engineering as a discipline** -- "Treating context as a first-class system with its own architecture, lifecycle, and constraints." Not just prompt engineering -- managing what context flows between agents, what gets summarized vs. preserved, entity extraction.

5. **Memory tiers** -- Converging on episodic (conversation), semantic (knowledge), and procedural (learned skills) memory. Short-term vs. long-term. mcpkit's memory/ package aligns with this.

6. **Guardrails running in parallel** -- Input/output validation running concurrently with agent execution, not serially blocking.

7. **Human-in-the-loop as interrupts** -- Not just approval gates, but the ability to inspect and modify agent state at any point, then resume.

8. **Durable execution** -- Agents that survive crashes, persist state, and resume from checkpoints. Critical for long-running tasks (hours/days).

### Emerging But Not Yet Standard

9. **AGENTS.md** -- An open format for describing agent capabilities and handoff instructions. Early but gaining traction alongside MCP.

10. **A2A protocol** -- Google's Agent-to-Agent protocol reached v0.3 with gRPC support. Adopted by Linux Foundation. However, development slowed by September 2025, with ecosystem consolidating around MCP.

11. **Voice/realtime agents** -- OpenAI and Google both offer bidirectional audio/video streaming for agents. Still niche.

12. **DAG-based agent orchestration** -- Moving beyond linear chains to directed acyclic graphs for complex workflows. LangGraph pioneered this.

---

## 4. Developer Experience

### What Makes Agent SDKs Easy to Use

- **Minimal primitives**: OpenAI's 3-concept model (Agent, Handoff, Guardrail) is widely praised for simplicity.
- **Type safety**: Strongly typed tool schemas, validated structured outputs. Microsoft Agent Framework emphasizes this.
- **Automatic schema generation**: Turn any function into a tool (OpenAI, Google ADK). Reduces boilerplate.
- **Built-in tracing**: OpenAI Agents SDK and LangGraph both include tracing out of the box. Developers can visualize agent execution without extra setup.
- **Test harnesses**: mcptest-style test servers. Google ADK includes evaluation framework.

### What Makes Them Hard

- **"Almost right" problem**: 45% of developers say their top frustration is AI solutions that are almost right but not quite, making debugging more time-consuming than writing from scratch.
- **Trust gap**: 46% of developers actively distrust AI tool output accuracy (vs. 33% who trust it).
- **Observability gaps**: Observability and evaluations are the lowest-rated parts of the agent stack. Fewer than 1 in 3 teams are satisfied. Nearly half are evaluating alternatives.
- **Environment/deployment reliability**: 21% of developer challenges relate to configuration and deployment issues.
- **Interface contracts**: 18% struggle with enforcing schema-validated tool inputs/outputs.
- **Abstraction leaks**: LangChain's abstraction complexity and API churn are frequently cited pain points. Teams "graduate" from simple frameworks (CrewAI) and face painful rewrites.
- **Non-determinism**: Debugging non-deterministic agent behavior requires fundamentally different approaches than traditional software debugging.

### Go-Specific DX Advantages

- **Compile-time safety**: Go's type system catches errors that Python discovers at runtime. mcpkit's handler/ package leverages this with TypedHandler generics.
- **Concurrency primitives**: Goroutines and channels are natural fits for parallel tool execution, streaming, and multi-agent coordination.
- **Single binary deployment**: No dependency management hell. Critical for production agent deployments.
- **Performance**: Go's lower latency matters for agent loops making many sequential LLM calls.

---

## 5. Production Readiness

### What Differentiates "Toy" from "Production"

| Capability | Toy | Production |
|-----------|-----|------------|
| State management | In-memory only | Persistent, survives crashes |
| Error handling | Panic/retry | Circuit breakers, fallbacks, graceful degradation |
| Observability | Print statements | OpenTelemetry traces, metrics, structured logs |
| Cost control | None | Token budgets, rate limits, spend alerts |
| Security | API key in env | RBAC, audit logging, input sanitization, DPoP |
| Testing | Manual | Automated test harnesses, eval frameworks |
| Scaling | Single process | Worker pools, concurrency groups |
| Memory | Conversation buffer | Tiered memory with persistence |
| Human oversight | None | Approval gates, state inspection, kill switches |

### Framework Production Readiness Rankings

1. **LangGraph** -- Most production-proven. 400+ companies. GA since May 2025. Durable execution, checkpointing, LangSmith observability.
2. **Microsoft Agent Framework** -- Enterprise-grade but still Preview. Azure integration, .NET patterns, telemetry.
3. **OpenAI Agents SDK** -- Production-ready primitives but younger. Built-in tracing, guardrails.
4. **Google ADK** -- Active development, evaluation tools, but newer.
5. **Claude Agent SDK** -- Battle-tested internally (powers Claude Code) but pre-1.0 externally.
6. **CrewAI** -- Not recommended for production. Migration stories are cautionary tales.

### mcpkit's Production Capabilities

mcpkit already implements many production-grade features that other Go frameworks lack:

- Circuit breakers and rate limiters (resilience/)
- OpenTelemetry middleware (observability/)
- JWT/JWKS auth, RBAC, audit logging (auth/, security/)
- Input sanitization (sanitize/)
- Health checks (health/)
- Token accounting and budget policies (finops/)
- Priority worker pools with concurrency groups (dispatcher/)
- Agent memory with pluggable backends (memory/)

This is a strong differentiator vs. other Go options which are mostly thin wrappers around LLM APIs.

---

## 6. Competitive Positioning

### mcpkit's Unique Position

mcpkit occupies a distinctive niche: **the only production-grade, Go-native, MCP-first agent infrastructure toolkit**. No other Go framework combines:

1. Full MCP spec coverage (100% of 2025-11-25 spec)
2. Production middleware (resilience, auth, security, observability, sanitization)
3. Agent loop runner (Ralph)
4. Token accounting / FinOps
5. Agent memory
6. Multi-server gateway
7. Priority dispatching

### Competitive Advantages

| Feature | mcpkit | Google ADK Go | Eino | LangChainGo |
|---------|--------|---------------|------|-------------|
| MCP spec coverage | 100% | Partial | None | None |
| Auth/Security | JWT, RBAC, DPoP, audit | IAM | None | None |
| Resilience middleware | Yes | No | No | No |
| FinOps/cost tracking | Yes | No | No | No |
| Agent memory | Yes | No | No | No |
| Observability (OTel) | Yes | Partial | No | No |
| Agent loops | Ralph | ADK loop | None | None |
| Multi-server gateway | Yes | No | No | No |
| Worker pool/dispatch | Yes | No | No | No |

### What Would Make mcpkit Stand Out Further

1. **Agent orchestration patterns** (orchestrator/) -- Fan-out/fan-in, pipeline, swarm. This is the biggest gap in Go. LangGraph proved it's the killer feature.
2. **DAG workflow engine** (workflow/) -- Cyclical graphs, state machines, conditional branching. Go's concurrency primitives make this natural.
3. **A2A protocol bridge** (a2a/) -- If A2A regains momentum (Linux Foundation backing), early Go support is valuable. If not, MCP gateway covers inter-agent communication.
4. **Agent handoffs** (handoff/) -- OpenAI-style handoffs are the simplest multi-agent pattern. High value, medium effort.
5. **Evaluation framework** -- No Go agent evaluation tooling exists. Even a basic eval harness would be unique.
6. **AGENTS.md support** -- Emerging standard for agent capability discovery. Early mover advantage.

### Risks

- Google ADK for Go is the biggest competitive threat. Backed by Google, multi-language, A2A-native. However, it lacks production middleware (auth, resilience, observability, finops).
- Python ecosystem dominance: 52% of developers not yet using agents, and when they do, Python is the default. Go adoption depends on teams already committed to Go for their stack.

---

## 7. Integration Patterns

### LLM Provider Integration

| Pattern | Examples | Notes |
|---------|----------|-------|
| Direct API client | OpenAI SDK, Anthropic SDK | Tightest integration, vendor lock-in |
| Provider-agnostic interface | LiteLLM, Vercel AI SDK, Jetify | Swap providers without code changes |
| MCP-mediated | mcpkit, mcp-go | Tools/resources are protocol-level, model-agnostic |

### Vector Store Integration

Most frameworks treat vector stores as pluggable backends behind a retrieval interface. Common pattern: embed -> store -> retrieve -> inject into context. Go options: pgvector, Weaviate Go client, Qdrant gRPC, Pinecone REST.

### External Tool Integration

- **MCP servers**: Becoming the standard. 1000+ servers available. mcpkit's gateway/ aggregates multiple MCP servers.
- **Function calling**: All frameworks support turning functions into tools. Schema auto-generation is expected.
- **OpenAPI/REST**: Microsoft Agent Framework emphasizes OpenAPI-first. Tool definitions from OpenAPI specs.

### Deployment Patterns

- **Single binary** (Go advantage): Deploy agent as a single binary. No Python virtualenv management.
- **Container/K8s**: Standard for production. Long-running agent processes need health checks, graceful shutdown.
- **Serverless**: Challenging for agents due to cold starts and state requirements. Better suited for tool endpoints.
- **Edge**: Go's small binary size enables edge deployment of agent infrastructure.

---

## 8. Open Questions & Unsolved Problems

### Unsolved in the Industry

1. **Agent evaluation is primitive** -- How do you measure if an agent is "good"? Accuracy on benchmarks doesn't predict production quality. No consensus on metrics. 89% have some observability but few are satisfied with it.

2. **Cost predictability** -- Agentic loops can drain tokens unpredictably. Budget policies exist (mcpkit's finops/) but predicting cost before execution remains hard. "Loops, tool misuse, and cost blowups" are top concerns.

3. **Context window management at scale** -- As agents run longer, context accumulates. When to summarize vs. truncate vs. use retrieval? No framework handles this well automatically.

4. **Multi-agent debugging** -- Tracing a single agent is hard enough. Tracing interactions across multiple agents with handoffs, shared state, and parallel execution is an open research problem.

5. **Security model for autonomous agents** -- What permissions should agents have? How do you audit agent actions? How do you prevent prompt injection across agent boundaries? MCP's security model is still maturing.

6. **Hallucination in tool use** -- Agents can hallucinate tool names, parameters, or misinterpret results. Guardrails help but don't eliminate the problem.

7. **Long-running agent reliability** -- Agents running for hours/days. Crash recovery, state persistence, timeout policies. Temporal-style workflow engines are promising but not yet integrated with agent frameworks.

8. **A2A vs. MCP convergence** -- Two competing protocols for agent interoperability. MCP is winning on adoption but A2A addresses agent-to-agent communication that MCP doesn't. Will they merge? Will one die?

9. **Agent identity and trust** -- When Agent A delegates to Agent B, how does B verify A's identity and permissions? DPoP and workload identity are partial answers.

10. **Deterministic testing of non-deterministic systems** -- How do you write reliable tests for agents that produce different outputs each time? Snapshot testing, property-based testing, and eval frameworks are all incomplete answers.

### Specific to Go Ecosystem

11. **No Go agent evaluation framework** -- Python has deepeval, ragas, LangSmith evals. Go has nothing.

12. **Limited Go LLM client options** -- Most providers ship Python-first SDKs. Go clients are often community-maintained or REST wrappers.

13. **No Go equivalent of LangSmith/Langfuse** -- Agent-specific observability platforms don't exist in Go. OpenTelemetry is the best available option.

### Specific to mcpkit

14. **Ralph loop patterns** -- How should multi-tool decisions work in practice? What's the right abstraction for DAG-based agent execution vs. sequential loops?

15. **Gateway scaling** -- How does the multi-server gateway perform under high concurrency? What's the right connection pooling strategy?

16. **Memory backend selection** -- Which storage backends matter most? In-memory for dev, SQLite for single-node, Redis for distributed, Postgres for persistence?

---

## Summary: Strategic Implications for mcpkit

### The Go Agent Framework Gap Is Real

Python dominates with 5+ mature frameworks. Go has Google ADK (backed by Google but lacking production middleware) and a handful of early-stage projects. mcpkit is the most feature-complete Go option for production agent infrastructure.

### MCP-First Is the Right Bet

MCP has won the protocol war for tool integration. Every major vendor adopted it in 2025. Anthropic donated it to the Linux Foundation. Being MCP-first (100% spec coverage) is a strong foundation.

### Production Middleware Is the Moat

Other Go frameworks are thin wrappers. mcpkit's auth, resilience, observability, security, finops, and dispatcher packages are what production teams actually need. This is hard to replicate and compounds over time.

### Highest-Impact Next Steps (by value/effort)

1. **Agent orchestration** (orchestrator/) -- The single biggest gap in Go. LangGraph proved this is what production teams need.
2. **Agent handoffs** (handoff/) -- Simple, high-value pattern. OpenAI proved 3 primitives (Agent, Handoff, Guardrail) is enough.
3. **DAG execution in Ralph** -- Extend existing ralph/ to support DAG-based workflows. Lower effort than a full workflow engine.
4. **AGENTS.md support** -- Emerging standard, easy to implement, positions mcpkit as forward-thinking.
5. **A2A bridge** (a2a/) -- Wait for protocol stability signal. Linux Foundation backing is encouraging but development slowed.

---

## Sources

- [Anthropic Claude Agent SDK Overview](https://platform.claude.com/docs/en/agent-sdk/overview)
- [Building Agents with Claude Agent SDK](https://www.anthropic.com/engineering/building-agents-with-the-claude-agent-sdk)
- [OpenAI Agents SDK Documentation](https://openai.github.io/openai-agents-python/)
- [OpenAI Agents SDK Review (Dec 2025)](https://mem0.ai/blog/openai-agents-sdk-review)
- [Google ADK Documentation](https://google.github.io/adk-docs/)
- [Google ADK for Go Announcement](https://developers.googleblog.com/announcing-the-agent-development-kit-for-go-build-powerful-ai-agents-with-your-favorite-languages/)
- [LangChain and LangGraph 1.0 Milestones](https://blog.langchain.com/langchain-langgraph-1dot0/)
- [Microsoft Agent Framework Overview](https://learn.microsoft.com/en-us/agent-framework/overview/)
- [Semantic Kernel + AutoGen = Microsoft Agent Framework](https://visualstudiomagazine.com/articles/2025/10/01/semantic-kernel-autogen--open-source-microsoft-agent-framework.aspx)
- [CrewAI vs AutoGen Comparison](https://sider.ai/blog/ai-tools/crewai-vs-autogen-which-multi-agent-framework-wins-in-2025)
- [DSPy Framework](https://dspy.ai/)
- [AI Agent Frameworks Comparison (Langflow)](https://www.langflow.org/blog/the-complete-guide-to-choosing-an-ai-agent-framework-in-2025)
- [State of Agent Engineering (LangChain)](https://www.langchain.com/state-of-agent-engineering)
- [A Year of MCP: From Internal Experiment to Industry Standard](https://www.pento.ai/blog/a-year-of-mcp-2025-review)
- [2026 MCP Roadmap](http://blog.modelcontextprotocol.io/posts/2026-mcp-roadmap/)
- [A2A Protocol Announcement](https://developers.googleblog.com/en/a2a-a-new-era-of-agent-interoperability/)
- [A2A Protocol v0.3 Upgrade](https://cloud.google.com/blog/products/ai-machine-learning/agent2agent-protocol-is-getting-an-upgrade)
- [What Happened to Google's A2A?](https://blog.fka.dev/blog/2025-09-11-what-happened-to-googles-a2a/)
- [AI Agents in Production 2025 (Cleanlab)](https://cleanlab.ai/ai-agents-in-production-2025/)
- [Stack Overflow 2025 Developer Survey - AI](https://survey.stackoverflow.co/2025/ai)
- [Developer Challenges in AI Agent Systems (arxiv)](https://arxiv.org/html/2510.25423v1)
- [Go AI Agent Frameworks (Relia Software)](https://reliasoftware.com/blog/golang-ai-agent-frameworks)
- [Go Ecosystem 2025 (JetBrains)](https://blog.jetbrains.com/go/2025/11/10/go-language-trends-ecosystem-2025/)
- [Eino Framework (CloudWeGo)](https://github.com/cloudwego/eino)
- [Designing Agentic Loops (Simon Willison)](https://simonwillison.net/2025/Sep/30/designing-agentic-loops/)
- [Agent Execution Loop (Victor Dibia)](https://newsletter.victordibia.com/p/the-agent-execution-loop-how-to-build)
