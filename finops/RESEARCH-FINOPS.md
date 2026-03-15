# FinOps Research: Cost Management for AI Agent Systems

Research conducted March 2026. Sources include Helicone, LiteLLM, RouteLLM, Not Diamond, OpenTelemetry GenAI semantic conventions, Anthropic prompt caching docs, and industry pricing data.

---

## 1. AI Cost Landscape

### Token Pricing Across Providers (March 2026, per 1M tokens)

| Provider | Model | Input | Output | Notes |
|----------|-------|-------|--------|-------|
| **Anthropic** | Claude Opus 4.6 | $5.00 | $25.00 | Cache read: $0.50 (90% off) |
| | Claude Sonnet 4.6 | $3.00 | $15.00 | Cache read: $0.30 |
| | Claude Haiku 4.5 | $1.00 | $5.00 | Cache read: $0.10 |
| **OpenAI** | GPT-4o | $2.50 | $10.00 | |
| | o3 | $2.00 | $8.00 | Reasoning model |
| | o4-mini | $1.10 | $4.40 | |
| | GPT-5.1 | $1.10 | $9.00 | Newest generation |
| | GPT-5-mini | $0.22 | $1.80 | |
| | GPT-5-nano | $0.04 | $0.36 | |
| **Google** | Gemini 3.1 Pro | $2-4 | $12-18 | Context caching available |
| | Gemini 2.5 Pro | $1.25-2.50 | $10-15 | |
| | Gemini 2.5 Flash | $0.30-1.00 | $2.50 | |
| | Gemini 3.1 Flash-Lite | $0.25-0.50 | $1.50 | Cheapest Google option |

### How Costs Scale with Agent Complexity

- **Simple tool call**: 1 LLM invocation = ~500-2K tokens. Cost: $0.001-0.01.
- **Multi-step agent**: 5-20 LLM calls with tool results injected into context. Context grows with each step. Cost: $0.05-0.50 per task.
- **Autonomous loop (Ralph pattern)**: 10-100+ iterations, each carrying accumulated context. Cost: $0.50-50.00+ per task depending on iteration count and context size.
- **Multi-agent orchestration**: Multiple agents each running loops, plus coordination overhead. Cost can reach $1-100+ per complex task.

The key cost driver is **context accumulation** -- each iteration in an agent loop re-sends the full conversation history. A 50-iteration loop with 4K tokens of context per step sends ~100K cumulative input tokens before accounting for growth.

---

## 2. Cost Optimization Strategies

### 2.1 Prompt Caching

**Anthropic's Prompt Caching** is the most mature implementation:
- Cache write: 1.25x base price (5-minute TTL) or 2x (1-hour TTL)
- Cache read: 0.1x base price (90% savings)
- Minimum cacheable: 1,024-4,096 tokens depending on model
- Up to 4 cache breakpoints per request
- Automatic caching mode for conversations (recommended)
- Cache invalidated by: tool definition changes, image additions, thinking parameter changes

**Google's Context Caching**: Available on Gemini models. Cache storage charged per hour ($1-4.50/hour depending on model). Reads at 10% of input price.

**Impact for agent loops**: System prompts, tool definitions, and stable context prefix can be cached. For a 50-iteration Ralph loop with 2K tokens of stable prefix, caching saves ~$0.45 per run on Sonnet 4.6.

### 2.2 Model Routing (Cheap for Simple, Expensive for Complex)

Route requests to the cheapest model that can handle the task:
- **Classification/extraction**: GPT-5-nano ($0.04/MTok) or Haiku ($1/MTok)
- **Summarization/reasoning**: Sonnet ($3/MTok) or GPT-4o ($2.50/MTok)
- **Complex analysis**: Opus ($5/MTok) or o3 ($2/MTok)

A 2-tier routing system (cheap model for 70% of requests, expensive for 30%) can cut costs by 50-70%.

### 2.3 Batching

- **OpenAI Batch API**: 50% discount for async processing
- **Google Batch API**: 50% discount on paid tiers
- Useful for non-real-time workloads: evaluation, data processing, bulk analysis

### 2.4 Context Window Optimization

- Truncate or summarize conversation history instead of sending full context
- Use sliding window: keep last N turns plus a running summary
- Strip tool results that are no longer relevant
- Compress intermediate reasoning steps into summaries

