# AI Gateway Research: Patterns for LLM/Agent Systems

Research date: 2026-03-14. Covers developments from 2024-2026.

---

## 1. AI Gateways

AI gateways sit between application code and LLM providers, providing a unified API surface with cross-cutting concerns (routing, caching, rate limiting, observability, failover). They have become essential infrastructure as production AI teams now commonly run 5+ LLMs simultaneously.

### LiteLLM

- **Type:** Open-source (enterprise features paid), Python SDK + proxy server
- **Providers:** 100+ LLM APIs via unified OpenAI-compatible interface
- **Key features:** Cost tracking, budget enforcement per virtual key/user, load balancing, retry/fallback logic, guardrails, RBAC, GUI admin
- **Architecture:** Python proxy server, YAML-configured routing
- **Scalability concerns:** At ~2,000 RPS, memory usage climbs past 8GB and requests start timing out
- **Enterprise:** SSO, RBAC, team-level budget enforcement locked behind paid license
- **Setup time:** 15-30 minutes with YAML configuration

### Portkey

- **Type:** Commercial ($49/mo+), with open-source gateway component
- **Providers:** 200+ LLM providers, 50+ AI guardrails
- **Key features:** Dynamic routing based on metadata/context/region/user tier, canary deployments, semantic caching, guardrail-based routing (deny/retry/switch model), prompt management
- **Compliance:** SOC2, HIPAA, GDPR built-in
- **Differentiator:** Strongest guardrails integration -- 60+ guardrails that can trigger routing decisions in real-time

### Helicone

- **Type:** Open-source, Rust-based
- **Performance:** 8ms P50 latency, ~1-5ms P95 overhead, horizontally scalable
- **Key features:** Smart load balancing (P2C with PeakEWMA), semantic caching, automatic failover, rate limiting, built-in observability
- **Integration:** Single base URL change to integrate
- **Cost savings:** 20-30% API cost reduction through caching
- **Deployment:** Docker, Kubernetes, or cloud-hosted

### Kong AI Gateway

- **Type:** Commercial (open-source core), extension of Kong API Gateway
- **Architecture:** Lua plugin system on top of existing API gateway infrastructure
- **Key features:** AI Proxy plugin (unified request/response format), AI Rate Limiting Advanced (token-based, enterprise), RAG pipeline automation, PII sanitization (12 languages)
- **MCP support (v3.12, Oct 2025):** AI MCP Proxy plugin (protocol bridge MCP to HTTP), AI MCP OAuth2 plugin
- **Differentiator:** Teams already using Kong for API management can extend to AI without new infrastructure

### Cloudflare AI Gateway

- **Type:** Free tier (core features), part of Cloudflare developer platform
- **Architecture:** Edge-deployed across 330+ cities globally
- **Key features:** Response caching (up to 90% latency reduction), rate limiting (fixed or sliding window), analytics dashboard
- **Rate limiting:** Request-count based (not token-based), configurable per-gateway
- **Caching:** Exact-match currently; semantic caching planned but not yet shipped
- **Limitation:** Less sophisticated than dedicated AI gateways -- no multi-provider failover, no guardrails, no token-based rate limiting
- **Strength:** Zero-cost, edge-native, integrates with Workers AI and Vectorize

---

## 2. Model Routing

Intelligent routing selects the optimal model per-request based on complexity, cost, and latency requirements. This can reduce costs 85%+ while maintaining 95% of top-tier model performance.

### RouteLLM (UC Berkeley / ICLR 2025)

- **Approach:** Learns routing decisions from human preference data (e.g., Chatbot Arena)
- **Methods:**
  - Preference-based routing: trains on human evaluations of model quality
  - Similarity-based ranking: matches query patterns to known model strengths
  - Threshold-based: assigns simpler queries to faster/cheaper models
- **Results:** 85% cost reduction on MT Bench, 45% on MMLU, 35% on GSM8K vs GPT-4-only, while retaining 95% of GPT-4 performance
- **Open source:** Yes, published at ICLR 2025

### Not Diamond

