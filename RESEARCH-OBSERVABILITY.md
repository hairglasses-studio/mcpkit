# Agent Observability Research

Research date: March 2026. Focused on tracing, debugging, and monitoring patterns for AI agent systems.

---

## 1. Agent-Specific Observability

Traditional APM monitors request/response cycles with predictable control flow. Agent observability is fundamentally different because agents are non-deterministic, reason over intermediate steps, choose tools dynamically, and adapt behavior at runtime.

### What Makes It Different

- **Decision traces**: Every action goes through a reasoning cycle. Observability must capture the "Chain of Thought" at each step -- not just inputs/outputs but _why_ the agent chose a particular path.
- **Reasoning chains**: A single user prompt can trigger 5-6+ internal reasoning steps. Step-level traces capture the model's thought at each stage, linking them into a coherent narrative.
- **Tool call graphs**: Agents select tools dynamically. The graph of which tools were called, in what order, with what parameters, and what was returned forms the core observable artifact.
- **Context evolution**: The agent's context window changes with each step. Tracking what information was added, dropped, or summarized reveals failure modes invisible to traditional monitoring.
- **Cascading failures**: A small error in an early step (e.g., malformed JSON from a tool) cascades through reasoning -- the agent tries to "reason through" bad data, producing hallucinated outputs.

### Key Insight

89% of organizations have implemented some form of agent observability (2025). 62% have detailed tracing that allows inspecting individual agent steps and tool calls. Step-level tracing is now table stakes.