### 2.5 Tool Result Caching

- Cache deterministic tool results (database lookups, API calls with same params)
- Use content-addressable storage keyed on tool name + arguments hash
- TTL-based invalidation aligned with data freshness requirements
- mcpkit's `resilience.Cache` already provides this pattern

---

## 3. Budget Enforcement

### Industry Patterns

**LiteLLM's approach** (most comprehensive open-source implementation):
- **Hierarchical budgets**: Global proxy > Team > User > API Key
- **max_budget** in USD per entity
- **budget_duration**: "30s", "30m", "30h", "30d" -- auto-resets
- **Per-model rate limits**: Different TPM/RPM per model within a key
- **Exceeded behavior**: Returns 401 with budget exceeded message
- **Team inheritance**: Team budget applies to members unless overridden
- Budget reset check runs every 10 minutes (configurable)

**Helicone**: Credit-based system, 0% markup over provider rates, unified billing across providers.

### Enforcement Granularities

| Level | Use Case | Reset Pattern |
|-------|----------|---------------|
| Per-request | Cap individual call cost | Immediate |
| Per-session | Limit agent loop total spend | End of session |
| Per-user | Monthly user quotas | Calendar period |
| Per-team | Department budgets | Monthly/quarterly |
| Per-agent | Limit autonomous agent spend | Per-task or rolling |
| Global | Organization-wide spend cap | Monthly |

### Circuit Breakers for Cost

- **Soft limit**: Log warning, notify, but allow request
- **Hard limit**: Reject request immediately
- **Graceful degradation**: Fall back to cheaper model when approaching limit
- **Emergency stop**: Kill all agent loops when global budget hit

### What mcpkit's finops Does Today

The existing `finops` package implements:
- Token-based budget with `TokenBudget` config (hard limit, pre-call check)
- `OnBudgetExceeded` callback for custom handling (notification, logging)
- Estimation-based accounting (4-chars-per-token heuristic)

**Gaps vs. industry**:
- No dollar-based budgets (only token counts)
- No per-user/per-team/per-session scoping
- No budget reset/duration support
- No hierarchical budget inheritance
- No graceful degradation (model fallback on approaching limit)
- No rate limiting (TPM/RPM)

---

## 4. Token Accounting

### What to Track

| Metric | Source | Notes |
|--------|--------|-------|
| Input tokens | API response `usage.prompt_tokens` | Actual, not estimated |
| Output tokens | API response `usage.completion_tokens` | Actual |
| Cache write tokens | `usage.cache_creation_input_tokens` | Anthropic-specific |
| Cache read tokens | `usage.cache_read_input_tokens` | Anthropic-specific |
| Reasoning tokens | `usage.completion_tokens_details.reasoning_tokens` | OpenAI o-series |
| Tool use overhead | System + tool definitions + tool results | Often 30-50% of total |

### Estimation vs. Actual