- **Approach:** Commercial routing service, trains custom routers on evaluation datasets
- **Claims:** Up to 25% accuracy improvement over any single model, up to 10x cost reduction
- **Custom routing:** Users can include their own fine-tuned models or agentic workflows in the routing pool
- **Architecture:** Cloud-hosted router that evaluates each query before dispatch

### Martian

- **Approach:** Real-time dynamic routing
- **Claims:** Up to 98% cost savings
- **Research:** Partnered with Apart Research on mechanistic router interpretability (understanding why routers make specific decisions)

### Arch-Router (2025)

- **Approach:** 1.5B parameter routing model that runs locally
- **Performance:** ~28x lower end-to-end latency than closest commercial competitor while matching or exceeding routing accuracy
- **Deployment:** Runs on commodity GPUs in tens of milliseconds

### Routing Decision Factors

| Factor | Description |
|--------|-------------|
| Query complexity | Estimated reasoning depth required |
| Token budget | Cost ceiling for the request |
| Latency requirement | Real-time vs batch tolerance |
| Task type | Classification, generation, code, math, etc. |
| Model strengths | Known per-model benchmarks for task categories |
| Historical performance | Learned from preference data or A/B testing |

---

## 3. MCP Gateway Patterns

### Industry Definition

An MCP gateway is a session-aware reverse proxy and lightweight control plane that fronts many MCP servers behind one endpoint. Core responsibilities:
- Aggregating tools/resources/prompts from multiple backend MCP servers
- Centralized authn/authz, policy enforcement, observability
- Namespace routing (single `tools/list` returns merged catalog)
- Lifecycle management of upstream connections

### Current mcpkit Gateway

The existing `gateway/` package implements the aggregation proxy pattern:
- Connects to upstream MCP servers via streamable HTTP
- Discovers tools via `ListTools`, registers them with namespaced names (`upstream.toolname`)
- Health checking with configurable interval and unhealthy threshold
- Proxy handler forwards `CallToolRequest` to the correct upstream
- Dynamic add/remove of upstreams at runtime
- Middleware chain applied to all proxied handlers

### Industry MCP Gateway Implementations (2025-2026)

**Kong AI MCP Gateway (v3.12+):**
- MCP-to-HTTP protocol bridge via AI MCP Proxy plugin
- OAuth 2.0 for MCP servers via AI MCP OAuth2 plugin
- Reuses existing Kong plugin ecosystem (rate limiting, logging, transforms)

**Envoy AI Gateway:**
- CNCF project (Tetrate + Bloomberg), built on Envoy proxy
- Two-tier architecture: Tier 1 (centralized entry, global rate limiting) + Tier 2 (fine-grained self-hosted model access)
- MCPRoute custom resource for Kubernetes-native MCP routing
- Streamable HTTP transport, JSON-RPC 2.0, MCP-spec-compliant OAuth 2.0 with JWKS
- Standalone mode via `--mcp-config` flag (no Kubernetes required)

**Solo.io Agent Gateway:**
- Open-source, supports both agent-to-agent and agent-to-tool communication
- Multiple backend types (MCP servers, other agents) under one endpoint
- Production-ready data plane for agentic AI

### Evolution Opportunities for mcpkit Gateway

| Capability | Current State | Potential Enhancement |
|------------|---------------|----------------------|
| Routing | Static namespace prefix | Weighted routing, content-based routing, priority queues |
| Load balancing | None (single upstream per namespace) | Round-robin, least-connections, P2C across replicas |
| Failover | Health check marks unhealthy, returns error | Automatic failover to replica, retry with different upstream |
| Caching | None | Response caching for idempotent tool calls |
| Rate limiting | None (delegated to middleware) | Token-aware rate limiting per upstream/tool |
| Auth | None (delegated to middleware) | OAuth 2.0 proxy, token forwarding to upstreams |
| Observability | None | Per-upstream metrics, latency tracking, error rates |
| Protocol | Streamable HTTP only | SSE fallback, stdio bridge for local servers |
| Discovery | Manual AddUpstream | MCP Registry integration for auto-discovery |