Sources:
- [TrueFoundry: AI Agent Observability](https://www.truefoundry.com/blog/ai-agent-observability-tools)
- [OpenTelemetry Blog: AI Agent Observability](https://opentelemetry.io/blog/2025/ai-agent-observability/)
- [Portkey: Agent Observability](https://portkey.ai/blog/agent-observability-measuring-tools-plans-and-outcomes/)
- [LangChain: State of Agent Engineering](https://www.langchain.com/state-of-agent-engineering)

---

## 2. Tracing Platforms Comparison

### Platform Matrix

| Platform | License | Self-Host | OTel Native | Agent Tracing | Evals | Pricing |
|----------|---------|-----------|-------------|---------------|-------|---------|
| **Langfuse** | MIT (all features) | Yes (Docker/K8s) | Yes | Yes | LLM-as-judge, manual, custom | Free self-hosted; cloud usage-based |
| **LangSmith** | Proprietary | No | Yes (via OTel) | Deep (LangChain/LangGraph) | Yes | Free tier; Plus $39/seat/mo |
| **Arize Phoenix** | Open source | Yes (single container) | Built on OTel | Deep multi-step traces | Yes | Free OSS; Arize AX enterprise SaaS |
| **Braintrust** | Proprietary + free tier | Yes (self-hosted option) | Yes | Yes | Strong eval focus, CI/CD integration | Free 50k obs/mo; Pro $59/mo |
| **Helicone** | Open source (core) | Yes | Partial | Basic | Basic | Free 10k req/mo; $20/seat/mo |
| **OpenLLMetry** | Open source | N/A (library) | Native OTLP | Via backends | Via backends | Free (library only) |
| **Portkey** | Open source gateway | Yes (gateway) | Yes | Framework integrations | Via guardrails | Free tier; enterprise pricing |

### Key Differentiators

**Langfuse**: Best open-source all-in-one. MIT licensed with full feature parity between self-hosted and cloud. Architecture: Postgres + ClickHouse + Redis + S3. Acquired by ClickHouse (Jan 2026). Horizontally scalable. Air-gapped deployment supported. All incoming events persisted to S3 first for data resilience.

**LangSmith**: Best for LangChain/LangGraph shops. Virtually no measurable overhead (best latency). Tightly integrated debugging and visualization. Limited outside LangChain-centric workflows.

**Arize Phoenix**: Best for ML teams already using Arize. Easiest self-hosting (single Docker container vs. Langfuse's multi-component stack). Deep agent evaluation with complete multi-step trace capture. PCI DSS support for financial compliance.

**Braintrust**: Best for eval-heavy workflows. Framework-agnostic with native TypeScript. Unified workspace for PMs and engineers. Strongest CI/CD integration for automated evaluation pipelines.

**Helicone**: Best for cost-focused teams. Proxy-based (change API endpoint, no code changes). Built-in caching and cost intelligence. Per-user/per-component cost breakdowns.

**OpenLLMetry (Traceloop)**: Best for OTel-native stacks. Emits standard OTLP spans. Plugs into any OTel-compatible backend. Library, not platform.

**Portkey**: Best as unified gateway + observability. 60+ guardrails. 40+ real-time metrics. Gartner Cool Vendor in LLM Observability 2025.

### Common Pattern

Use a gateway (Helicone/Portkey) for cost tracking and caching alongside a platform (Braintrust/Langfuse) for evals and quality monitoring. OTel makes this feasible since traces export in a standard format.

Sources:
- [Langfuse Self-Hosting](https://langfuse.com/self-hosting)
- [Softcery: 8 AI Observability Platforms Compared](https://softcery.com/lab/top-8-observability-platforms-for-ai-agents-in-2025)
- [Braintrust: AI Observability Tools 2026](https://www.braintrust.dev/articles/best-ai-observability-tools-2026)
- [Helicone: Complete Guide to LLM Observability](https://www.helicone.ai/blog/the-complete-guide-to-LLM-observability-platforms)
- [Portkey: Gartner Cool Vendor](https://portkey.ai/blog/portkey-in-2025-gartner-cool-vendors-in-llm-observability/)

---

## 3. OpenTelemetry for AI

### Maturity Status

The OTel GenAI semantic conventions are **experimental** (not yet stable). A transition plan exists but stable marking has not happened yet. The `OTEL_SEMCONV_STABILITY_OPT_IN=gen_ai_latest_experimental` env var opts into the latest experimental version.

### Core `gen_ai.*` Attributes (Experimental)

**System & Model:**
- `gen_ai.system` -- GenAI model family (e.g., "openai", "anthropic")
- `gen_ai.request.model` -- Model name requested
- `gen_ai.response.model` -- Model name that actually served the request
- `gen_ai.provider.name` -- Discriminator for provider-specific telemetry flavor

**Request Parameters:**
- `gen_ai.request.max_tokens`
- `gen_ai.request.temperature`
- `gen_ai.request.top_p`
- `gen_ai.request.top_k`
- `gen_ai.request.stop_sequences`
- `gen_ai.request.frequency_penalty`
- `gen_ai.request.presence_penalty`

**Token Usage:**
- `gen_ai.usage.input_tokens`
- `gen_ai.usage.output_tokens`
- Recommended when token counts are readily available (e.g., from streaming responses)

**Response:**
- `gen_ai.response.finish_reasons`
- `gen_ai.response.id`

### Agent-Specific Conventions (Experimental, Draft)

The GenAI SIG has introduced semantic conventions specifically for agentic systems:

**Agent Attributes:**
- `gen_ai.agent.id` -- Unique identifier
- `gen_ai.agent.name` -- Human-readable name (e.g., "Math Tutor")
- `gen_ai.agent.description` -- Free-form description
- `gen_ai.agent.version` -- Version string

**Agent Spans:**
- `create_agent {gen_ai.agent.name}` -- Span for agent creation (span kind: CLIENT)
- `invoke_agent {gen_ai.agent.name}` -- Span for agent invocation (CLIENT or INTERNAL if same process)

**Task Attributes (Proposed):**
- `gen_ai.task.kind` -- Core intent: planning, retrieval, reasoning, execution, evaluation, delegation, synthesis, coordination, clarification, update_memory, other
- `gen_ai.task.state` -- Lifecycle: created, submitted, planned, started, in-progress, paused, suspended, ended
- `gen_ai.task.requester.id`, `gen_ai.task.requester.type` (human | system), `gen_ai.task.requester.role`

**Team/Multi-Agent:**
- Teams as dynamic groups of agents with shared goals
- Attributes for distributing roles, responsibilities, and communication

**Artifacts & Memory:**
- Proposed attributes for tracking artifacts produced by agents
- Memory attributes for agent state persistence

### Three Signal Types

1. **Traces/Spans**: Model invocations, agent steps, tool calls
2. **Metrics**: Token usage histograms, duration, operation counts (e.g., `gen_ai.client.token.usage`, `gen_ai.client.operation.duration`)
3. **Events**: Input/output content, system/user/assistant messages as structured log events

### Provider-Specific Conventions

Exist for: Anthropic, OpenAI, Azure AI Inference, AWS Bedrock. Each extends the base conventions with provider-specific attributes.

### Relevance to mcpkit

The `observability` package should align with these conventions. Key mappings:
- Tool call spans should carry `gen_ai.*` attributes
- Token usage from sampling responses maps to `gen_ai.usage.*`
- Ralph loop iterations map to `gen_ai.task.*` / `gen_ai.agent.*` spans
- MCP server tool handlers can emit `gen_ai.tool.*` style spans

Sources:
- [OTel GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/)
- [OTel GenAI Spans](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/)
- [OTel GenAI Agent Spans](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-agent-spans/)
- [OTel GenAI Metrics](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-metrics/)
- [OTel GenAI Events](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-events/)
- [GitHub Issue #2664: Agentic Systems](https://github.com/open-telemetry/semantic-conventions/issues/2664)
- [GitHub Issue #2665: Task Conventions](https://github.com/open-telemetry/semantic-conventions/issues/2665)
- [OTel GenAI Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/gen-ai/)
- [OpenLLMetry Semantic Conventions](https://www.traceloop.com/docs/openllmetry/contributing/semantic-conventions)

---

## 4. Distributed Tracing

### Cross-MCP-Server Tracing

The core challenge: a single agent request may fan out to multiple MCP servers, each running different tools. End-to-end traces require propagating context across these boundaries.

**Approach 1: W3C Trace Context Headers**
- Standard HTTP header propagation (`traceparent`, `tracestate`)
- Works for Streamable HTTP MCP transport
- Client must propagate Trace ID; server must recognize and continue the trace

**Approach 2: MCP `_meta` Field Convention**
- Transport-agnostic context propagation via the MCP protocol's `_meta` field
- Works across stdio, SSE, and HTTP transports
- Adopted by FastMCP and compatible with both OTel and Langfuse
- More flexible than HTTP-only headers

**Approach 3: MCP Gateway Centralization**
- Gateway aggregates multiple MCP servers
- Injects/extracts trace context at the gateway boundary
- Provides security isolation, observability, and centralized management

### Multi-Agent Trace Architecture

```
User Request
  |
  +-- Agent A (root span)
       |
       +-- LLM Call (child span, gen_ai.* attributes)
       |
       +-- Tool Call: MCP Server 1 (child span, trace context propagated)
       |     |
       |     +-- Database Query (child span)
       |
       +-- Tool Call: MCP Server 2 (child span, trace context propagated)
       |     |
       |     +-- External API Call (child span)
       |
       +-- Agent B Delegation (child span or span link)
             |
             +-- LLM Call (child span)
             +-- Tool Call (child span)
```

### Span Links vs. Parent-Child

- **Parent-child**: Use for synchronous, sequential tool calls within a single agent
- **Span links**: Use for async handoffs, fan-out to multiple agents, or cross-process delegation where causal ordering is unclear
- Datadog recommends span links for asynchronous systems and event-driven architectures

### Implementation Considerations for mcpkit

- The `gateway` package should inject/extract W3C trace context
- The `ralph` package should create agent-level root spans with loop iteration child spans
- Tool call middleware should propagate context via both HTTP headers and `_meta`
- The `dispatcher` package's worker pool should link spans for concurrent tool executions

Sources:
- [Glama: OTel for MCP Analytics](https://glama.ai/blog/2025-11-29-open-telemetry-for-model-context-protocol-mcp-analytics-and-agent-observability)
- [FastMCP Distributed Tracing with OTel and Langfuse](https://timvw.be/2025/06/27/distributed-tracing-with-fastmcp-combining-opentelemetry-and-langfuse/)
- [FastMCP _meta Context Propagation](https://timvw.be/2025/10/14/fastmcp-distributed-tracing-transport-agnostic-context-propagation-with-_meta/)
- [MCP November 2025 Spec](https://medium.com/@dave-patten/mcps-next-phase-inside-the-november-2025-specification-49f298502b03)
- [Fast.io: AI Agent Distributed Tracing Guide](https://fast.io/resources/ai-agent-distributed-tracing/)
- [Datadog: Parent-Child vs Span Links](https://www.datadoghq.com/blog/parent-child-vs-span-links-tracing/)

---

## 5. Debugging Agent Failures

### Failure Taxonomy

Research from 2025 identifies systematic failure categories in multi-agent LLM systems:

**Agent-Level Failures:**
- Improper task decomposition
- Role disobedience (agent ignores assigned role)
- Tool misuse (wrong tool, wrong parameters)
- Hallucinated tool calls (calling nonexistent tools)
- Reasoning errors in intermediate steps

**Workflow-Level Failures:**
- Inter-agent misalignment (agents working at cross purposes)
- Specification flaws (ambiguous or incomplete task definitions)
- Weak verification (no validation of intermediate results)

**Platform-Level Failures:**
- Framework bugs (failure rates up to 86.7% in some frameworks like OpenHands and MetaGPT)
- Malformed data propagation between components
- Context window overflow

### Counterfactual Replay (AgenTracer)

The most promising debugging technique from 2025 research:

1. **Record** the full failure trajectory (all steps, tool calls, intermediate states)
2. **Identify** candidate error steps
3. **Replay** with one step replaced by a "correct" (oracle) action while keeping prior steps unchanged
4. **Adjust** following steps according to the corrected action
5. **Evaluate**: If the trajectory outcome changes from failure to success, that step was the **decisive error**

This reframes failure attribution from "which step looks wrong?" to the causal question: "which single corrective action would have turned failure into success?"

### Practical Debugging Approaches

**Step-through debugging**: Platforms like LangSmith and Langfuse allow stepping through each reasoning step, examining inputs/outputs at each stage.

**Trace comparison**: Compare successful vs. failed traces for the same task type to identify divergence points.

**Evaluation-driven debugging**: Run LLM-as-judge evaluations on individual steps to identify quality degradation points.

**Replay with modified context**: Re-run a failed agent session with modified tool responses to test hypotheses about failure causes.

### Implications for mcpkit

- Ralph loop should capture full trajectories (all steps + intermediate state) for replay
- Tool call middleware should log inputs, outputs, and timing for every call
- The `finops` package's budget exhaustion events are a key failure mode to trace
- Consider a "debug mode" that captures richer context (full prompt, full response) at the cost of storage

Sources:
- [AgenTracer Paper](https://openreview.net/pdf/4ad6b1217a99a5f8e7a76d23157ebf94d0e328d6.pdf)
- [Where LLM Agents Fail (ArXiv)](https://arxiv.org/abs/2509.25370)
- [Why Do Multi-Agent LLM Systems Fail?](https://arxiv.org/pdf/2503.13657)
- [Comprehensive Study of Bugs in LLM Agents](https://arxiv.org/html/2601.15232)
- [Bugs in Modern LLM Agent Frameworks](https://arxiv.org/html/2602.21806)

---

## 6. Metrics That Matter

### Token Economics

| Metric | What It Measures | Why It Matters |
|--------|-----------------|----------------|
| `input_tokens_per_step` | Tokens consumed per reasoning step | Context efficiency; growing = context bloat |
| `output_tokens_per_step` | Tokens generated per step | Verbosity control |
| `total_tokens_per_task` | End-to-end token consumption | Cost attribution |
| `cost_per_task` | Dollar cost per completed task | Unit economics |
| `cost_of_pass` | Cost per successful outcome | True efficiency (accounts for retries) |
| `wasted_tokens` | Tokens from failed/abandoned steps | Optimization target |

### Quality Metrics

| Metric | What It Measures | Why It Matters |
|--------|-----------------|----------------|
| `task_completion_rate` | % of tasks completed successfully | Core effectiveness |
| `pass@1` | Success rate on first attempt | Agent reliability |
| `tool_selection_accuracy` | Correct tool chosen / total tool calls | Decision quality |
| `tool_ndcg` | Quality of tool ranking when multiple relevant | Nuanced tool selection |
| `parameter_f1` | Correct parameter identification for tools | Execution correctness |
| `reasoning_quality` | LLM-as-judge score on reasoning steps | Process quality, not just outcome |
| `goal_completion_steps` | Steps taken vs. optimal path | Efficiency of reasoning |

### Operational Metrics

| Metric | What It Measures | Why It Matters |
|--------|-----------------|----------------|
| `latency_p50/p95/p99` | End-to-end response time percentiles | User experience |
| `step_latency` | Time per individual step | Bottleneck identification |
| `tool_call_latency` | Time waiting for tool responses | External dependency health |
| `error_rate` | Failed requests / total requests | Reliability |
| `retry_rate` | Steps retried / total steps | Stability indicator |
| `context_window_utilization` | % of context window used | Capacity planning |

### The CLASSic Framework

Enterprise evaluation framework with five dimensions:
- **C**ost: API usage, token consumption, infrastructure overhead
- **L**atency: End-to-end response times
- **A**ccuracy: Correctness in selecting and executing workflows
- **S**tability: Consistency across diverse inputs
- **S**ecurity: Resilience against adversarial inputs and prompt injections

### Recommendations for mcpkit

The `finops` package already tracks token usage and budgets. Additional metrics to consider:
- Tool selection accuracy (log which tools the agent considered vs. chose)
- Steps-to-completion histogram
- Cost-of-pass (cost divided by success rate)
- Context window utilization percentage
- Wasted token ratio (tokens in failed/abandoned steps / total tokens)

Sources:
- [Efficient Agents: Reducing Cost (ArXiv)](https://arxiv.org/html/2508.02694v1)
- [CLASSic: LLM Agent Benchmark](https://aisera.com/ai-agents-evaluation/)
- [Rethinking LLM Benchmarks for 2025](https://www.fluid.ai/blog/rethinking-llm-benchmarks-for-2025)
- [Evaluation and Benchmarking of LLM Agents Survey](https://arxiv.org/html/2507.21504v1)

---

## 7. Alerting

### When to Page

**Immediate (PagerDuty-tier):**
- Cost anomaly: Spend exceeds N% of daily budget in M minutes (runaway agent loop)
- Tool failure spike: Tool error rate exceeds threshold (external dependency down)
- Prompt injection detected: Guardrail triggers on suspicious input patterns
- Agent stuck: Loop iterations exceed maximum without progress
- Token budget exhausted: Agent hits hard budget limit mid-task

**Urgent (Slack alert):**
- Quality degradation: LLM-as-judge scores drop below baseline by >N%
- Latency spike: P95 latency exceeds SLA threshold
- Completion rate drop: Task success rate falls below threshold over rolling window
- Unusual tool usage: Agent calling tools outside normal distribution
- Context window pressure: Approaching context limits with increasing frequency

**Informational (Dashboard/Daily digest):**
- Cost trending upward week-over-week
- New tool usage patterns (may indicate model behavior shift)
- Token efficiency changes after model updates
- Evaluation score distributions shifting

### Alert Design Principles

1. **Rate-based, not count-based**: Alert on error _rate_ changes, not absolute counts
2. **Anomaly detection over static thresholds**: Agent behavior varies by task type; static thresholds create noise
3. **Per-agent and per-tool granularity**: A failing tool should not alert for every agent
4. **Budget-aware**: Cost alerts should account for expected variation by time-of-day and task type
5. **Composite signals**: Combine cost spike + quality drop for higher-confidence alerts

### Platform Support

- **Helicone**: Budget alerts, usage analytics alerts
- **Portkey**: 60+ guardrails that can trigger alerts, real-time anomaly detection
- **Langfuse**: Configurable threshold alerts on metrics
- **Datadog AI Observability**: AI-SPM dashboard with prompt injection alerts, cost-harvesting detection
- **Braintrust**: Automated evaluation alerts when quality metrics degrade

### Implications for mcpkit

The `finops` package's budget policies are already an alerting primitive. Consider:
- Exposing alert hooks from `finops` budget exhaustion
- Adding a tool failure rate tracker to `observability`
- Ralph loop iteration count as an alertable metric
- Integration points for external alerting (OTel metrics -> Prometheus -> AlertManager)

Sources:
- [Monte Carlo: Best AI Observability Tools](https://www.montecarlodata.com/blog-best-ai-observability-tools/)
- [Portkey: Guardrails](https://portkey.ai/features/guardrails)
- [Braintrust: AI Observability Tools 2026](https://www.braintrust.dev/articles/best-ai-observability-tools-2026)
- [Coralogix: Best AI Observability Tools 2025](https://coralogix.com/ai-blog/the-best-ai-observability-tools-in-2025/)

---

## 8. Compliance Logging

### Regulatory Requirements

**HIPAA (Healthcare):**
- All activity involving PHI must be logged: prompts, responses, timestamps, who made the request (45 CFR 164.312(b))
- Business Associate Agreement (BAA) required with any LLM provider processing PHI
- Zero data retention or limited retention periods for LLM API calls
- Audit logs for access to protected health information
- Tracking of data uses, disclosures, and automated processing

**SOC 2 (Trust Services):**
- Five criteria: data security, availability, confidentiality, privacy, processing integrity
- Encryption (AES-256), strict access controls, audit logging, continuous monitoring
- Applied to AI systems: model access logs, data flow tracking, output auditing

**GDPR (Data Protection):**
- Right to explanation for automated decisions
- Data minimization in prompts and context
- PII redaction in traces and logs
- Cross-border data transfer restrictions (relevant for cloud LLM providers)

**PCI DSS (Financial):**
- Arize AX specifically supports PCI DSS compliance
- Cardholder data must never appear in traces or model inputs

### PII in Traces

The core tension: rich traces are essential for debugging, but they contain user data.

**Strategies:**
1. **Redaction at ingestion**: Strip PII before writing to trace store. Lossy but compliant.
2. **Tokenization**: Replace PII with reversible tokens. Allows debugging with access controls.
3. **Tiered retention**: Full traces for N days (debugging), redacted for M months (compliance), metadata for Y years (audit).
4. **Encryption at rest**: Encrypt trace content with per-tenant keys. Access requires key + justification.
5. **Air-gapped deployment**: Self-host observability (Langfuse, Phoenix) within VPC. No data leaves perimeter.

### Audit Trail Requirements

An effective agent audit trail must capture:
- Who initiated the request (user identity)
- What the agent was asked to do (task/prompt)
- What tools were called and with what parameters
- What data was accessed or modified
- What the agent's output was
- When each step occurred (timestamps)
- Why the agent made each decision (reasoning trace, if captured)

### Cost of Compliance

Adding GDPR/HIPAA/SOC 2 compliance to production AI agents adds approximately $8k-25k to development costs (2025 estimates), primarily in infrastructure, legal review, and ongoing compliance monitoring.

### Implications for mcpkit

- The `observability` middleware should support configurable PII redaction
- Trace content (prompts, responses) should be optionally encrypted or excluded
- The `security` package's audit logging needs to capture the full agent decision chain
- Consider a compliance mode that automatically redacts tool parameters matching PII patterns
- Self-hosted Langfuse (air-gapped) is the strongest open-source option for regulated environments

Sources:
- [AI Agent Security: GDPR, HIPAA & SOC 2](https://p0stman.com/guides/ai-agent-security-data-privacy-guide-2025.html)
- [Agent Compliance Layer](https://www.agentcompliancelayer.com/)
- [Security & Compliance Checklist for LLM Gateways](https://www.requesty.ai/blog/security-compliance-checklist-soc-2-hipaa-gdpr-for-llm-gateways-1751655071)
- [AI and LLM Data Provenance for Healthcare](https://www.onhealthcare.tech/p/ai-and-llm-data-provenance-and-audit)
- [HIPAA-Compliant AI for Developers](https://www.aptible.com/hipaa/hipaa-compliant-ai)
- [SOC2, HIPAA, GDPR in the Age of AI](https://llm.co/blog/soc2-hipaa-gdpr-ai-compliance)

---

## Summary: Key Takeaways for mcpkit

### Immediate Opportunities

1. **Align `observability` with OTel `gen_ai.*` conventions**: The semantic conventions are experimental but widely adopted. Emitting `gen_ai.system`, `gen_ai.request.model`, `gen_ai.usage.*` attributes from the existing middleware is straightforward and future-proof.

2. **Add agent-level spans to Ralph**: Use `gen_ai.agent.*` and `gen_ai.task.*` attributes on Ralph loop spans. This positions mcpkit for the agentic conventions as they stabilize.

3. **MCP trace context propagation via `_meta`**: The `_meta` field approach is transport-agnostic and already adopted by FastMCP. The `gateway` package should propagate trace context through this mechanism.

4. **PII redaction in traces**: Add optional redaction middleware to `observability`. Even a simple regex-based redactor for emails/SSNs/credit cards would be valuable for regulated users.

### Medium-Term Opportunities

5. **Export to Langfuse/Phoenix**: Native OTLP export already works since both platforms accept OTel traces. But purpose-built exporters with score/evaluation metadata would differentiate.

6. **Cost-of-pass metric**: Combine `finops` cost tracking with task completion tracking to compute this key metric. More useful than raw cost alone.

7. **Counterfactual replay for Ralph**: Record full loop trajectories. Allow replaying with modified tool responses for debugging. This is the cutting-edge debugging technique from 2025 research.

8. **Alert hooks**: Expose `finops` budget events and `observability` metric thresholds as hookable events that users can wire to their alerting systems.

### Watch List

- OTel GenAI conventions moving to stable (will require attribute name changes)
- Langfuse architecture evolution post-ClickHouse acquisition
- A2A protocol observability requirements (relevant for Phase 6)
- MCP spec evolution around built-in trace context support
