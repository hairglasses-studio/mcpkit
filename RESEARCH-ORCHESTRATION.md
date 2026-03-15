# Multi-Agent Orchestration Research

Research findings on multi-agent orchestration patterns, coordination protocols, and production considerations for AI systems.
Last updated: 2026-03-14.

---

## 1. Orchestration Patterns

### Pattern Taxonomy

| Pattern | Topology | Control Flow | Best For |
|---------|----------|-------------|----------|
| Fan-out/Fan-in | Star | Central coordinator dispatches parallel tasks, aggregates results | Independent subtasks (research, search, validation) |
| Pipeline (Sequential) | Linear | Each stage processes and passes to next | Document processing, multi-stage reasoning |
| Supervisor/Worker | Hierarchical | Supervisor delegates, monitors, collects | Structured workflows with clear task decomposition |
| Swarm/Mesh | Decentralized | Agents hand off directly to peers based on capability | Open-ended tasks, customer support routing |
| Hierarchical Delegation | Tree | Multi-level supervisors, each managing sub-teams | Complex enterprise workflows with specialization |

### Framework Implementations

**Fan-out/Fan-in:**
- AutoGen v0.4: ConcurrentOrchestration via AgentChat layer; agents work in parallel with results collected by runtime
- LangGraph: Parallel branches in graph with fan-in node; state merging at convergence point
- Semantic Kernel: ConcurrentOrchestration pattern (experimental) — agents process same input independently, results aggregated
- CrewAI: Parallel task execution within Flows; Crew tasks can run concurrently when no dependencies exist

**Pipeline:**
- AutoGen v0.4: Sequential pattern — linear task flow, each agent builds on previous output
- LangGraph: Linear graph edges, each node is an agent step
- Semantic Kernel: SequentialOrchestration — agents process in turn, output becomes next input
- CrewAI: Sequential process within a Crew; tasks execute in defined order

**Supervisor/Worker:**
- LangGraph: `langgraph-supervisor-py` library; supervisor uses tool-calling to delegate to sub-agents. Recent recommendation: use tool-calling approach directly rather than specialized libraries for better context engineering control
- AutoGen v0.4: SelectorGroupChat — LLM or function selects next agent per turn
- Semantic Kernel: GroupChatOrchestration with selection strategy; supports human-in-the-loop at decision points
- OpenAI Agents SDK: Agent-as-tool pattern — manager calls sub-agents as tools, maintains single thread of control

**Swarm:**
- LangGraph: `langgraph-swarm` library — agents with different specializations dynamically hand off control. 40% reduction in end-to-end response time vs supervisor in one case study. Lower token usage because sub-agents respond directly without supervisor "translation"
- AutoGen v0.4: Swarm pattern in AgentChat — agents hand off tasks based on capabilities
- OpenAI Agents SDK: Handoff pattern — agents transfer control via `transfer_to_<agent>` tool calls
- Semantic Kernel: HandoffOrchestration — agents hand off to peers with context transfer

**Hierarchical Delegation:**
- Anthropic multi-agent research system: Lead agent decomposes queries into subtasks, spawns specialized subagents in parallel. Each subagent needs: objective, output format, tool/source guidance, clear task boundaries
- CrewAI: Flows orchestrate multiple Crews; each Crew is a team, Flow is the project plan coordinating teams
- Semantic Kernel: MagenticOrchestration — experimental pattern for dynamic multi-level coordination

### Key Insight: Supervisor vs Swarm Trade-offs

LangGraph benchmarking (2025) found:
- **Supervisor** uses more tokens due to translation overhead but provides better global task visibility
- **Swarm** is faster (40% latency reduction) and cheaper but harder to maintain global state
- **Hybrid** approaches (supervisor with swarm sub-groups) emerging as practical middle ground