---

## 4. API Management for AI

Traditional API rate limiting (requests/second) is insufficient for LLM APIs because a single request can consume vastly different resources (200-token prompt vs 20,000-token prompt).

### Token-Based Rate Limiting

- **Azure API Management:** `llm-token-limit` policy limits token consumption per key over configurable time periods
- **Google Apigee:** LLM token quota policies at the API product level
- **Kong:** AI Rate Limiting Advanced plugin (enterprise) limits by tokens
- **Envoy AI Gateway:** Usage-based rate limiting that counts tokens not requests

### Quota Management Dimensions

| Dimension | Description |
|-----------|-------------|
| Per-user | Individual developer or end-user quotas |
| Per-team | Organizational unit budgets |
| Per-application | Service-level consumption limits |
| Per-model | Limits on expensive model usage |
| Per-environment | Dev/staging/prod differentiation |
| Time-based | Per-minute, per-hour, per-day, per-month windows |

### Billing Integration

- Enrich rate limiting with JWT claims (user_id, team, app, env) for cost attribution
- Token-based metering provides accurate resource-proportional billing
- Real-time dashboards for finance/ops visibility into AI spend

### Relevance to mcpkit

The `finops/` package already handles token accounting and budget policies. A gateway-level rate limiter could:
- Apply token budgets per upstream (prevent one chatty upstream from consuming all quota)
- Enforce per-tool rate limits based on estimated token cost
- Feed usage data to finops for cross-cutting budget enforcement

---

## 5. Caching Layers

### Exact-Match Caching

- Hash the full request (model + messages + parameters) as cache key
- Cache hits return in <1ms vs 2-5 seconds for LLM inference
- Simple to implement, high confidence in correctness
- Low hit rate for conversational workloads (unique prompts)
- Best for: template-based queries, repeated tool calls, deterministic operations

### Semantic Caching

- Convert prompts to vector embeddings, search cache by cosine similarity
- Hit when similarity exceeds threshold (typically 0.8)
- Cache hits return in ~5ms vs 2-5 seconds for inference
- 30-40% cache hit rate translates to meaningful cost savings

**Technical components:**
- Embedding model: GPTCache uses `GPTCache/albert-duplicate-onnx` (ONNX); production systems typically use OpenAI `text-embedding-3-small` or similar
- Vector database: Milvus, FAISS, PGVector, Chroma, Zilliz Cloud
- Similarity measurement: Cosine similarity (primary), BM25, L2 distance
- Threshold tuning: Balance between cache hits and false positives (too low = wrong answers, too high = low hit rate)

**Cache invalidation strategies:**
- TTL policies (time-based expiration)
- Context-aware invalidation (when underlying data changes)
- Environment-specific rules (prod cache separate from staging)
- Manual purge for known stale entries

**Gateway-level implementation advantages:**
- Shared cache across teams and services
- Centralized threshold and TTL control
- Unified observability linking cache hits to cost savings
- Model-agnostic (works across self-hosted, fine-tuned, or external models)

### Prompt Cache Warming

- Pre-populate caches with common query patterns
- Anthropic and OpenAI both support provider-side prompt caching (system prompt reuse across requests)
- Gateway-level warming distinct from provider-level caching

### GPTCache (Zilliz)

- Open-source semantic cache, integrates with LangChain and LlamaIndex
- Modular architecture: pluggable embedding models, vector stores, similarity evaluators
- Supports FAISS, Milvus, PGVector, Chroma as vector backends
- Default similarity threshold: 0.8
- Active research on adversarial resilience (Nature, 2026) -- preventing cache poisoning attacks

---

## 6. Observability

### Three Signal Types

| Signal | Description | Examples |
|--------|-------------|----------|
| Traces | Request path across prompts, retrievals, tools, guardrails | End-to-end latency, tool chain visualization |
| Metrics | Aggregated performance, cost, and quality measures | Token throughput, p50/p99 latency, cost per request |
| Events | Safety or governance alerts requiring review | PII detection, guardrail violations, budget exhaustion |

