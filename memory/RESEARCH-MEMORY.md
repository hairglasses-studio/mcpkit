# Memory and Context Management for AI Agents

Research survey covering memory taxonomies, context management strategies, persistent stores,
retrieval mechanisms, multi-agent memory, MCP integration, cognitive architectures, and
production implementations. Compiled March 2026 from 2024-2025 literature and systems.

---

## 1. Memory Taxonomies

### Classical Cognitive Science Taxonomy

Human memory research provides the foundational vocabulary:

- **Sensory memory** -- raw input buffer (sub-second)
- **Short-term / working memory** -- active manipulation, limited capacity (~7 items / 4 chunks)
- **Long-term memory**:
  - **Episodic** -- specific experiences bound to time and place
  - **Semantic** -- general facts and knowledge, context-detached
  - **Procedural** -- motor skills and "how-to" knowledge

### LLM Agent Memory Taxonomy (2025 Survey)

The survey "Memory in the Age of AI Agents" (Liu et al., arXiv 2512.13564, Dec 2025) argues
that the traditional short-term/long-term split is insufficient for modern agent systems and
proposes a three-dimensional taxonomy:

1. **Forms** (how memory is physically realized):
   - **Token-level** -- raw text stored and injected into context
   - **Parametric** -- encoded in model weights (fine-tuning, LoRA)
   - **Latent** -- compressed vector representations (embeddings, KV cache states)

2. **Functions** (what the memory is for):
   - **Factual** -- entity attributes, relationships, world knowledge
   - **Experiential** -- task traces, interaction logs, episodic records
   - **Working** -- scratchpad for current reasoning, intermediate states

3. **Dynamics** (how memory changes):
   - **Formation** -- how new memories are created and encoded
   - **Evolution** -- consolidation, updating, conflict resolution
   - **Retrieval** -- how stored memories are accessed and surfaced

### MemGPT / Letta Memory Model

MemGPT (Packer et al., 2023; now Letta) draws an OS-inspired analogy:

| OS Concept | MemGPT Equivalent | Description |
|---|---|---|
| RAM | Main context | Fixed-size prompt window |
| Virtual memory | Virtual context management | Paging between tiers |
| Disk | External storage | Archival + recall storage |

The main context contains three sections:
- **System prompt** (static) -- base instructions, persona
- **Working context** (dynamic) -- self-editable scratchpad of key facts
- **Message buffer** (FIFO) -- recent conversation turns

External storage splits into:
- **Recall storage** -- full interaction history, searchable by text/time
- **Archival storage** -- long-term vector-indexed store for large documents

Key insight: MemGPT agents **self-edit** their own memory via function calls. The LLM
decides when to page information in/out and when to update the working context. This is
analogous to an OS managing page faults -- the agent "traps" when it needs information
not in main context and issues retrieval calls.

**Semantization process**: MemGPT mirrors human memory consolidation where episodic memories
(specific interactions) gradually transform into semantic memories (general knowledge),
decoupling core information from contextual details over time.

### MIRIX Multi-Type Memory

The MIRIX architecture implements specialized memory managers for distinct types:
- Core memory, episodic memory, semantic memory, procedural memory
- Resource memory, knowledge vault
- A **meta-memory manager** coordinates routing and retrieval across all types

---

## 2. Context Window Management

### The Fundamental Problem

LLM context windows are finite (4K-2M tokens). Agents operating over long sessions,
multi-turn conversations, or large codebases must decide what stays in context and what
gets evicted. This is the central tension in agent memory design.

### Strategy Taxonomy

**Summarization (Abstractive)**
- Condense older conversation segments into shorter summaries
- ConversationSummaryMemory pattern: running summary updated after each exchange
- Multi-level summarization maintains 91% of critical information at 68% size reduction
- Risk: lossy compression can drop important details

