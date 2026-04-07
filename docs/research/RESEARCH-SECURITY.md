# AI Agent Security: Threat Models and Defense Patterns

Research compiled March 2026. Covers academic papers, OWASP frameworks, vendor security research, and real-world incidents from 2024-2026.

---

## Table of Contents

1. [Agent Threat Model](#1-agent-threat-model)
2. [Permission Models](#2-permission-models)
3. [Sandboxing and Isolation](#3-sandboxing-and-isolation)
4. [MCP Security](#4-mcp-security)
5. [Input Sanitization](#5-input-sanitization)
6. [Audit and Compliance](#6-audit-and-compliance)
7. [Multi-Tenant Security](#7-multi-tenant-security)
8. [Workload Identity](#8-workload-identity)
9. [Implications for mcpkit](#9-implications-for-mcpkit)

---

## 1. Agent Threat Model

### 1.1 OWASP Frameworks

Two OWASP frameworks define the threat landscape:

**OWASP Top 10 for LLM Applications 2025** — Focuses on model-level vulnerabilities:
1. Prompt Injection (direct and indirect)
2. Sensitive Information Disclosure
3. Supply Chain Vulnerabilities
4. Data and Model Poisoning
5. Improper Output Handling
6. Excessive Agency
7. System Prompt Leakage
8. Vector and Embedding Weaknesses (new in 2025)
9. Misinformation
10. Unbounded Consumption

**OWASP Top 10 for Agentic Applications 2026** (ASI01-ASI10) — Focuses on autonomous agent risks:
1. **Agent Goal Hijack** — Attacker alters agent objectives through malicious content
2. **Rogue Agents** — Compromised agents that self-replicate, persist, or impersonate
3. **Tool Misuse** — Agents manipulated into invoking tools with malicious arguments
4. **Identity Delegation Failures** — Broken chains of trust in multi-agent delegation
5. **Memory Poisoning** — Corrupting persistent agent memory/context
6. **Cascading Failures** — Error propagation across agent chains
7. **Human-Agent Trust Exploitation** — Abusing approval fatigue and trust patterns
8. **Excessive Agency** — Agents granted more capabilities than needed
9. **Supply Chain Compromise** — Malicious tools, plugins, or MCP servers
10. **Rogue Agents** — Agents that deviate from intended behavior

Key principle introduced: **Least-Agency** — an extension of least privilege where agents should only be granted the minimum level of autonomy required to complete their defined task.

Sources:
- [OWASP Top 10 for LLM Applications 2025](https://owasp.org/www-project-top-10-for-large-language-model-applications/)
- [OWASP Top 10 for Agentic Applications 2026](https://genai.owasp.org/resource/owasp-top-10-for-agentic-applications-for-2026/)
- [OWASP AI Agent Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/AI_Agent_Security_Cheat_Sheet.html)

### 1.2 Prompt Injection

The most persistent and dangerous attack vector. Two forms:

**Direct Prompt Injection**: Attacker provides malicious instructions directly in user input. Model interprets instructions as legitimate commands.

**Indirect Prompt Injection (IPI)**: Malicious instructions embedded in external content the agent processes — web pages, emails, calendar entries, database records, API responses. The agent treats poisoned content as trusted context.

Key research findings:
- A comprehensive review (2023-2025) analyzing 45 sources found that **no existing defense is robust** against adaptive attacks. Researchers bypassed 12 published defenses with >90% attack success rate by systematically tuning optimization techniques.
- RAG poisoning demonstrated against ChatGPT in May 2024 via "watering hole" pattern — compromising resources targets naturally visit.
- GitHub Copilot CVE-2025-53773 enabled RCE affecting millions of developers.
- Anthropic's September 2025 espionage incident: Chinese state-sponsored attackers jailbroke Claude Code, running 80-90% of the operation autonomously.

**Current state**: Prompt injection remains unsolved. Model-level guardrails are "architectural suggestions, not enforcement mechanisms" — they operate through learned behavioral patterns vulnerable to adversarial optimization.

Sources:
- [Comprehensive Review of Prompt Injection (MDPI)](https://www.mdpi.com/2078-2489/17/1/54)
- [From prompt injections to protocol exploits (ScienceDirect)](https://www.sciencedirect.com/science/article/pii/S2405959525001997)
- [Simon Willison: Agents Rule of Two](https://simonwillison.net/2025/Nov/2/new-prompt-injection-papers/)
- [Anthropic AI Espionage Disclosure](https://securiti.ai/blog/anthropic-exploit-era-of-ai-agent-attacks/)

### 1.3 Confused Deputy Attacks

The confused deputy problem is particularly acute for AI agents. An agent with legitimate privileges is tricked into performing actions on behalf of an attacker.

**Real-world incident (2024)**: A financial services reconciliation agent was tricked into exporting "all customer records matching pattern X" where X was a regex matching every record. The agent found the request reasonable because it was phrased as a business task. Result: 45,000 customer records exfiltrated.

Attack progression for agent escape follows five stages:
1. Malicious input
2. Tool misuse and reconnaissance
3. Privilege escalation inside container
4. Container breakout to host/control plane
5. Persistence and data exfiltration

Tool misuse and privilege escalation were the most common threats in 2025 (520 documented incidents).

Sources:
- [BeyondTrust: Confused Deputy Problem](https://www.beyondtrust.com/blog/entry/confused-deputy-problem)
- [Unit42: Agentic AI Threats](https://unit42.paloaltonetworks.com/agentic-ai-threats/)
- [ARMO: AI Agent Escape Detection](https://www.armosec.io/blog/ai-agent-escape-detection/)

### 1.4 Supply Chain Risks in Tool Registries

MCP tool registries introduce npm-style supply chain risks:

**Postmark-MCP Backdoor (September 2025)**: Attackers modified version 1.0.16 of the `postmark-mcp` npm package (~1,500 weekly downloads), adding a single line that BCC'd every outgoing email to an attacker-controlled domain. Sensitive communications exfiltrated for days before detection.

**Smithery Registry Compromise (October 2025)**: Path-traversal bug in `smithery.yaml` build configuration allowed exfiltration of API tokens and credentials from 3,000+ hosted MCP applications.

**CVE-2025-6514 (July 2025)**: Critical OS command-injection in `mcp-remote` (CVSS 9.6), a widely-used OAuth proxy with 437,000+ downloads, featured in official guides from Cloudflare, Hugging Face, and Auth0.

Sources:
- [Invariant Labs: MCP Tool Poisoning](https://invariantlabs.ai/blog/mcp-security-notification-tool-poisoning-attacks)
- [OWASP MCP03:2025 Tool Poisoning](https://owasp.org/www-project-mcp-top-10/2025/MCP03-2025%E2%80%93Tool-Poisoning)
- [Elastic: MCP Tools Attack Vectors](https://www.elastic.co/security-labs/mcp-tools-attack-defense-recommendations)
- [Data Science Dojo: State of MCP Security 2025](https://datasciencedojo.com/blog/mcp-security-risks-and-challenges/)

---

## 2. Permission Models

### 2.1 Agency vs. Autonomy

AWS and OWASP distinguish two dimensions:
- **Agency**: The scope of actions permitted — what systems it can interact with, what operations it can perform, what resources it can modify. Agency is about capabilities and permissions.
- **Autonomy**: The degree of independent decision-making — when it operates, how it chooses between actions, whether it requires human approval. Autonomy is about independence.

AWS Security Scoping Matrix defines four scope levels:
- **Scope 1 (No Agency)**: Pure information retrieval, no tool execution
- **Scope 2 (Prescribed Agency)**: Human approval required for all actions
- **Scope 3 (Supervised Agency)**: Autonomous execution after human initiation, optional guidance
- **Scope 4 (Full Agency)**: Self-initiating systems with strategic human oversight only

### 2.2 Human-in-the-Loop Patterns

Three implementation patterns for gating agent actions:

**Synchronous approval**: Agent pauses execution and waits for human approval before tool invocation. MCP servers can implement approval workflows as tools the LLM calls but only execute after human approval via dashboard controls.

**Asynchronous authorization**: Agent requests authorization and continues work while waiting. Decouples the authorization request from actual action execution.

**Tiered approval**: Read operations execute freely; write operations require approval; destructive operations require multi-party approval. Organizations configure granular tool access by role — e.g., enabling read-only database operations while excluding write tools entirely.

### 2.3 Capability-Based Security

IronClaw's WASM sandboxing approach: isolated WebAssembly sandboxes with zero default access. To read a file, a skill must hold a `FileRead` capability token specifying exactly which paths it can access. Overhead: ~15ms per skill invocation.

CaMel framework: enforces separation at a deeper level, preventing data from untrusted sources from being used as arguments in dangerous function calls. Blocks the model from treating external content as executable instructions.

### 2.4 OAuth Scopes for Tool Access

MCP spec requires OAuth 2.1 with scope-based authorization. Protected MCP servers act as OAuth 2.1 resource servers accepting access tokens. Resource Indicators (RFC 8707) ensure tokens are tightly scoped to specific servers.

Sources:
- [AWS: Agentic AI Security Scoping Matrix](https://aws.amazon.com/blogs/security/the-agentic-ai-security-scoping-matrix-a-framework-for-securing-autonomous-ai-systems/)
- [Auth0: Human-in-the-Loop for AI Agents](https://auth0.com/blog/secure-human-in-the-loop-interactions-for-ai-agents/)
- [Permit.io: Human-in-the-Loop Best Practices](https://www.permit.io/blog/human-in-the-loop-for-ai-agents-best-practices-frameworks-use-cases-and-demo)
- [IBM: AI Agent Security Best Practices](https://www.ibm.com/think/tutorials/ai-agent-security)

---

## 3. Sandboxing and Isolation

### 3.1 Anthropic's Approach

Anthropic's `sandbox-runtime` provides OS-level isolation without containers:
- **Filesystem isolation**: Agent can only access or modify specific directories
- **Network isolation**: Agent can only connect to approved hosts
- **Implementation**: Linux bubblewrap, macOS seatbelt (OS-level primitives)
- **No container overhead**: Lighter weight than Docker-based isolation

Claude Code's sandboxing uses these primitives to enforce restrictions at the kernel level.

### 3.2 Container Isolation (NanoClaw Pattern)

NanoClaw implements ephemeral container isolation:
- Every agent session runs inside an ephemeral Docker container
- Container spins up, processes the message, returns result, self-destructs
- Zero persistence between invocations
- Network policies restrict egress to approved endpoints

### 3.3 WASM Sandboxing (IronClaw Pattern)

IronClaw uses zero-trust WebAssembly sandboxes:
- Zero default access — capabilities must be explicitly granted
- Capability tokens specify exact resource paths and operations
- ~15ms overhead per skill invocation
- Memory isolation between tools prevents cross-tool contamination

### 3.4 Industry Adoption

As of 2025, sandboxing or VM isolation is documented for only 9 of 30 surveyed agent systems, primarily developer/CLI tools and browser agents. Comprehensive sandboxing adoption remains limited.

### 3.5 nono: Capability-Based CLI Sandbox

Open-source `nono` project provides:
- Kernel-enforced sandbox for AI agents
- Capability-based isolation with secure key management
- Atomic rollback for filesystem changes
- Cryptographic immutable audit chain of provenance
- Zero-trust environment for agent execution

Sources:
- [Anthropic: Claude Code Sandboxing](https://www.anthropic.com/engineering/claude-code-sandboxing)
- [Anthropic sandbox-runtime (GitHub)](https://github.com/anthropic-experimental/sandbox-runtime)
- [nono sandbox (GitHub)](https://github.com/always-further/nono)
- [2025 AI Agent Index](https://arxiv.org/html/2602.17753)
- [Northflank: Best Code Execution Sandbox 2026](https://northflank.com/blog/best-code-execution-sandbox-for-ai-agents)

---

## 4. MCP Security

### 4.1 OAuth 2.1 Authorization

The MCP spec (2025-11-25) defines authorization using OAuth 2.1:
- Protected MCP servers act as OAuth 2.1 resource servers
- MCP clients must implement Resource Indicators (RFC 8707) to prevent token mis-redemption
- Authorization servers issue tokens tightly scoped and valid only for specific MCP servers
- Prevents malicious/compromised servers from reusing tokens against other resources

### 4.2 DPoP Token Binding

Sender-constrained tokens address bearer token theft:
- DPoP (Demonstration of Proof-of-Possession) binds tokens to cryptographic keys
- Requires both token and key for access — stolen token alone is insufficient
- Combined with short-lived access tokens that expire after task completion
- mTLS as alternative sender-constraining mechanism

### 4.3 Tool Poisoning (MCP03:2025)

OWASP MCP Top 10 identifies tool poisoning as a critical risk:

**Rug Pull Attack**: Tool description or behavior silently altered after user approval. Attacker establishes trust, then injects hidden instructions to steer behavior, exfiltrate data, or trigger unauthorized actions.

**Tool Shadowing**: Malicious MCP server injects a tool description that modifies agent behavior with respect to a trusted service. Combined with rug pull, a malicious server can hijack an agent without appearing in user-facing interaction logs.

### 4.4 Known Security Gaps

1. **Authorization spec conflicts**: Community identified that current spec includes implementation details conflicting with modern enterprise practices (SAML integration, existing IdP infrastructure)
2. **No tool description integrity**: No cryptographic signing of tool descriptions — descriptions can be modified between approval and invocation
3. **No server identity verification standard**: Beyond TLS, no mechanism to verify MCP server identity or authenticity
4. **Session fixation risks**: OAuth flows in MCP can be vulnerable to session fixation if state parameters aren't properly validated
5. **MCP-related vulnerabilities surged 270% in Q3 2025**

### 4.5 MCP Security Timeline

- **March 2025**: First tool poisoning PoCs published
- **June 2025**: Spec update adds Resource Indicators requirement
- **July 2025**: CVE-2025-6514 (mcp-remote RCE, CVSS 9.6)
- **September 2025**: postmark-mcp supply chain compromise
- **October 2025**: Smithery registry compromise (3,000+ apps)
- **Q3 2025**: 270% surge in MCP-related vulnerabilities

Sources:
- [MCP Authorization Spec](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization)
- [Auth0: MCP Spec Updates June 2025](https://auth0.com/blog/mcp-specs-update-all-about-auth/)
- [AuthZed: Timeline of MCP Breaches](https://authzed.com/blog/timeline-mcp-breaches)
- [Red Hat: MCP Security Risks and Controls](https://www.redhat.com/en/blog/model-context-protocol-mcp-understanding-security-risks-and-controls)
- [Descope: Top 6 MCP Vulnerabilities](https://www.descope.com/blog/post/mcp-vulnerabilities)
- [Breaking the Protocol (arXiv)](https://arxiv.org/html/2601.17549)

---

## 5. Input Sanitization

### 5.1 Defense-in-Depth Architecture

Five-layer defense model (OWASP/industry consensus):

1. **Permission Boundaries**: Strict capability limits on what tools can access
2. **Action Gating**: Human approval for high-risk operations
3. **Input Sanitization**: Validate and sanitize all external inputs before tool execution
4. **Output Monitoring**: Detect anomalous patterns in tool outputs and agent responses
5. **Blast Radius Containment**: Network isolation, credential scoping, filesystem restrictions

The question shifts from "will injection happen" to "how bad when it does."

### 5.2 Tool Argument Validation

- Every tool must sanitize and validate inputs before execution
- File system tools must validate parameters against allow-lists or sandboxed directories (path traversal defense)
- URL/URI parameters must be validated against SSRF allow-lists
- SQL/command arguments must use parameterized queries, never string interpolation
- JSON schema validation on all tool input parameters

### 5.3 Firewall Architecture

Two complementary LLM firewalls at the agent-tool boundary:

**Tool-Input Firewall (Minimizer)**: Strips unnecessary data and private information from tool call arguments before execution. Mitigates data exfiltration via tool arguments.

**Tool-Output Firewall (Sanitizer)**: Filters tool responses before feeding back to agent. Removes suspicious instructions and potentially malicious content from tool outputs.

### 5.4 CommandSans

Research paper proposing "surgical precision prompt sanitization" — identifying and neutralizing injection payloads within tool arguments while preserving legitimate content. Targeted approach vs. blanket filtering.

### 5.5 Resource URI Injection

MCP resource URIs are vulnerable to:
- **Path traversal**: `../../etc/passwd` in resource paths
- **SSRF**: Crafted URIs that cause server-side requests to internal services
- **Protocol smuggling**: URIs that exploit protocol handler differences

Defenses: URI normalization, scheme allow-listing, path canonicalization, internal network blocking.

Sources:
- [OWASP LLM Prompt Injection Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/LLM_Prompt_Injection_Prevention_Cheat_Sheet.html)
- [CommandSans (arXiv)](https://arxiv.org/html/2510.08829v1)
- [Multi-Agent LLM Defense Pipeline (arXiv)](https://arxiv.org/html/2509.14285v4)
- [Anthropic: Prompt Injection Defenses](https://www.anthropic.com/research/prompt-injection-defenses)
- [Google ADK Safety and Security](https://google.github.io/adk-docs/safety/)

---

## 6. Audit and Compliance

### 6.1 Logging Requirements

Every agent action must produce an audit record containing:
- **Intent**: What the agent was trying to accomplish
- **Policy version**: Which policy/rules were in effect
- **Decision context**: Counters, thresholds, and state at decision time
- **Cryptographic fingerprint**: Tamper-evident hash of the decision record
- **Tool invocation details**: Tool name, arguments (sanitized), return values, latency
- **Identity chain**: User -> agent -> tool delegation trail

### 6.2 SOC 2 Requirements

SOC 2 focuses on process consistency, control design, and operating effectiveness:
- Auditor's core question: "Does the control operate consistently as documented?"
- AI agents must demonstrate consistent behavior within defined boundaries
- Automated evidence collection required at scale (IBM Concert pattern)
- Immutable audit trails with cryptographic integrity

### 6.3 GDPR Implications

- **Right to explanation**: Agents making decisions affecting individuals must provide reasoning trails
- **Data minimization**: Agents should only access and process data necessary for the task
- **Purpose limitation**: Tool invocations must be traceable to a legitimate purpose
- **Data subject access requests**: Agent memory and conversation history may constitute personal data
- **Cross-border transfers**: Agent tool calls that route through different jurisdictions

### 6.4 The Scale Challenge

Autonomous agents operating at machine speed create exponential evidence collection challenges. Traditional compliance frameworks require human accountability and documented reasoning — autonomous action at scale is the primary barrier to enterprise deployment.

AgentComplianceLayer pattern: generates audit-ready reports (PDF, CSV, JSON) with complete decision trails, timestamps, and policy enforcement logs. Scheduled or on-demand generation.

Sources:
- [PolicyLayer: SOC 2 Compliance for AI Agents](https://www.policylayer.com/blog/soc2-compliance-ai-agents)
- [MindStudio: AI Agent Compliance](https://www.mindstudio.ai/blog/ai-agent-compliance)
- [IBM: Building Trustworthy AI Agents](https://www.ibm.com/think/insights/building-trustworthy-ai-agents-compliance-auditability-explainability)
- [Zenity: AI Agents Compliance](https://zenity.io/use-cases/business-needs/ai-agents-compliance)

---

## 7. Multi-Tenant Security

### 7.1 Context Contamination

The primary multi-tenant threat is context contamination — tenant-specific context not consistently enforced throughout the data pipeline:
- Shared vector databases filtering by similarity but failing to check tenant IDs
- Conversation history buffers where session management bugs merge traces from multiple tenants
- Agent memory stores that leak context across tenant boundaries

### 7.2 Tenant Identity Embedding

Every interaction must carry metadata: `{tenant_id, user_id, agent_id, session_id}`. This metadata must be applied to:
- All tool calls
- Resource paths and query filters
- Memory read/write operations
- Logging and audit records

Each tenant must exist within a logically and cryptographically separate identity boundary. Tokens issued to agents must contain tenant-scoped claims that cannot be altered or reused.

### 7.3 Isolation Patterns (AWS Prescriptive Guidance)

Three deployment models:

**Siloed Model**: Each tenant receives dedicated agents, no shared compute or resources. Maximum isolation, highest cost.

**Hybrid Model**: Critical agents siloed, non-sensitive agents pooled. Balances security with cost.

**Pooled Model**: All agents shared across tenants with common infrastructure. Tenant isolation enforced through:
- IAM policies scoping access to tenant resources
- Row-level security in databases
- Namespace isolation in memory stores
- Tenant-aware rate limiting and throttling

### 7.4 Memory Isolation

Agent memory requires special attention:
- Embedding stores must enforce tenant partitioning at the query level
- Long-term memory must be cryptographically separated per tenant
- Cross-tenant memory queries must be architecturally impossible, not just policy-prevented
- Memory garbage collection must respect tenant boundaries

### 7.5 Azure Confidential Computing (2025)

Microsoft released hardware-based encryption for multi-tenant AI workloads:
- Isolated execution environments using TEEs (Trusted Execution Environments)
- Zero-trust verification protocols
- Hardware-enforced tenant isolation

Sources:
- [AWS: Building Multi-Tenant Architectures for Agentic AI](https://docs.aws.amazon.com/prescriptive-guidance/latest/agentic-ai-multitenant/introduction.html)
- [AWS: Enforcing Tenant Isolation](https://docs.aws.amazon.com/prescriptive-guidance/latest/agentic-ai-multitenant/enforcing-tenant-isolation.html)
- [Prefactor: MCP Security for Multi-Tenant Agents](https://prefactor.tech/blog/mcp-security-multi-tenant-ai-agents-explained)
- [StackAI: Multi-Tenant AI Security](https://www.stackai.com/insights/multi-tenant-ai-security-for-enterprises-risks-best-practices-and-essential-checklist)
- [ResearchGate: Multi-Tenant Isolation Challenges](https://www.researchgate.net/publication/399564099_Multi-Tenant_Isolation_Challenges_in_Enterprise_LLM_Agent_Platforms)

---

## 8. Workload Identity

### 8.1 The Problem

AI agents are non-human identities (NHIs) that need to authenticate to external services. Static credentials (API keys, service account keys) are the primary risk:
- Keys don't expire automatically
- Keys can be exfiltrated via prompt injection
- Keys are often over-scoped
- No built-in audit trail for key usage

### 8.2 GCP Workload Identity Federation

- Workloads use short-lived tokens instead of service account keys
- Supports federation from AWS, Azure, OIDC providers, X.509 certificates
- IAM roles granted to federated identities in workload identity pools
- Tokens are time-bound (minutes to hours), expire automatically, require no storage

Cross-cloud: AWS workloads can exchange AWS IAM credentials for short-lived GCP access tokens. EC2 instances or Lambda functions with IAM roles authenticate directly to GCP.

### 8.3 AWS IAM for Agents

- IAM roles assumed by agent processes with session-based credentials
- STS (Security Token Service) for temporary credential generation
- Resource-based policies for fine-grained access control
- CloudTrail logging for all credential usage

### 8.4 Azure Entra (Managed Identity)

- System-assigned and user-assigned managed identities
- Automatic credential rotation handled by the platform
- Confidential computing integration for TEE-based identity attestation

### 8.5 Agent-Specific Patterns

- **Credential proxying**: Agent never sees raw credentials; a proxy injects credentials into requests
- **Just-in-time access**: Credentials provisioned only for the duration of a specific tool invocation
- **Credential-free architectures**: Workload identity federation eliminates static secrets entirely
- **Identity chain propagation**: User identity flows through agent to tool, maintaining attribution

### 8.6 Non-Human Identity Management (2025-2026 Trend)

Aembit and similar platforms extend workload IAM to cover all non-human identities — AI agents, CI/CD pipelines, containers, serverless functions. The trend is toward unified identity management for all machine-to-machine authentication, eliminating fragmented credential management.

Sources:
- [GCP: Workload Identity Federation](https://docs.google.com/iam/docs/workload-identity-federation)
- [GCP: Identities for Workloads](https://cloud.google.com/iam/docs/workload-identities)
- [AWS: Cross-Cloud Authentication with GCP](https://aws.amazon.com/blogs/security/access-aws-using-a-google-cloud-platform-native-workload-identity/)
- [Aembit: Workload IAM](https://aembit.io/blog/what-identity-federation-means-for-workloads-in-cloud-native-environments/)

---

## 9. Implications for mcpkit

### 9.1 Existing Coverage

mcpkit already implements several security patterns:
- **`auth/`**: JWT/JWKS validation, OAuth discovery, Bearer middleware, DPoP proof validation
- **`security/`**: RBAC, audit logging middleware
- **`sanitize/`**: Input sanitization for tool params
- **`observability/`**: OpenTelemetry tracing/metrics
- **`registry/`**: Middleware chain enabling defense-in-depth

### 9.2 Gaps to Address

Based on this research, the following areas represent gaps or enhancement opportunities:

**High Priority**:
1. **Tool description integrity** — No mechanism to detect rug-pull attacks where tool descriptions change after approval. Consider cryptographic hashing of tool definitions at registration time.
2. **Tool-output sanitization** — Current `sanitize/` focuses on input; output firewall (sanitizer) pattern from research is not implemented. Tool responses fed back to agents can carry injection payloads.
3. **Resource URI validation** — Path traversal and SSRF protection for MCP resource URIs beyond current sanitization.
4. **Tenant context propagation** — `{tenant_id, user_id, agent_id, session_id}` metadata through all tool calls, resource access, and memory operations.

**Medium Priority**:
5. **Human-in-the-loop gates** — Middleware that pauses execution for approval on high-risk tool invocations (write, delete, external API calls). Both sync and async patterns.
6. **Workload identity integration** — `auth/workload.go` for GCP/AWS/Azure credential-free authentication. Already on Phase 7 roadmap.
7. **Agent memory isolation** — Tenant-partitioned memory in `memory/` package with cryptographic separation.
8. **Supply chain verification** — Tool/server signature verification for `discovery/` and `gateway/` packages.

**Lower Priority (but important)**:
9. **Compliance report generation** — Structured audit trail export (JSON/CSV) from `security/` audit logs for SOC 2 evidence.
10. **Blast radius containment middleware** — Rate limiting + circuit breaking scoped per-tenant (partially covered by `resilience/`).
11. **Capability-based tool access** — Beyond RBAC, fine-grained capability tokens specifying exact resources each tool can access.

### 9.3 Defense-in-Depth Stack for mcpkit

Recommended middleware chain order implementing the five-layer defense:

```
Request Flow:
  1. auth/         → Identity verification, token validation, DPoP binding
  2. security/     → RBAC check, tenant context injection
  3. sanitize/     → Input validation, path traversal prevention, URI normalization
  4. resilience/   → Rate limiting, circuit breaking (per-tenant)
  5. observability/ → Tracing, metrics, audit logging
  6. [tool execution]
  7. sanitize/     → Output sanitization (NEW: strip injection payloads)
  8. observability/ → Response logging, latency metrics
```

### 9.4 Key Takeaway

Prompt injection remains unsolved at the model level. The only reliable defense is defense-in-depth at the infrastructure level — the exact pattern mcpkit's middleware chain enables. Every layer assumes the previous layer has been bypassed.