Sources:
- [LangGraph Benchmarking Multi-Agent Architectures](https://blog.langchain.com/benchmarking-multi-agent-architectures/)
- [LangGraph Supervisor GitHub](https://github.com/langchain-ai/langgraph-supervisor-py)
- [LangGraph Swarm](https://www.marktechpost.com/2025/05/15/meet-langgraph-multi-agent-swarm-a-python-library-for-creating-swarm-style-multi-agent-systems-using-langgraph/)
- [Agent Orchestration Patterns: Swarm vs Mesh vs Hierarchical](https://gurusup.com/blog/agent-orchestration-patterns)

---

## 2. Agent-to-Agent Protocols

### Google A2A Protocol

**Version History:**
- v0.1 (April 2025): Initial release with HTTP/SSE/JSON-RPC foundation
- v0.2 (June 2025): Donated to Linux Foundation's Agentic AI Foundation (AAIF)
- v0.3 (July 2025): gRPC support, signed Agent Cards, extended SDK coverage (Python, Go, JS, Java, .NET)

**Core Concepts:**

| Concept | Description |
|---------|-------------|
| Agent Card | JSON metadata at `/.well-known/agent.json` — identity, capabilities, skills, endpoint, auth requirements |
| Task | Fundamental unit of work with lifecycle: submitted → working → input-required → completed/failed/canceled/rejected |
| Message | Communication unit within a task; contains Parts (text, data, file) |
| Push Notifications | Webhook-based async updates for long-running tasks (hours/days) |
| Streaming | SSE for real-time task progress; stream begins with Task object, followed by status/artifact events |

**Agent Discovery:** Agents publish Agent Cards specifying supported features (SSE streaming, push notifications, task history). Cards include authentication requirements and capability negotiation.

**Input-Required Loop:** When a server agent needs more info mid-task, it transitions to `input-required` state with a message explaining what it needs. Client gathers info, sends follow-up message, task returns to `working`. This is analogous to MCP's elicitation pattern.

**Governance:** Now under Linux Foundation AAIF (co-founders: OpenAI, Anthropic, Google, Microsoft, AWS, Block). 150+ supporting organizations as of v0.3.

### MCP Inter-Server Communication

MCP does not natively define agent-to-agent communication — it standardizes how an agent connects to external tools, data sources, and services. However, MCP servers can compose into multi-agent systems through:
- **Gateway pattern**: Aggregating multiple MCP servers behind a single namespace (mcpkit's `gateway/` package)
- **Sampling**: Server-initiated LLM requests that enable agentic behavior within MCP
- **Tool chaining**: One server's output feeds another server's input via the host

**MCP + A2A Complementarity:**
- MCP provides the tool/context integration layer (agent-to-tool)
- A2A provides the coordination layer (agent-to-agent)
- Most production systems will use both: MCP for reliable tool integration, A2A for cross-agent orchestration

### OpenAI Agents SDK Handoffs

Handoffs are represented as tools to the LLM (e.g., `transfer_to_refund_agent`). Key characteristics:
- Stay within a single run
- Input guardrails apply only to first agent in chain
- Output guardrails apply only to final agent
- Support both decentralized handoff (agents know about each other) and manager pattern (central agent calls others as tools)

Sources:
- [A2A Protocol Specification](https://a2a-protocol.org/latest/specification/)
- [Google A2A Announcement](https://developers.googleblog.com/en/a2a-a-new-era-of-agent-interoperability/)
- [A2A v0.3 Upgrade](https://cloud.google.com/blog/products/ai-machine-learning/agent2agent-protocol-is-getting-an-upgrade)
- [MCP vs A2A Guide](https://auth0.com/blog/mcp-vs-a2a/)
- [A2A and MCP Complementarity](https://a2a-protocol.org/latest/topics/a2a-and-mcp/)
- [OpenAI Agents SDK Handoffs](https://openai.github.io/openai-agents-python/handoffs/)
- [OpenAI Agent Orchestration](https://openai.github.io/openai-agents-python/multi_agent/)

---

## 3. Framework Comparison

### Core Abstractions

| Framework | Agent Abstraction | Orchestration Model | State Management | Language |
|-----------|------------------|---------------------|------------------|----------|
| AutoGen v0.4 | AssistantAgent, UserProxyAgent | Actor model (Core), AgentChat (high-level) | Event-driven, async runtime | Python |
| CrewAI | Agent (role, goal, backstory) | Crew (team) + Flow (project plan) | Flow state decorator, shared context | Python |
| LangGraph | Node (agent function) | State graph with edges/conditions | Checkpointed state, persistence layer | Python |
| Semantic Kernel | ChatCompletionAgent, OpenAIAssistantAgent | Orchestration patterns (5 types) | Plugin/function-based, kernel state | C#, Python, Java |
| OpenAI Agents SDK | Agent (instructions, tools, handoffs) | Code-first, no pre-defined graph | Runner maintains conversation state | Python |
| Claude Agent SDK | Agent (tools, MCP servers) | Orchestrator-worker, tool-based | External memory, context handoffs | Python, TypeScript |

### Architecture Layers

**AutoGen v0.4** (3 layers):
1. Core: Actor model, event-driven messaging, async runtime
2. AgentChat: High-level API — SelectorGroupChat, Swarm, sequential/concurrent patterns
3. Extensions: Third-party integrations, specialized agents

**CrewAI** (2 layers):
1. Crews: Autonomous agent teams with role-based specialization
2. Flows: Event-driven orchestration above Crews; conditional logic, parallel execution, state management

**LangGraph** (graph-based):
- Agents as nodes, edges define transitions
- Conditional edges for routing decisions
- Built-in persistence (checkpointing) for long-running workflows
- Human-in-the-loop via interrupt nodes

**Semantic Kernel** (5 orchestration patterns):
1. SequentialOrchestration: Pipeline processing
2. ConcurrentOrchestration: Parallel execution
3. GroupChatOrchestration: Multi-role conversation with selection strategy
4. HandoffOrchestration: Peer-to-peer delegation
5. MagenticOrchestration: Dynamic coordination (experimental)

### Error Recovery

| Framework | Approach |
|-----------|----------|
| AutoGen v0.4 | Max-turns limit, termination conditions, exception handlers per agent |
| CrewAI | Flow-level error handling, conditional branching on failure |
| LangGraph | State checkpointing enables replay from last good state; retry edges |
| Semantic Kernel | Plugin-level error handling, orchestration-level termination |
| OpenAI Agents SDK | Guardrails (input/output), max_turns on Runner |
| Claude Agent SDK | Context limit detection → spawn fresh subagent with handoff |

Sources:
- [AutoGen v0.4 Guide](https://atalupadhyay.wordpress.com/2025/03/04/autogen-v0-4-a-complete-guide-to-the-next-generation-of-agentic-ai/)
- [AutoGen Multi-Agent Patterns Deep Dive](https://sparkco.ai/blog/deep-dive-into-autogen-multi-agent-patterns-2025)
- [CrewAI Flows Guide](https://docs.crewai.com/en/guides/flows/first-flow)
- [Semantic Kernel Agent Orchestration](https://learn.microsoft.com/en-us/semantic-kernel/frameworks/agent/agent-orchestration/)
- [Semantic Kernel Multi-Agent Orchestration Blog](https://devblogs.microsoft.com/semantic-kernel/semantic-kernel-multi-agent-orchestration/)
- [Claude Agent SDK Overview](https://platform.claude.com/docs/en/agent-sdk/overview)
- [Anthropic Multi-Agent Research System](https://www.anthropic.com/engineering/multi-agent-research-system)
- [OpenAI Agents SDK vs Claude Agent SDK](https://agentpatch.ai/blog/openai-agents-sdk-vs-claude-agent-sdk/)

---

## 4. Handoff Patterns

### Pattern Classification

| Pattern | Description | Control Flow | Context Transfer |
|---------|-------------|-------------|-----------------|
| Manager-Agent | Central manager delegates subtasks, collects results | Centralized | Manager maintains full context; sends task description to worker |
| Agent-as-Tool | Sub-agent invoked as a tool call; result returned to caller | Centralized | Input/output through tool schema; limited context |
| Peer Delegation (Handoff) | Agent transfers full control to peer | Decentralized | Full conversation history transferred (or summarized) |
| Escalation | Agent recognizes it cannot handle task, escalates up | Hierarchical | Current state + reason for escalation |

### When to Hand Off — Decision Strategies

1. **Capability matching**: Agent checks if task matches its declared skills/tools. OpenAI Agents SDK uses handoff descriptions to let LLM decide routing.
2. **Confidence threshold**: Agent hands off when confidence drops below threshold (common in customer support).
3. **Task complexity**: Simple tasks handled locally; complex tasks escalated. CrewAI uses Flow conditional logic for this.
4. **Context window limits**: Anthropic's system spawns fresh subagents when context limits approach, with careful handoff summaries.
5. **Domain boundaries**: Swarm agents hand off based on domain classification (billing agent → technical support agent).

### Context Transfer Strategies

**Full history transfer:** Pass entire conversation. Simple but expensive — grows unbounded.

**Summary transfer:** Agent summarizes completed work before handoff. Anthropic's research system uses this: "agents summarize completed work phases and store essential information in external memory."

**Structured handoff object:** Define a schema for what gets transferred (task description, key findings, remaining questions). Most robust but requires schema design.

**Shared memory/blackboard:** All agents read/write to shared state. Avoids explicit transfer but requires coordination. AutoGen and CrewAI support this.

### Anthropic's Subagent Design Principles

From their multi-agent research system engineering blog:
- Each subagent needs: **objective**, **output format**, **tool/source guidance**, **clear task boundaries**
- Vague instructions like "research the semiconductor shortage" cause duplicate work, gaps, or misinterpretation
- Lead agent should specify search queries, expected output structure, and scope limits
- Fresh subagents with clean contexts outperform long-running agents approaching context limits

Sources:
- [Anthropic Multi-Agent Research System](https://www.anthropic.com/engineering/multi-agent-research-system)
- [Building Agents with Claude Agent SDK](https://www.anthropic.com/engineering/building-agents-with-the-claude-agent-sdk)
- [OpenAI Handoffs Documentation](https://openai.github.io/openai-agents-python/handoffs/)
- [OpenAI Orchestrating Agents Cookbook](https://cookbook.openai.com/examples/orchestrating_agents)

---

## 5. Consensus and Conflict Resolution

### Decision Protocols

| Protocol | Mechanism | Best For |
|----------|-----------|----------|
| Majority Voting | Each agent votes; majority wins | Reasoning tasks (13.2% improvement over baseline) |
| Consensus | Agents discuss until agreement; timeout → fallback | Knowledge tasks (2.8% improvement) |
| Judge/Arbiter | Separate agent evaluates competing outputs | Quality-critical tasks; code review |
| Debate | Structured argument rounds; agents explicitly agree/disagree | Complex analysis requiring diverse perspectives |
| Hierarchical Arbitration | Superior agent resolves subordinate disagreements | Enterprise workflows with clear authority chains |
| Weighted Voting | Votes weighted by agent reliability/confidence | Heterogeneous agent teams with varying expertise |

### Research Findings (2025)

**Voting vs Consensus (ACL 2025 paper):**
- Voting protocols improve performance by 13.2% on reasoning tasks
- Consensus protocols improve by 2.8% on knowledge tasks
- Increasing agent count improves performance
- More discussion rounds before voting actually *reduces* performance (over-deliberation)

**Adaptive Heterogeneous Multi-Agent Debate (A-HMAD):**
- Diverse specialized agents with dynamic debate routing
- Learned consensus optimizer weights each agent's vote by reliability and argument confidence
- Answer diversity (independent drafting, limited communication) substantially boosts performance

**Key Design Principle:** Deliberation that prompts agents to explicitly agree/disagree and justify stances using logical evidence, weighted by argument validity, maximizes improvement.

Sources:
- [Voting or Consensus? Decision-Making in Multi-Agent Debate (ACL 2025)](https://arxiv.org/abs/2502.19130)
- [Patterns for Democratic Multi-Agent AI: Debate-Based Consensus](https://medium.com/@edoardo.schepis/patterns-for-democratic-multi-agent-ai-debate-based-consensus-part-1-8ef80557ff8a)
- [Adaptive Heterogeneous Multi-Agent Debate](https://link.springer.com/article/10.1007/s44443-025-00353-3)

---

## 6. State Management

### Approaches

| Approach | Description | Trade-offs |
|----------|-------------|------------|
| Shared Memory / Blackboard | All agents read/write to common store | Simple; risk of conflicts; needs locking |
| Event Sourcing | Immutable log of all events as single source of truth | Full audit trail; replay capability; storage cost |
| Checkpointing | Periodic state snapshots for recovery | Good for long-running workflows; LangGraph's primary approach |
| Message Passing | Agents communicate via messages; no shared state | Clean separation; harder to maintain global view |
| CQRS | Separate read/write models for agent state | Scales well; complexity overhead |
| Distributed Shared State | Local state per agent + periodic sync | Low latency reads; eventual consistency |

### Framework State Strategies

**LangGraph:** Checkpointed state is the primary pattern. State is a typed dictionary flowing through graph nodes. Each node can read and modify state. Checkpoints enable:
- Resume from any point after failure
- Human-in-the-loop (pause at interrupt nodes, resume later)
- Time-travel debugging (replay from earlier state)

**AutoGen v0.4:** Event-driven architecture with async runtime. Core layer uses actor model — agents communicate via messages through the runtime. State is maintained per-agent with runtime managing message routing.

**CrewAI Flows:** State management via `@flow.state` decorator. Flow state persists across steps and is accessible to all agents within the flow. Supports conditional branching based on state.

**Swarm Pattern (General):** Blackboard/environment signals. Agents coordinate through shared state without direct peer connections. Works well with message queues (Kafka, RabbitMQ) for decoupled communication.

### Event Sourcing for Multi-Agent Systems

Every command/event an agent processes is recorded in an immutable log:
- Single source of truth across all agents
- Full audit trail for compliance and debugging
- Enables replay and state reconstruction
- Natural fit for passing context between agents (read the log to catch up)
- Maps well to MCP's tool invocation logging (mcpkit's `logging/` package)

Sources:
- [Event-Driven Multi-Agent Systems (InfoWorld)](https://www.infoworld.com/article/3808083/a-distributed-state-of-mind-event-driven-multi-agent-systems.html)
- [Designing Scalable Multi-Agent AI Systems (DZone)](https://dzone.com/articles/multi-agent-ai-ddd-event-storming)
- [Shared Persistent State in Multi-Agent Systems](https://medium.com/@aiforhuman/multi-agent-systems-shared-persistent-state-bd33a1b5030f)

---

## 7. Production Considerations

### Cost Management

| Strategy | Description |
|----------|-------------|
| Model tiering | Use smaller models (SLMs) for simple tasks (intent classification, parameter extraction); reserve large models for complex reasoning |
| Token tracking | Track per-agent, per-task token usage; observability tools enable identification of expensive operations |
| Budget policies | Set per-task and per-agent token budgets; abort or downgrade when exceeded |
| Prompt optimization | Reduce prompt size through summarization; avoid passing full history when unnecessary |
| Caching | Cache tool results and LLM responses for repeated queries |
| Agent pruning | Monitor which agents actually contribute; remove or consolidate underperforming agents |

**mcpkit relevance:** The `finops/` package provides token accounting and budget policies. Ralph loop has cost hooks for per-iteration tracking. These compose naturally into multi-agent cost management.

### Observability and Tracing

**Industry convergence on OpenTelemetry (OTEL):**
- Standard for collecting agent telemetry data
- Prevents vendor lock-in; interoperability across frameworks
- Nested spans capture each sub-action (LLM calls, tool invocations, API interactions)
- Distributed tracing essential for multi-agent workflows spanning multiple services

**Leading platforms (2025-2026):**
- Langfuse: Open-source, flexible tracing
- Arize: Enterprise-grade, OTEL-based
- LangSmith: Native LangChain/LangGraph observability
- Maxim AI: End-to-end simulation + evaluation + observability

**mcpkit relevance:** The `observability/` package provides OTEL tracing/metrics middleware. Extend with parent-child span propagation across agent boundaries for multi-agent tracing.

### Error Propagation and Failure Modes

**Common failure modes in multi-agent production:**
1. **Deadlocks**: 3+ agents in mutual wait cycles; no explicit error signals. Prevent with cycle detection in task graphs, auto-termination after N iterations
2. **Infinite loops**: Agent keeps retrying failed subtask. Prevent with max-turn limits and heartbeat timeouts
3. **Context overflow**: Long conversations exceed model limits. Prevent with summarization and fresh subagent spawning
4. **Cascading timeouts**: Rate limits trigger exponential retries that timeout — unlike traditional systems, each retry sends full context to LLM, making retries expensive
5. **Memory overwrites**: Concurrent agents writing to shared state. Prevent with event sourcing or optimistic concurrency

**Timeout strategies:**
- Total request timeouts (default 30s) — tool calls abandoned if exceeded
- Retry with exponential backoff + jitter (3 retries default)
- Circuit breaker: open after 10% failure rate in 30-second window
- Heartbeat timeouts: kill subtasks that fail to report progress

**Impact:** Properly orchestrated systems experience 3.2x lower failure rates compared to systems lacking formal orchestration.

### Deadlock Prevention

- Detect cycles in task dependency graphs before execution
- Auto-terminate after configurable iteration count
- Every action carries `task_id`, `parent_id`, and `expiry` for traceability
- Hierarchical timeout: parent task timeout > child task timeout (prevents orphaned work)

**mcpkit relevance:** `resilience/` package provides CircuitBreaker and RateLimiter. `dispatcher/` provides priority worker pool with concurrency groups. These are building blocks for production multi-agent resilience.

Sources:
- [Why Multi-Agent AI Systems Fail (Galileo)](https://galileo.ai/blog/multi-agent-ai-failures-prevention)
- [Why Multi-Agent LLM Systems Fail (Augment Code)](https://www.augmentcode.com/guides/why-multi-agent-llm-systems-fail-and-how-to-fix-them)
- [Preventing Cascading Failures in AI Agents](https://www.willvelida.com/posts/preventing-cascading-failures-ai-agents)
- [Why Multi-Agent Orchestration Collapses](https://dev.to/onestardao/-ep-6-why-multi-agent-orchestration-collapses-deadlocks-infinite-loops-and-memory-overwrites-1e52)
- [AI Agents in Production (Microsoft)](https://microsoft.github.io/ai-agents-for-beginners/10-ai-agents-production/)

---

## 8. MCP Relevance — Composition and A2A Integration

### How MCP Servers Compose into Multi-Agent Systems

**Current MCP composition patterns:**
1. **Gateway aggregation** (mcpkit `gateway/`): Multiple MCP servers behind a single namespace with tool routing
2. **Sampling-based agents**: MCP servers use sampling to make LLM calls, enabling autonomous behavior
3. **Tool chaining via host**: Host orchestrates calls across multiple servers, passing outputs as inputs
4. **Resource sharing**: Servers expose data via resources that other agents consume through the host

**Limitations of MCP alone:**
- No native agent discovery (no equivalent of A2A Agent Cards)
- No task lifecycle management (tasks exist in spec but not as coordination primitive)
- No direct server-to-server communication (all flows through host)
- No push notifications for long-running operations (SSE only for connected clients)

### What A2A Patterns Could Enhance MCP

| A2A Feature | MCP Gap It Fills | Potential mcpkit Implementation |
|-------------|-----------------|-------------------------------|
| Agent Cards | No standard discovery mechanism | `discovery/` package could publish/consume Agent Cards at `/.well-known/agent.json` |
| Task lifecycle | No cross-agent task coordination | `orchestrator/` could manage task states (submitted → working → completed) across agents |
| Push notifications | No async updates for disconnected clients | Webhook-based notification system for long-running tool executions |
| Input-required loop | Elicitation is client-only | Enable server-to-server input requests during delegated tasks |
| Capability negotiation | Static tool registration | Dynamic capability exchange based on Agent Card skills |

### Architectural Vision: MCP + A2A in mcpkit

```
┌─────────────────────────────────────────────────┐
│                  Orchestrator                    │
│  (fan-out/fan-in, pipeline, supervisor/worker)   │
├──────────┬──────────┬──────────┬────────────────┤
│ Agent A  │ Agent B  │ Agent C  │ External Agent │
│ (MCP)    │ (MCP)    │ (MCP)    │ (A2A)          │
├──────────┴──────────┴──────────┤                │
│        mcpkit gateway          │  A2A Client    │
│   (namespace, tool routing)    │  (Agent Card   │
│                                │   discovery,   │
│   Tools | Resources | Prompts  │   task mgmt)   │
└────────────────────────────────┴────────────────┘
```

**Phase 6 packages and their roles:**
- `a2a/`: A2A protocol client/server — Agent Card publishing, task lifecycle, push notifications
- `orchestrator/`: Multi-agent coordination — fan-out/fan-in, pipeline, swarm patterns over MCP + A2A agents
- `handoff/`: Manager-agent and peer delegation — context transfer, escalation, agent-as-tool
- `skills/`: Context-aware lazy tool loading — dynamic capability matching based on task requirements

### Design Principles for mcpkit Multi-Agent

1. **Protocol-agnostic orchestration**: Orchestrator should work with both MCP servers (via gateway) and A2A agents (via a2a client) through a common Agent interface
2. **Event-sourced state**: Use immutable event log for cross-agent coordination; aligns with existing `logging/` package patterns
3. **Budget-aware delegation**: Integrate `finops/` token tracking into orchestrator decisions (don't delegate to expensive agent if budget is low)
4. **Resilience by default**: Apply `resilience/` circuit breakers and `dispatcher/` concurrency groups to agent calls
5. **Observable**: Propagate OTEL trace context across agent boundaries; parent span for orchestration, child spans per agent
6. **Ralph as building block**: Ralph loop (autonomous iteration) can power individual agents within the orchestrator; each agent is a ralph loop with its own tool set and budget

Sources:
- [MCP vs A2A (Auth0)](https://auth0.com/blog/mcp-vs-a2a/)
- [Building Multi-Agent Systems with A2A and MCP](https://medium.com/@therahulpahuja/building-multi-agent-systems-with-a2a-and-mcp-protocols-04aff90e0cf0)
- [A2A and MCP Integration](https://a2a-protocol.org/latest/topics/a2a-and-mcp/)
- [Deploying Multi-Agent Systems with MCP and A2A (Anthropic Webinar)](https://www.anthropic.com/webinars/deploying-multi-agent-systems-using-mcp-and-a2a-with-claude-on-vertex-ai)
- [MCP, A2A, ACP: What Does It All Mean? (Akka)](https://akka.io/blog/mcp-a2a-acp-what-does-it-all-mean)