**Summarization (Extractive)**
- Select important sentences/passages verbatim
- 2024 study: extractive reranker-based compression achieved +7.89 F1 on 2WikiMultihopQA
  at 4.5x compression, outperforming abstractive approaches
- LLMLingua family (Microsoft Research): up to 20x compression with 1.5% performance loss

**Sliding Window**
- Fixed-size window slides forward, oldest content ages out
- Simple and predictable but loses all historical context
- Often combined with summarization of evicted content

**Hierarchical Context**
- Multiple memory tiers at different time scales
- Immediate working memory (current session)
- Episodic memory (important past interactions)
- Semantic memory (extracted general knowledge)
- Each tier has different retention policies and access patterns

**Observation Masking**
- Hide older, less important information rather than summarizing
- JetBrains research (2025) on efficient context management for agents
- Can be combined with importance scoring

### Low-Level: KV Cache Management

At the inference level, attention sink and cache eviction research addresses context limits:

**StreamingLLM (MIT, ICLR 2024)**
- Discovery: initial tokens serve as "attention sinks" even without semantic importance
- Keeping KV of initial tokens + recent window recovers full-attention performance
- Enables infinite-length streaming with finite attention windows
- Works across Llama-2, MPT, Falcon without fine-tuning

**SAGE-KV (2025)**
- Self-Attention Guided Eviction: LLMs implicitly "know" which tokens can be dropped
- One-time top-k selection at token and head levels after pre-filling
- 4x memory efficiency improvement over StreamingLLM with better accuracy

**SharedLLM**
- Splits pretrained model into lower/upper modules
- Encodes long contexts as shallow KV representations with multi-grained binary tree compression
- Handles hundreds of thousands of tokens

### Context Engineering (2025 Paradigm)

The field has shifted from "prompt engineering" to "context engineering" -- the art of
constructing the right context for each agent step. Key principles:
- Every token in context should earn its place
- Dynamic context assembly per step, not static prompt templates
- Retrieval-augmented generation as context filling, not just knowledge lookup

---

## 3. Persistent Memory Stores

### Comparison of Major Platforms

| Platform | Architecture | Storage | Retrieval | Latency | Open Source |
|----------|-------------|---------|-----------|---------|-------------|
| **Mem0** | Dual (vector + graph) | Vector DB + Neo4j | Semantic + entity | Fast | Yes (core) |
| **Zep/Graphiti** | Temporal knowledge graph | Neo4j + vector | Graph traversal + semantic | P95: 300ms | Yes (Graphiti) |
| **LangMem** | Flat key-value + vector | Structured store | Vector similarity | Slow (p50: 18s) | Yes |
| **Letta** | MemGPT two-tier | PostgreSQL + vector | Function-call driven | Moderate | Yes |
| **MemoClaw** | Hierarchical extraction | Vector DB | Multi-level | Moderate | Yes |

### Mem0

- YC-backed, 50K+ developers
- Hierarchical memory at user, session, and agent levels
- Extracts "memories" (facts, preferences) from interactions automatically
- Dual retrieval: vector semantic search + optional graph for entity relationships
- Published research paper (arXiv 2504.19413): F1 score 28.64, J score 51.15 on DMR benchmark
- OpenMemory MCP server for Claude Desktop integration
- Graph memory feature (Jan 2026) adds entity extraction and relationship modeling

### Zep / Graphiti

- Core innovation: temporal knowledge graph with bi-temporal model
- Tracks both when events occurred and when they were ingested
- Every graph edge has explicit validity intervals (facts can expire/be superseded)
- Entity extraction via LLM + name matching for entity resolution
- LongMemEval benchmark: up to 18.5% accuracy improvement, 90% latency reduction vs baselines
- Supports both prescribed and learned ontology
- Graphiti is open-source; Zep Cloud is the managed service
- Provides an MCP server for knowledge graph access

### LangMem / LangChain Memory