### Key Metrics for AI Systems

| Category | Metrics |
|----------|---------|
| Performance | Latency (p50, p95, p99), tokens/second, time-to-first-token |
| Cost | Cost per request, cost per user, cost per tool chain, daily/monthly spend |
| Quality | Response quality scores, hallucination rate, guardrail pass rate |
| Reliability | Error rate, timeout rate, provider availability, cache hit rate |
| Usage | Request volume, token consumption (input vs output), model distribution |

### Leading Observability Platforms (2025-2026)

**Gateway-integrated:**
- Portkey: Observability native within AI Gateway, traces across providers/users/workspaces
- Helicone: Open-source, every request logged with cost/latency, custom dashboards
- Cloudflare AI Gateway: Free analytics dashboard with request logs

**Standalone platforms:**
- LangSmith: Millions of traces/day, cost and latency attribution at step level
- Langfuse: Open-source, cost attribution at generation/embedding level per span
- Datadog LLM Observability: End-to-end tracing across AI agents with input/output/latency/token/error visibility
- OpenLLMetry (Traceloop): OTLP-compatible spans, integrates with existing observability stacks
- Braintrust: Zero-config LangChain integration, framework-aware cost tracking

### Cost Attribution

- Per-span token count and cost figures within traces
- Identify costliest requests in complex multi-tool workflows
- Break down by user, team, application, environment via JWT claims
- Real-time dashboards for engineering and finance

### Relevance to mcpkit

The `observability/` package provides OpenTelemetry middleware. A gateway enhancement could:
- Emit per-upstream spans with tool name, latency, token count
- Aggregate cost attribution across upstream calls within a single request
- Surface health metrics (upstream availability, error rates) as Prometheus gauges

---

## 7. Failover and Resilience

### Three-Layer Failure Handling

**Layer 1: Retries**
- Handle transient, self-resolving failures
- Exponential backoff with jitter
- Respect `Retry-After` headers from providers
- Risk: retry storms at scale if failure is persistent

**Layer 2: Fallbacks**
- Route to secondary provider/model when primary fails definitively
- Example: GPT-4o returns 503 -> retry on Claude 3.5 Sonnet
- Limitation: reactive -- checks primary every time before falling back, adding latency
- Risk: fallback may share failure domain with primary (e.g., same cloud region)

**Layer 3: Circuit Breakers**
- Monitor failure patterns proactively
- Triggers: failure count, failure rate over time, specific HTTP codes (429, 502, 503)
- When threshold crossed: remove provider from routing pool for cooldown period
- Prevents cascade failures and retry storms
- Enables preemptive fallback activation

### Production Patterns

| Pattern | When to Use |
|---------|-------------|
| Simple retry | Transient network errors, rate limit with known reset |
| Multi-tier fallback chain | Provider outage, need guaranteed availability |
| Circuit breaker + fallback | High-traffic systems, multiple provider options |
| Health-aware routing | Load balancing across provider replicas |
| Model degradation | Accept lower quality model when primary unavailable |

### Leading Implementations

- **Bifrost (Maxim AI):** 11 microsecond overhead at 5K RPS, multi-tier fallback chains, cluster mode resilience, health-aware routing with circuit breaking
- **Portkey:** Configurable retry/fallback routing logic across providers, budget and rate limit integration
- **Helicone:** Health-aware routing with circuit breaking, P2C load balancing
- **LiteLLM:** Retry with exponential backoff, fallback routing across deployments

### Relevance to mcpkit

The `resilience/` package already provides CircuitBreaker and RateLimiter. The gateway could:
- Apply circuit breakers per-upstream (already has health checking with threshold)
- Add fallback routing: when upstream A is unhealthy, route to upstream B (same tool namespace, different server)
- Implement retry with backoff before marking unhealthy
- Support replica groups: multiple URLs for the same upstream namespace

---

## 8. Edge Deployment

### Cloudflare Workers + AI Gateway