- **Estimation** (mcpkit's current approach): `len(text)/4` heuristic. Inaccurate for non-English, code, JSON. Typical error: 20-40%.
- **Actual** from API response: Exact but requires integration with the LLM provider response. Not available at the MCP tool layer -- the MCP server doesn't call the LLM directly.
- **Tokenizer-based**: Use tiktoken (OpenAI) or Anthropic's tokenizer. Accurate to ~99% but adds dependency and latency.

### Cost Attribution in Multi-Agent Systems

Multi-agent cost attribution is an unsolved problem in the industry. Approaches:

1. **Request-scoped tracking**: Tag each LLM call with agent ID, session ID, task ID. Attribute cost to the initiating agent.
2. **Proportional attribution**: If Agent A delegates to Agent B, split cost by contribution (e.g., A pays for coordination, B pays for execution).
3. **Full-path attribution**: Record the full call chain. Cost rolls up through the hierarchy.
4. **Context propagation**: Use context.Context to carry cost attribution metadata through agent handoffs (mcpkit's middleware pattern supports this naturally).

### What mcpkit Should Do

Since mcpkit operates at the MCP tool layer (not the LLM call layer), it can:
- Track tool invocation frequency, duration, and estimated token cost of tool I/O
- Provide hooks for injecting actual token counts from the sampling layer
- Aggregate by tool name, category, session, agent ID
- Export metrics for correlation with LLM-layer cost data

---

## 5. Model Routing

### RouteLLM (UC Berkeley, open-source)

Architecture: Binary router that decides "strong model" vs. "weak model" per query.

**Routing strategies**:
1. **Matrix Factorization** (recommended): Trained on preference data, predicts which model would win
2. **BERT Classifier**: Neural network on preference data
3. **Semantic Weighted Ranking**: Elo-based scoring weighted by prompt similarity
4. **Causal LLM**: LLM-based classifier
5. **Random**: Baseline

**Results**: 85% cost reduction while maintaining 95% of GPT-4 performance. Outperforms commercial offerings while being 40%+ cheaper.

**Key insight**: The cost threshold is user-configurable. Lower threshold = more requests to cheap model = more savings but lower quality.

### Not Diamond (commercial)

- Meta-model architecture that learns when to invoke specific LLMs
- Claims to outperform every individual model on evaluation benchmarks
- Supports cost/latency tradeoff configuration
- API: pass array of candidate models, Not Diamond picks the best one

### Martian (commercial)

- Model router focused on cost-quality optimization
- Limited public technical documentation

### LiteLLM Router

Production-grade routing with multiple strategies:
- **Weighted shuffle** (default): Distribute by RPM/TPM weights
- **Cost-based**: Pick cheapest deployment that isn't rate-limited
- **Latency-based**: Pick fastest deployment (with time window averaging)
- **Least-busy**: Pick deployment with fewest in-flight requests
- **Rate-limit aware**: Filter out deployments exceeding TPM/RPM limits
- **Deployment ordering**: Priority-based with automatic failover
- **Cooldown**: Auto-exclude failing deployments for configurable period
- **Traffic mirroring**: Shadow traffic for A/B testing

### MCP Relevance

Model routing happens above the MCP layer (the MCP server doesn't pick models). However, mcpkit could:
- Expose tool complexity metadata that a router could use
- Track per-tool latency/cost to inform routing decisions
- Provide a `gateway` integration that routes tool calls to different MCP servers based on cost

---

## 6. Caching Strategies

### Exact Caching

- Cache keyed on exact input (tool name + arguments hash)
- Highest hit rate for deterministic tools (lookups, calculations)
- mcpkit's `resilience.Cache` already implements this with TTL
- Zero false positives

### Semantic Caching

- Cache keyed on embedding similarity of input
- "What's the weather in NYC?" and "NYC weather?" return same cached result
- Requires embedding model call (adds latency and cost for cache check)
- Risk of false positives (similar but different queries)
- Tools: GPTCache, Redis Vector Similarity Search
- Break-even: Only worthwhile for expensive operations with high semantic overlap

### Anthropic Prompt Caching (detailed above)

- Server-side KV cache of prompt prefixes
- 90% cost reduction on cached portions
- Automatic mode recommended for conversations
- Critical for agent loops: cache system prompt + tool definitions + stable context

### Result Memoization

- Application-level caching of LLM responses for identical prompts
- Useful for: few-shot example generation, classification of known categories
- Storage: In-memory (fastest), Redis (shared), disk (persistent)
- Invalidation: TTL-based, version-based (prompt template version), manual

### Multi-Level Cache Architecture

```
Request -> Exact cache (in-memory, <1ms)
        -> Semantic cache (Redis + embeddings, ~50ms)
        -> Prompt cache (provider-side, ~100ms savings on TTFT)
        -> Full LLM call (~500ms-5s)
```

---

## 7. Open-Source / Self-Hosting Alternatives

### When Self-Hosting Makes Sense

| Factor | API Preferred | Self-Host Preferred |
|--------|--------------|-------------------|
| Volume | < 1M tokens/day | > 10M tokens/day |
| Latency | Acceptable 200-2000ms | Need < 100ms |
| Data privacy | Non-sensitive | Regulated (healthcare, legal, finance) |
| Model quality needs | State-of-the-art required | "Good enough" acceptable |
| Team | No ML infra team | Has GPU/ML ops capability |
| Budget predictability | Variable OK | Need fixed costs |

### Cost Comparison Framework

**API costs** (variable):
```
Monthly cost = (input_tokens * input_price + output_tokens * output_price) * requests_per_month
```

**Self-hosted costs** (mostly fixed):
```
Monthly cost = GPU_rental + electricity + bandwidth + ops_engineering_time
```

**Break-even example** (Llama 3.1 70B on 2x A100):
- GPU rental: ~$3,000/month (cloud) or amortized ~$1,500/month (owned)
- Throughput: ~50 tokens/sec = ~130M tokens/month
- Equivalent API cost at $1/MTok: $130/month (much cheaper via API)
- Equivalent API cost at $15/MTok (Sonnet-class): $1,950/month (approaching break-even)
- At $25/MTok (Opus-class): $3,250/month (self-hosting wins if quality is acceptable)

**Key insight**: Self-hosting only makes economic sense at high volume AND when the open-source model quality is acceptable for the use case. For agent systems requiring frontier model reasoning, API access remains more cost-effective.

### Notable Self-Hosted Options

- **vLLM**: High-throughput serving engine, PagedAttention for efficient memory
- **Ollama**: Easy local deployment, good for development/testing
- **PrivateGPT**: RAG-focused, zero cloud cost, CPU-capable
- **TGI (Hugging Face)**: Production-grade serving with continuous batching

---

## 8. Prometheus/Grafana for AI

### OpenTelemetry GenAI Semantic Conventions (Development status)

Standard metric names defined by OTel:

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `gen_ai.client.token.usage` | Histogram | `{token}` | Input/output token counts |
| `gen_ai.client.operation.duration` | Histogram | `s` | End-to-end LLM call duration |
| `gen_ai.server.request.duration` | Histogram | `s` | Server-side request duration |
| `gen_ai.server.time_per_output_token` | Histogram | `s` | Generation speed |
| `gen_ai.server.time_to_first_token` | Histogram | `s` | TTFT latency |

**Required attributes**: `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.token.type` ("input"/"output"), `gen_ai.request.model`.

**Notable gap**: No cost metrics in the OTel spec. Cost must be computed application-side from token counts and pricing tables.

### Recommended Metrics for MCP/Agent Systems

Beyond the OTel standard, agent systems should export:

**Cost metrics** (custom):
```
mcp_tool_cost_dollars_total{tool, category, model}       -- Counter
mcp_session_cost_dollars{session_id, agent_id}            -- Gauge
mcp_budget_remaining_ratio{scope, scope_id}               -- Gauge (0-1)
mcp_budget_exceeded_total{scope}                          -- Counter
```

**Tool invocation metrics**:
```
mcp_tool_invocations_total{tool, category, status}        -- Counter
mcp_tool_duration_seconds{tool, category}                 -- Histogram
mcp_tool_input_tokens{tool, category}                     -- Histogram
mcp_tool_output_tokens{tool, category}                    -- Histogram
```

**Agent loop metrics** (Ralph-specific):
```
ralph_loop_iterations_total{task_id, agent_id}            -- Counter
ralph_loop_cost_dollars{task_id, agent_id}                -- Gauge
ralph_loop_duration_seconds{task_id}                      -- Histogram
ralph_loop_budget_utilization_ratio{task_id}              -- Gauge
```

**Cache metrics**:
```
mcp_cache_hits_total{tool, cache_type}                    -- Counter
mcp_cache_misses_total{tool, cache_type}                  -- Counter
mcp_cache_savings_tokens{tool}                            -- Counter
```

### Dashboard Template Recommendations

1. **Cost Overview**: Total spend by model, by tool, by agent. Trend over time. Budget burn rate.
2. **Tool Performance**: Invocations, latency percentiles, error rates per tool.
3. **Agent Efficiency**: Iterations per task, cost per task completion, token efficiency ratio.
4. **Budget Alerts**: Approaching limits (80%, 90%, 100%), exceeded events, anomaly detection.
5. **Cache Performance**: Hit rates, estimated savings, cache size.

---

## 9. MCP Relevance: mcpkit finops Assessment

### What mcpkit finops Has Today

The existing implementation provides a solid foundation:

- **Tracker**: Thread-safe token usage recording with per-tool and per-category aggregation
- **Middleware**: Pre-call budget check, post-call token estimation and recording
- **Estimation**: Character-based heuristic (4 chars/token), extensible via `EstimateFunc`
- **Budget enforcement**: Hard limit with callback hook
- **Summary**: Aggregated view with breakdowns by tool and category

### What's Missing vs. Industry Standards

**Priority 1 -- High Impact**:
- **Dollar-based budgets**: Convert token counts to costs using a pricing table. The token-only model doesn't account for different models having vastly different per-token costs.
- **Budget duration/reset**: Support time-windowed budgets ("$10/hour", "1M tokens/day") with automatic reset, following LiteLLM's pattern.
- **Scoped budgets**: Per-session, per-user, per-agent budgets using context.Context to carry scope. Essential for multi-tenant and multi-agent systems.
- **Prometheus export**: `Exporter` that converts Tracker data into Prometheus metrics. Use the OTel GenAI metric names where applicable.

**Priority 2 -- Important**:
- **Graceful degradation hooks**: When approaching budget limit (e.g., 80%), trigger model downgrade instead of hard stop. Return a "budget warning" alongside results.
- **Rate limiting**: TPM/RPM limits per tool or per scope, complementing the token budget.
- **Actual token injection**: Hook for the sampling layer to inject actual token counts (from LLM API response) instead of relying on estimation.
- **Cost attribution context**: Propagate cost tracking metadata through context.Context for multi-agent attribution.

**Priority 3 -- Nice to Have**:
- **Pricing table**: Built-in cost lookup for major providers (Anthropic, OpenAI, Google). Could be a simple map updated periodically.
- **Usage reports**: Structured export (JSON, CSV) of usage data for billing/chargeback.
- **Anomaly detection**: Flag unusual spending patterns (sudden 10x cost spike).
- **Cache savings tracking**: Integrate with resilience.Cache to track how much caching saves.

### Architecture Recommendation

```
Context (session_id, user_id, agent_id)
    |
    v
finops.Middleware (per-tool tracking)
    |
    v
finops.Tracker (thread-safe aggregation)
    |
    +---> finops.Budget (hierarchical: global > user > session > agent)
    |         |
    |         +---> Hard limit (reject)
    |         +---> Soft limit (warn + degrade)
    |         +---> Duration reset (time-windowed)
    |
    +---> finops.Exporter
    |         |
    |         +---> Prometheus metrics
    |         +---> JSON reports
    |         +---> Webhook alerts
    |
    +---> finops.PricingTable
              |
              +---> Token-to-dollar conversion
              +---> Per-model pricing lookup
```

### Key Design Decisions

1. **Estimation is fine for MCP tools**: The MCP server doesn't see actual LLM token counts (those come from the sampling layer above). The 4-chars/token heuristic is reasonable for tool I/O costing. Provide a hook for actual counts when available.

2. **Middleware is the right integration point**: mcpkit's middleware pattern naturally intercepts every tool call. The finops middleware already sits here correctly.

3. **Context-based scoping**: Use `context.Context` to carry budget scope (session, user, agent). This aligns with mcpkit's existing patterns (auth identity in context, sampling client in context).

4. **Separate budget from tracking**: The Tracker records what happened. The Budget decides what's allowed. Keep them decoupled so tracking works even without budget enforcement.

---

## Key Takeaways

1. **The 90% rule**: Anthropic's prompt caching delivers 90% cost reduction on cached content. For agent loops, this is the single highest-impact optimization.

2. **Model routing saves 50-85%**: RouteLLM demonstrates that routing simple queries to cheap models preserves 95% quality while cutting costs dramatically.

3. **Hierarchical budgets are table stakes**: LiteLLM's per-key/user/team budget system is the industry standard. Production systems need multi-level enforcement.

4. **OTel GenAI is immature but directionally right**: The semantic conventions are in development status. Adopt the metric names now for future compatibility, but expect breaking changes.

5. **Self-hosting rarely wins for agents**: Agent systems need frontier model quality for reasoning. Self-hosted models are cost-effective only at high volume with relaxed quality requirements.

6. **Cost attribution in multi-agent is unsolved**: No standard approach exists. Context propagation (mcpkit's strength) is the most practical pattern.

7. **mcpkit's finops foundation is solid**: The Tracker/Middleware/Estimation pattern is correct. The main gaps are dollar-based budgets, scoped budgets, duration resets, and Prometheus export.