- Part of LangGraph ecosystem
- Flat key-value memory with vector search
- Focuses on working memory: compressing long histories into actionable summaries
- ConversationBufferMemory, ConversationSummaryMemory, ConversationKGMemory patterns
- Performance concerns: p50 search latency of ~18s, p95 of ~60s
- Best suited for LangGraph-native applications

### Storage Backend Options

Production systems use various backends:
- **Vector databases**: Pinecone, Qdrant, Weaviate, pgvector, MongoDB Atlas
- **Graph databases**: Neo4j (dominant for knowledge graph memory)
- **Relational**: PostgreSQL (Letta, various MCP memory servers)
- **Document stores**: MongoDB, DynamoDB
- **File-based**: SQLite, JSON files, markdown (CLAUDE.md pattern)
- **In-memory**: For testing and ephemeral sessions

### Cost Considerations

A full retrieval pipeline costs roughly $0.002-0.01 per query at low volume. Plain
filesystem approaches (markdown files) score 74% on memory tasks, beating some
specialized vector-store libraries -- suggesting that simpler approaches can be
competitive for many use cases.

---

## 4. Memory Retrieval

### Core Retrieval Signals

Most systems combine multiple signals for memory retrieval scoring:

**Recency (temporal decay)**
- Exponential decay: score decreases hourly by a decay factor (e.g., 0.995)
- Recently accessed memories score higher
- Mirrors the psychological recency effect
- ACT-R base-level activation: decay function based on time since last access and frequency

**Relevance (semantic similarity)**
- Cosine similarity between query embedding and memory embeddings
- Standard vector search approach
- Works well for factual retrieval, less well for procedural knowledge

**Importance (salience weighting)**
- LLM-generated importance scores for each memory
- Determines the agent's perceived significance
- Used to distinguish routine interactions from pivotal moments

**Frequency (access count)**
- Memories accessed more often are strengthened
- Parallels spaced repetition in human learning

### Combined Scoring

The generative agents framework (Park et al., 2023) popularized the combined formula:

```
score = alpha * recency + beta * importance + gamma * relevance
```

Recent work (Frontiers, 2025) proposes cross-attention networks trained to learn optimal
weighting of these signals, outperforming hand-tuned combinations.

### Memory Consolidation

Several systems implement consolidation -- the process of transforming and compressing
memories over time:

- **Episodic-to-semantic**: Repeated similar experiences get abstracted into general knowledge
- **Memory merging**: Overlapping memories are merged, with conflicts resolved by recency
- **Hierarchical summarization**: Old detailed memories become progressively more abstract
- **Forgetting**: Low-activation memories are pruned (SOAR's automatic forgetting policy)

### Advanced Retrieval Patterns

**Graph-based retrieval** (Zep/Graphiti):
- Traverse entity relationships for multi-hop queries
- Temporal filtering: retrieve facts valid at a specific time
- Community detection for topic-based retrieval

**Nemori memory graphs**:
- Semantically segmented episodes
- Capture relational and temporal dependencies
- Support multi-hop, temporal, and topic-based retrieval

**Field-theoretic memory** (arXiv 2602.21220, 2026):
- Models memory as continuous fields rather than discrete entries
- Attention-based dynamics for context preservation
- Novel approach treating memory retrieval as field sampling

---

## 5. Multi-Agent Shared Memory

### Architectural Patterns

**Blackboard Architecture**
- Central shared workspace accessible to all agents
- Agents read from and write to the blackboard
- A controller/orchestrator manages access and conflict resolution
- 2025 LLM-based blackboard systems show 13-57% improvement over master-slave frameworks
- Well-suited for creative tasks where specialists contribute partial solutions

**Orchestrator-Hosted Memory**
- A central coordinator acts as memory hub for the entire team
- Aggregates, summarizes, and indexes information on behalf of all agents
- PC-Agent pattern: manager maintains global task state, workers handle subtasks
- Simple but creates a bottleneck and single point of failure

**Externally-Hosted Shared Memory**
- All agents query a shared database, knowledge graph, or document store
- No single agent owns the memory
- More scalable but requires coordination for write conflicts

**Federated Memory**
- Each agent maintains its own memory, with selective sharing
- Agents can publish memories to shared channels
- Privacy-preserving: agents control what they expose

### Key Challenges

- **Write conflicts**: Multiple agents updating the same memory simultaneously
- **Consistency**: Ensuring all agents see a coherent view of shared knowledge
- **Scale**: Memory grows combinatorially with number of agents
- **Attribution**: Tracking which agent contributed which memory
- **Privacy**: Not all agents should access all memories

### Multi-Agent Memory Survey (2024-2025)

The TechRxiv paper "Memory in LLM-based Multi-agent Systems" identifies that agentic/multi-agent
papers skyrocketed from 820 in 2024 to over 2,500 in 2025, with shared memory as a
critical unsolved problem.

---

## 6. MCP and Memory

### Current MCP Memory Capabilities

The Model Context Protocol provides several primitives relevant to memory:

**Resources as Memory**
- Resources expose data via URI templates (e.g., `memory://user/{userId}/facts`)
- Resource subscriptions enable real-time updates when memory changes
- Resource templates allow parameterized memory access patterns
- Static resources can serve as persistent knowledge bases

**Tools as Memory Operations**
- Tools provide CRUD operations on memory stores
- The reference Memory MCP Server uses a knowledge graph with entities and relations
- Tool output schemas (2025-11-25 spec) reduce context waste from memory retrieval

**Sampling for Reflection**
- Sampling with tool calling (new in 2025-11-25 spec) enables:
  - Server-side agent loops for memory consolidation
  - Autonomous memory management without client orchestration
  - Multi-step reasoning about what to remember

**Notifications**
- Resource change notifications can trigger memory updates
- Tool list changes can signal new memory capabilities

### MCP Memory Server Implementations

Several community implementations exist:

1. **@modelcontextprotocol/server-memory** -- Reference implementation with local knowledge graph
2. **memory-bank-mcp** -- Cline-inspired memory bank for project context
3. **memory-keeper** -- Persistent context management for Claude Code
4. **local-memory-mcp** -- SQLite-backed local memory with full-text search
5. **mcp-ai-memory** -- Semantic memory management with vector search
6. **OpenMemory MCP** (Mem0) -- Cloud-backed memory with entity extraction
7. **Zep MCP Server** -- Knowledge graph memory via Graphiti

### What MCP is Missing for Memory

**No native memory primitive**: Memory is implemented via tools and resources, but there
is no first-class "memory" capability in the protocol. This means:
- No standardized memory schema or operations
- No interoperability between memory implementations
- Clients cannot discover memory capabilities uniformly

**No memory lifecycle management**: No protocol support for:
- Memory consolidation or garbage collection
- Memory importance scoring or decay
- Memory sharing between servers
- Memory migration or export

**No context budget negotiation**: Servers cannot:
- Declare how much context their memories need
- Negotiate context allocation with other servers
- Prioritize which memories to inject when context is limited

**No temporal semantics**: Resources have no built-in concept of:
- Validity intervals (when a fact was true)
- Memory age or freshness
- Temporal queries ("what did the user prefer last month?")

### Implications for mcpkit

The `memory/` package should consider:
- Exposing memory via both tools (CRUD) and resources (read access, subscriptions)
- Using resource templates for parameterized memory access
- Leveraging tool output schemas to minimize context usage
- Supporting pluggable storage backends (the existing `Store` interface)
- Providing middleware for automatic memory extraction from tool calls
- Implementing memory consolidation via sampling (when available)

---

## 7. Cognitive Architectures

### SOAR

Developed at University of Michigan, SOAR has four memory types:

- **Working Memory** -- current situational awareness, active goals, operator selections
- **Procedural Memory** -- production rules (if-then), the primary driver of behavior
- **Semantic Memory** -- declarative facts about the world (Soar 9+)
- **Episodic Memory** -- temporal record of working memory snapshots (Soar 9+)

Key mechanisms:
- **Impasses** trigger learning -- when SOAR cannot proceed, it drops into a sub-state
- **Chunking** -- successful sub-state resolutions become new procedural rules
- **Forgetting** -- automatic discarding of low-activation knowledge based on base-level
  activation and reconstructability scores

Recent LLM integration (2025): A novel framework automates generation of executable SOAR
rules from natural language with >86% success rate, enabling natural language programming
of cognitive architectures.

### ACT-R

Anderson's ACT-R (Adaptive Control of Thought-Rational):

- **Declarative memory** -- chunks (facts) with activation levels
- **Procedural memory** -- production rules with utility values
- **Base-level activation** -- `B_i = ln(sum(t_j^{-d}))` where t_j is time since j-th access
- **Spreading activation** -- related chunks boost each other's activation
- **Partial matching** -- retrieval can succeed with imperfect matches

LLM integration (2025): A novel architecture directly integrates ACT-R's memory activation
model into the LLM generation process, making memory recall and forgetting linked to content
generation itself. Dynamic adjustment of memory retention based on emotional arousal and
surprise signals.

### Modern LLM-Based Cognitive Architectures

**CoALA (Cognitive Architectures for Language Agents, 2023)**
- Framework mapping LLM agents to cognitive architecture components
- Working memory = context window
- Long-term memory = external retrieval stores + model weights
- Decision-making = LLM inference + action selection

**Generative Agents (Stanford, 2023)**
- Observation -> Reflection -> Planning loop
- Memory stream with recency/importance/relevance scoring
- Reflection: periodic synthesis of observations into higher-level insights
- Still the most cited architecture for LLM agent memory

**Voyager (2023) / DEPS (2024)**
- Procedural memory as a skill library (code)
- Successful action sequences stored as reusable programs
- Memory is executable, not just declarative

### Key Insight: Convergence

Modern LLM agent architectures are converging on similar memory structures regardless
of their origin (cognitive science, OS design, or pure engineering):

| Component | SOAR | ACT-R | MemGPT | Generative Agents |
|-----------|------|-------|--------|-------------------|
| Working memory | Working memory | Buffers | Main context | Current observation |
| Episodic | Episodic memory | Declarative chunks | Recall storage | Memory stream |
| Semantic | Semantic memory | Declarative chunks | Archival storage | Reflections |
| Procedural | Production rules | Productions | Function calls | Plans |
| Retrieval | Activation + cue | Base-level + spreading | Search queries | Recency+importance+relevance |

---

## 8. Practical Production Implementations

### ChatGPT Memory

- Precomputed context injection with layered caching
- Memories stored as structured data objects associated with user profile
- Memory object loaded and injected into system prompt each session
- Users can view, edit, and delete individual memories
- Lightweight and predictable but limited depth
- No cross-conversation reasoning about memories themselves

### Claude Memory (Anthropic)

- RAG-style on-demand retrieval with dynamic memory updates
- **Memory Tool** (memory_20250818): stores/retrieves info across conversations
- Writes to a memory file directory, building knowledge over time
- **CLAUDE.md system**: project-level persistent context
  - Claude analyzes codebase and creates files with build commands, conventions
  - Hierarchical: `~/.claude/CLAUDE.md` (global) -> project -> directory level
  - Plain markdown, human-readable and version-controllable
- Practical finding: filesystem-based approaches score 74% on memory benchmarks

### Cursor / Windsurf

- **Rules files** for persistent project context (analogous to CLAUDE.md)
- **Codebase indexing**: combination of grep/file search + knowledge graph retrieval + re-ranking
- Indexed embeddings of entire codebase for semantic search
- Context assembled per-query from multiple retrieval sources
- Practical focus on code understanding rather than conversation memory

### GitHub Copilot

- Codebase-wide context via repository indexing
- File-level context from open editors and recent files
- No persistent memory across sessions (as of early 2025)
- Relies heavily on in-context code rather than stored memories

### Key Production Insights

1. **Simple works**: Markdown files and grep often outperform sophisticated vector stores
   for structured knowledge (project conventions, preferences)

2. **Hybrid is necessary**: No single retrieval method works for all memory types. Production
   systems combine keyword search, semantic search, and structured lookups.

3. **User control matters**: All production systems provide memory inspection and deletion.
   Trust requires transparency.

4. **Cost is real**: Memory retrieval adds latency and token cost to every interaction.
   $0.002-0.01 per query at low volume; scales linearly.

5. **Context is king**: The shift from "prompt engineering" to "context engineering" reflects
   that what you put in context matters more than how you phrase the prompt.

---

## 9. Key Takeaways and Open Problems

### What Works Now

- **Two-tier memory** (working + archival) is the minimum viable architecture
- **Vector similarity + recency** covers most retrieval needs
- **Self-editing memory** via function calls (MemGPT pattern) is the dominant paradigm
- **Knowledge graphs** add significant value for entity-rich domains (Zep: 18.5% accuracy gain)
- **Temporal awareness** is essential for long-lived agents (bi-temporal models)

### Open Research Problems

1. **Memory automation**: When should an agent remember vs. forget? Current systems rely on
   heuristics or LLM judgment, both brittle.

2. **Memory evaluation**: No standard benchmarks. DMR, LongMemEval, and LOCOMO exist but
   test different aspects. The field lacks a comprehensive evaluation framework.

3. **Multi-modal memory**: Almost all current work is text-only. How to store and retrieve
   memories from images, audio, structured data?

4. **Reinforcement learning for memory**: Using RL to learn optimal memory policies
   (what to store, when to retrieve, how to consolidate).

5. **Trustworthy memory**: Memory poisoning, hallucinated memories, privacy in shared
   memory systems. Mostly unexplored.

6. **Memory-aware planning**: Agents that reason about their own memory limitations
   and plan accordingly (meta-cognition).

7. **Efficient consolidation**: Current consolidation is expensive (requires LLM calls).
   Need cheaper methods that preserve quality.

8. **Cross-agent memory interoperability**: No standard format for memory exchange between
   different agent frameworks. MCP could fill this gap.

---

## Sources

### Survey Papers
- [Memory in the Age of AI Agents (Liu et al., 2025)](https://arxiv.org/abs/2512.13564)
- [A Survey on the Memory Mechanism of LLM-based Agents (ACM TOIS)](https://dl.acm.org/doi/10.1145/3748302)
- [Memory in LLM-based Multi-agent Systems (TechRxiv)](https://www.techrxiv.org/users/1007269/articles/1367390/master/file/data/LLM_MAS_Memory_Survey_preprint_/LLM_MAS_Memory_Survey_preprint_.pdf)
- [AI Meets Brain: Memory Systems from Cognitive Neuroscience to Autonomous Agents](https://arxiv.org/html/2512.23343v1)
- [Rethinking Memory in AI: Taxonomy, Operations, Topics](https://arxiv.org/html/2505.00675v1)
- [Survey of AI Agent Memory Frameworks (Graphlit)](https://www.graphlit.com/blog/survey-of-ai-agent-memory-frameworks)
- [Agent Memory Paper List (GitHub)](https://github.com/Shichun-Liu/Agent-Memory-Paper-List)

### Foundational Systems
- [MemGPT: Towards LLMs as Operating Systems (Packer et al., 2023)](https://arxiv.org/abs/2310.08560)
- [Letta / MemGPT Documentation](https://docs.letta.com/concepts/memgpt/)
- [Zep: A Temporal Knowledge Graph Architecture for Agent Memory](https://arxiv.org/abs/2501.13956)
- [Graphiti: Knowledge Graph Memory (Neo4j)](https://neo4j.com/blog/developer/graphiti-knowledge-graph-memory/)
- [Mem0: Building Production-Ready AI Agents with Scalable Long-Term Memory](https://arxiv.org/pdf/2504.19413)
- [Mem0 Research](https://mem0.ai/research)

### Context Management
- [Efficient Context Management for LLM-Powered Agents (JetBrains, 2025)](https://blog.jetbrains.com/research/2025/12/efficient-context-management/)
- [Context Engineering for Agents (Lance Martin)](https://rlancemartin.github.io/2025/06/23/context_engineering/)
- [StreamingLLM: Efficient Streaming with Attention Sinks (ICLR 2024)](https://arxiv.org/abs/2309.17453)
- [SAGE-KV: Self-Attention Guided KV Cache Eviction](https://arxiv.org/abs/2503.08879)
- [Context Window Management Strategies (Maxim)](https://www.getmaxim.ai/articles/context-window-management-strategies-for-long-context-ai-agents-and-chatbots/)

### Cognitive Architectures
- [Cognitive LLMs: Integrating Cognitive Architectures and LLMs](https://arxiv.org/pdf/2408.09176)
- [Human-Like Remembering and Forgetting: ACT-R-Inspired Memory Architecture (HAI 2024)](https://dl.acm.org/doi/10.1145/3765766.3765803)
- [Enhancing Memory Retrieval via LLM-Trained Cross Attention Networks (Frontiers, 2025)](https://www.frontiersin.org/journals/psychology/articles/10.3389/fpsyg.2025.1591618/full)
- [Field-Theoretic Memory for AI Agents (2026)](https://arxiv.org/html/2602.21220)

### Multi-Agent Memory
- [LLM Multi-Agent Systems Based on Blackboard Architecture (2025)](https://arxiv.org/abs/2507.01701)
- [Multi-Agent Memory from a Computer Architecture Perspective (2026)](https://arxiv.org/html/2603.10062)
- [Designing Effective Multi-Agent Architectures (O'Reilly)](https://www.oreilly.com/radar/designing-effective-multi-agent-architectures/)

### MCP and Memory
- [MCP Specification 2025-11-25](https://modelcontextprotocol.info/specification/2025-11-25/)
- [One Year of MCP: November 2025 Spec Release](http://blog.modelcontextprotocol.io/posts/2025-11-25-first-mcp-anniversary/)
- [OpenMemory MCP (Mem0)](https://mem0.ai/blog/introducing-openmemory-mcp)
- [Zep Knowledge Graph MCP Server](https://www.getzep.com/product/knowledge-graph-mcp/)
- [Memory MCP Server (npm)](https://www.npmjs.com/package/@modelcontextprotocol/server-memory)

### Production Systems
- [Reflections on ChatGPT and Claude's Memory Systems (Milvus)](https://milvus.io/blog/reflections-on-chatgpt-and-claude-memory-systems.md)
- [Comparing Memory Implementations of Claude and ChatGPT (Simon Willison)](https://simonwillison.net/2025/Sep/12/claude-memory/)
- [Claude Memory Tool Documentation](https://platform.claude.com/docs/en/agents-and-tools/tool-use/memory-tool)
- [How Claude Remembers Your Project (Claude Code Docs)](https://code.claude.com/docs/en/memory)
- [Memory & Context Management Cookbook](https://platform.claude.com/cookbook/tool-use-memory-cookbook)
- [Mem0 vs Zep vs LangMem vs MemoClaw Comparison (DEV Community)](https://dev.to/anajuliabit/mem0-vs-zep-vs-langmem-vs-memoclaw-ai-agent-memory-comparison-2026-1l1k)
- [Design Patterns for Long-Term Memory in LLM Architectures (Serokell)](https://serokell.io/blog/design-patterns-for-long-term-memory-in-llm-powered-architectures)