- 330+ cities globally, within 50ms of 95% of world population
- Workers AI: serverless inference on Cloudflare's GPU network across 190+ cities
- AI Gateway: free caching, rate limiting, analytics at the edge
- Containers coming to Workers (beta mid-2025): any language in containers, Workers as API gateways/orchestrators
- Sub-50ms global response times correlate with 27% higher engagement (Q4 2025 industry data)

### Deno Deploy

- JavaScript/TypeScript edge runtime
- Lacks dedicated GPU infrastructure and AI-specific services compared to Cloudflare
- Better suited for lightweight proxy/routing logic than inference

### Edge AI Gateway Architecture

```
Client -> Edge Gateway (auth, cache check, rate limit) -> Origin Gateway (routing, fallback) -> LLM Provider
```

Benefits:
- Authentication and rate limiting at the edge (reject bad requests early)
- Cache hits served from nearest edge node (sub-5ms)
- Geographic routing to nearest LLM provider region
- Request/response logging at the edge

Limitations:
- Stateful operations (circuit breaker state, token budgets) harder at the edge
- Semantic caching requires vector DB access (latency tradeoff)
- Complex routing logic better suited for origin gateway

### Relevance to mcpkit

The gateway package is a Go library (not an edge runtime), but could:
- Expose an HTTP handler compatible with edge deployment via WASM compilation
- Implement a two-tier architecture: edge tier (cache, auth, rate limit) + origin tier (routing, failover, upstream management)
- Cache tool responses at a CDN layer for idempotent operations

---

## 9. Commercial vs Open-Source Comparison

| Feature | LiteLLM (OSS) | Helicone (OSS) | Portkey (Commercial) | Kong AI (Commercial) | Cloudflare (Free) |
|---------|---------------|----------------|---------------------|---------------------|-------------------|
| Providers | 100+ | 100+ | 200+ | Major providers | Major providers |
| Language | Python | Rust | TypeScript | Lua/Go | Edge JS |
| Latency overhead | 50-200ms | 1-5ms (P95) | ~50ms | Varies | Edge-native |
| Semantic caching | Yes | Yes | Yes | Yes (plugin) | Planned |
| Token rate limiting | Yes | Custom | Yes | Enterprise only | No (request-based) |
| Guardrails | Via plugins | No | 60+ built-in | PII sanitization | No |
| Model routing | Basic | P2C load balancing | Dynamic + canary | Plugin-based | No |
| Circuit breaking | Yes | Yes | Yes | Yes | No |
| MCP support | No | No | No | Yes (v3.12) | No |
| Self-host | Yes | Yes | Yes | Yes | No |
| RBAC | Enterprise only | No | Yes | Yes | No |
| Cost tracking | Yes | Yes | Yes | Via plugins | Basic |
| Setup complexity | Medium | Low | Low | High (existing Kong) | Very low |
| Best for | Max flexibility | Low-latency proxy | Enterprise + compliance | API-first orgs | Simple caching/logging |

### Open-Source Advantages
- Full control over data residency and compliance
- No vendor lock-in for critical AI infrastructure
- Community-driven feature development
- Self-hosting in regulated environments

### Commercial Advantages
- Faster time to production with managed services
- Built-in compliance certifications (SOC2, HIPAA, GDPR)
- Enterprise support and SLAs
- Advanced features (guardrails, prompt management) out of the box

---

## 10. Key Takeaways for mcpkit

### What the gateway package already does well
- Namespace-based tool aggregation (core MCP gateway pattern)
- Health checking with configurable thresholds
- Dynamic upstream add/remove
- Middleware chain for cross-cutting concerns

### High-value enhancements (ordered by impact)

1. **Upstream replica groups with failover:** Allow multiple URLs per namespace, automatic failover when one is unhealthy. This is the most requested pattern in production MCP deployments.

2. **Response caching for idempotent tools:** Exact-match cache keyed on tool name + arguments hash. Low complexity, immediate cost/latency benefit.

3. **Per-upstream observability:** Emit OpenTelemetry spans per proxied call with upstream name, tool name, latency, success/failure. Integrate with existing `observability/` package.

4. **Token-aware rate limiting:** Leverage `finops/` package to apply per-upstream and per-tool token budgets at the gateway level.

5. **Load balancing across replicas:** P2C or round-robin when multiple instances serve the same upstream namespace.

6. **Circuit breaker integration:** Use `resilience/` CircuitBreaker per upstream instead of simple health-check threshold. Provides faster failure detection and automatic recovery.

7. **Semantic tool routing:** Route to different upstreams based on tool call content, not just namespace prefix. Enables cost-optimized routing (simple queries to cheaper upstreams).

8. **MCP Registry auto-discovery:** Integrate with `discovery/` package to automatically discover and connect to upstream servers.

---

## Sources

- [LiteLLM AI Gateway](https://docs.litellm.ai/docs/simple_proxy)
- [LiteLLM GitHub](https://github.com/BerriAI/litellm)
- [Portkey AI Gateway](https://portkey.ai/features/ai-gateway)
- [Portkey Guardrails](https://portkey.ai/features/guardrails)
- [Helicone AI Gateway](https://www.helicone.ai/)
- [Helicone GitHub](https://github.com/Helicone/ai-gateway)
- [Kong AI Gateway Docs](https://developer.konghq.com/ai-gateway/)
- [Kong AI Gateway 3.10](https://konghq.com/blog/product-releases/ai-gateway-3-10)
- [Kong AI Gateway 3.11](https://konghq.com/blog/product-releases/ai-gateway-3-11)
- [Kong MCP Gateway Technical Breakdown](https://medium.com/@claudioacquaviva/kong-ai-mcp-gateway-and-kong-mcp-server-technical-breakdown-13420f610ee6)
- [Cloudflare AI Gateway Docs](https://developers.cloudflare.com/ai-gateway/)
- [Cloudflare AI Gateway Caching](https://developers.cloudflare.com/ai-gateway/features/caching/)
- [RouteLLM Paper (ICLR 2025)](https://arxiv.org/abs/2406.18665)
- [RouteLLM Blog (LMSYS)](https://lmsys.org/blog/2024-07-01-routellm/)
- [Not Diamond](https://www.toolify.ai/tool/not-diamond)
- [Awesome AI Model Routing](https://github.com/Not-Diamond/awesome-ai-model-routing)
- [Semantic Caching for LLMs (TrueFoundry)](https://www.truefoundry.com/blog/semantic-caching)
- [Semantic Caching Gateway Guide (Maxim)](https://www.getmaxim.ai/articles/semantic-caching-for-llms-how-to-cut-token-spend-with-ai-gateways/)
- [GPTCache GitHub](https://github.com/zilliztech/GPTCache)
- [MCP Gateway Comparison (Moesif)](https://www.moesif.com/blog/monitoring/model-context-protocol/Comparing-MCP-Model-Context-Protocol-Gateways/)
- [Envoy AI Gateway](https://aigateway.envoyproxy.io/)
- [Envoy AI Gateway MCP Support](https://aigateway.envoyproxy.io/blog/mcp-implementation/)
- [Agent Gateway Rate Limiting](https://agentgateway.dev/blog/2025-11-02-rate-limit-quota-llm/)
- [Portkey Failover Strategies](https://portkey.ai/blog/failover-routing-strategies-for-llms-in-production/)
- [Portkey Retries/Fallbacks/Circuit Breakers](https://portkey.ai/blog/retries-fallbacks-and-circuit-breakers-in-llm-apps/)
- [LLM Observability Guide (Portkey)](https://portkey.ai/blog/the-complete-guide-to-llm-observability/)
- [Top LLM Observability Tools (Braintrust)](https://www.braintrust.dev/articles/top-10-llm-observability-tools-2025)
- [Token Rate Limiting (Azure)](https://learn.microsoft.com/en-us/azure/api-management/llm-token-limit-policy)
- [Best MCP Gateways 2026 (TrueFoundry)](https://www.truefoundry.com/blog/best-mcp-gateways)
- [Top LLM Gateways 2025 (Helicone)](https://www.helicone.ai/blog/top-llm-gateways-comparison-2025)
