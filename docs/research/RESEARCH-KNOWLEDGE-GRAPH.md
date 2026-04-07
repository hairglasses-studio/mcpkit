# Knowledge Graphs for AI Agent Systems

Research compiled 2026-03-14. Covers fundamentals, LLM integration patterns, Microsoft GraphRAG, agent knowledge bases, dynamic construction, MCP relevance, and GNN+LLM research.

---

## 1. Knowledge Graph Fundamentals

### Core Concepts

A **knowledge graph** (KG) represents structured information as a network of entities (nodes) and their relationships (edges). Two dominant data models exist:

**RDF (Resource Description Framework):**
- Expresses information as **triples**: subject-predicate-object (e.g., `<Alice> <worksAt> <Acme>`)
- Standardized by W3C; queried via SPARQL
- Supports formal ontologies through RDFS and OWL (class hierarchies, property domains/ranges, cardinality constraints, transitive/symmetric/inverse relations)
- Strength: semantic interoperability, federated queries across datasets
- Weakness: cannot represent multiple distinct relationships of the same type between two nodes; limited expressiveness for application-specific attributes on edges

**Property Graphs (Labeled Property Graphs / LPG):**
- Nodes and edges carry arbitrary key-value properties
- Queried via Cypher (Neo4j), Gremlin (Apache TinkerPop), or AQL (ArangoDB)
- Strength: intuitive modeling, high-speed traversal, scales well for big data workloads
- Weakness: no standardized ontology layer (though schema constraints exist per database)

**For AI agent systems, property graphs are the dominant choice in 2024-2025** due to alignment with embedding-centric workflows, flexible schemas, and performance characteristics. RDF remains relevant for enterprise knowledge management and cross-organization data sharing.

### Graph Databases

| Database | Model | Key Characteristics |
|----------|-------|-------------------|
| **Neo4j** | Property Graph (Cypher) | Market leader; best for pure graph workloads; extensive LLM/agent integrations |
| **FalkorDB** | Property Graph (Cypher subset) | Sparse matrix representation (GraphBLAS); sub-10ms query latency; optimized for GraphRAG |
| **ArangoDB** | Multi-model (graph + document + key-value) | AQL query language; flexible for hybrid workloads; self-hosted or cloud |
| **Amazon Neptune** | RDF + Property Graph | Managed AWS service; supports SPARQL, Gremlin, OpenCypher; integrated GraphRAG with Bedrock |
| **Memgraph** | Property Graph (Cypher) | In-memory; real-time streaming graph analytics; MCP server available |

### Ontologies

An ontology defines the schema of a knowledge graph: entity types, relationship types, constraints, and inference rules. Two approaches emerging in agent systems:

- **Prescribed ontology**: Domain experts define the schema upfront. More precise but requires human effort.
- **Learned/emergent ontology**: LLMs extract entity types and relationships from data, building the schema dynamically. More scalable but requires deduplication and consistency enforcement (see Graphiti in Section 5).

---

## 2. Knowledge Graph + LLM Integration

Three fundamental paradigms have emerged (per multiple 2024-2025 surveys):

### 2.1 KG-Augmented LLMs

KGs provide structured context to improve LLM accuracy and reduce hallucinations.

- **Retrieval-augmented prompting**: Dynamically enrich LLM context windows with KG subgraphs relevant to the query
- **Entity linking**: Ground LLM outputs to canonical KG entities, reducing ambiguity
- **CuriousLLM** (Yang & Zhu, 2025): Integrates KG prompting, reasoning-infused LLM agent, and graph traversal agent for multi-document QA

### 2.2 LLM-Augmented KGs (KG Construction)

LLMs automate the traditionally expensive process of building knowledge graphs.

- **KGGEN** (Mo et al., 2025): Two-phase extraction — first detect entities, then generate relations — reducing error propagation
- **KARMA framework**: Multi-agent design where specialized agents handle schema alignment, conflict resolution, and quality evaluation
- Classical pipeline reshaped: ontology engineering, knowledge extraction, and knowledge fusion all benefit from LLM capabilities
- Scale demonstrated: one earthquake emergency framework extracted 284,801 entities and 833,000 relationships from 2,682 documents

### 2.3 Synergized Frameworks

Bidirectional integration where KGs and LLMs mutually enhance each other:

- LLMs reason over KG structure while KGs ground LLM outputs
- **LLM-Align / EntGPT**: Multi-step prompting for semantic discrimination with two-phase refinement pipelines
- **Workshop ecosystem**: LLM-TEXT2KG (4th edition in 2025) dedicated to LLM-integrated KG generation

### Key Resources
- [KG-LLM-Papers](https://github.com/zjukg/KG-LLM-Papers) — comprehensive paper list maintained on GitHub
- [LLM-empowered KG construction survey](https://arxiv.org/abs/2510.20345) (2025)
- [Springer survey on augmenting KGs with LLMs](https://link.springer.com/article/10.1007/s44163-024-00175-8) (2024)

---

## 3. Microsoft GraphRAG

### The Problem with Naive RAG

Standard vector-based RAG retrieves text chunks by semantic similarity. This fails for **global sensemaking queries** ("What are the main themes in this dataset?") because:
- No single chunk contains the answer
- Chunk-level retrieval misses cross-document patterns
- No mechanism for hierarchical summarization

### GraphRAG Architecture

**Paper**: [From Local to Global: A Graph RAG Approach to Query-Focused Summarization](https://arxiv.org/abs/2404.16130) (Edge et al., Microsoft Research, 2024)

**Indexing pipeline:**
1. **Entity & relationship extraction**: LLM extracts entities and their relationships from source text chunks
2. **Knowledge graph construction**: Build a graph from extracted entities/relationships
3. **Community detection**: Apply Leiden clustering algorithm to detect hierarchical communities of densely connected nodes
4. **Community summarization**: LLM generates summaries for each community, bottom-up through the hierarchy (lower-level summaries feed into higher-level ones)

**Query modes:**

| Mode | How it works | Best for |
|------|-------------|----------|
| **Local Search** | Start from query-relevant entities, traverse neighborhood | Specific factual questions |
| **Global Search** | Map query across all community summaries, reduce to final answer | Thematic/sensemaking questions |
| **DRIFT Search** | Dynamic Reasoning and Inference with Flexible Traversal; starts with community info then traverses locally | Hybrid queries needing both breadth and depth |

**Results**: GraphRAG achieved 72-83% higher comprehensiveness and 62-82% higher diversity vs traditional RAG, with up to 97% fewer tokens for root-level summaries.

### LazyGraphRAG (2025)

A radical cost optimization that eliminates up-front LLM summarization:
- **Indexing cost = vector RAG** (0.1% of full GraphRAG indexing cost)
- Defers LLM calls to query time using an NLP-based graph built from noun-phrase co-occurrence
- **Outperforms all methods on local queries** at comparable cost to vector RAG
- Matches GraphRAG Global Search quality at **700x lower query cost**
- Now available through Microsoft Discovery platform

### LightRAG (EMNLP 2025)

Open-source alternative from University of Hong Kong (28K+ GitHub stars):
- Dual-level retrieval: low-level (specific entities) and high-level (thematic)
- Supports Neo4j, PostgreSQL as backends
- Reranker support for mixed query performance
- Significantly cheaper than full GraphRAG

**Repo**: [github.com/HKUDS/LightRAG](https://github.com/HKUDS/LightRAG)

---

## 4. Agent Knowledge Bases: Approaches Compared

### Vector Store

- **How**: Embed text chunks; retrieve by cosine similarity
- **Strengths**: Simple deployment, strong semantic matching, handles unstructured data well
- **Weaknesses**: No explicit relationships, "semantic drift" for multi-hop reasoning, no temporal awareness
- **Examples**: Pinecone, Weaviate, Chroma, pgvector

### Knowledge Graph

- **How**: Entities as nodes, explicit typed relationships as edges; traverse graph for context
- **Strengths**: Precise relational reasoning, multi-hop queries, explainable retrieval paths, reduces hallucination
- **Weaknesses**: Construction cost, schema design complexity, harder to handle purely unstructured data
- **Examples**: Neo4j, FalkorDB, Amazon Neptune

### Hybrid (Industry Consensus for 2025+)

The dominant trend is combining both approaches:

1. **Vector retrieval** identifies relevant entry nodes in the KG (broad, fuzzy recall)
2. **Graph traversal** extracts precise relational context from those entry points (strict, deterministic precision)
3. **Combined context** feeds into LLM for generation

**Trade-off**: Requires maintaining two systems in sync, but delivers the best accuracy for complex agentic workloads.

### Comparison Matrix

| Dimension | Vector Store | Knowledge Graph | Hybrid |
|-----------|-------------|----------------|--------|
| Setup complexity | Low | High | High |
| Semantic search | Excellent | Moderate | Excellent |
| Relational reasoning | Poor | Excellent | Excellent |
| Multi-hop queries | Poor | Excellent | Excellent |
| Temporal awareness | None (unless encoded) | Native (with temporal KG) | Native |
| Maintenance burden | Low | Moderate | High |
| Hallucination reduction | Moderate | High (90%+ with FalkorDB) | Highest |

---

## 5. Dynamic Knowledge Graph Construction

### The Challenge

Static KGs become stale. Agents operating in real-world environments need KGs that evolve with new information, correct errors, and track change over time.

### Graphiti / Zep (Key Innovation, 2025)

**Paper**: [Zep: A Temporal Knowledge Graph Architecture for Agent Memory](https://arxiv.org/abs/2501.13956) (Rasmussen et al., January 2025)

**Repo**: [github.com/getzep/graphiti](https://github.com/getzep/graphiti)

Core capabilities:

- **Incremental graph construction**: No need to recompute entire graphs when data changes; integrates updates, resolves conflicts based on temporal metadata
- **Bi-temporal model**:
  - *Event Time (T)*: When the fact actually occurred
  - *Ingestion Time (T')*: When information was added to the graph
  - Enables reasoning about retroactive data, corrections, and fact supersession
- **Automatic ontology**: Builds ontology from incoming data, deduplicates nodes, maintains consistent edge labels
- **Hierarchical memory**: Mirrors human cognition with episodic (raw data), semantic (extracted entities), and community (domain summaries) subgraphs
- **Hybrid retrieval**: Combines semantic embeddings, BM25 keyword search, and graph traversal — no LLM calls during retrieval
- **Performance**: P95 retrieval latency of 300ms; up to 18.5% accuracy improvement with 90% latency reduction vs baselines
- **Default backend**: FalkorDB (sub-10ms graph queries)

### Agentic KG Construction (DeepLearning.AI Course)

DeepLearning.AI released an "Agentic Knowledge Graph Construction" course, indicating the pattern is maturing into standard practice.

### Patterns for Dynamic Construction

1. **Conversation-to-graph**: Agent conversations are parsed into entity-relationship triples and merged into an evolving KG (Graphiti pattern)
2. **Tool-output integration**: Structured data from tool calls (APIs, databases) is ingested as graph nodes/edges
3. **Conflict resolution**: Temporal metadata determines which facts supersede others; agents can handle contradictions
4. **Schema evolution**: New entity types and relationship types emerge organically as the agent encounters new domains

---

## 6. MCP Relevance

### Current MCP Knowledge Graph Implementations

Several KG MCP servers already exist:

**Official/Reference:**
- **Anthropic Memory Server**: KG-based persistent memory using a local JSON graph. Stores entities, observations, and relationships. Reference implementation in the [MCP servers repo](https://github.com/modelcontextprotocol/servers).

**Neo4j MCP Servers** (neo4j-contrib/mcp-neo4j):
- **mcp-neo4j-cypher**: Exposes graph schema to LLM, enables Cypher query generation for read/write
- **mcp-neo4j-memory**: Stores entities with observations and relationships; supports search and subgraph retrieval
- **mcp-neo4j-data-modeler**: Schema design, validation, import/export, code generation

**FalkorDB + Graphiti MCP**: Docker-deployable MCP server that combines Graphiti's temporal KG engine with FalkorDB's low-latency graph database. Conversations become persistent, queryable knowledge graphs.

**Memgraph MCP Server**: Lightweight MCP server for the Memgraph in-memory graph database.

**CodeNexus MCP**: MCP interface for knowledge graphs representing software codebases.

### What a KG MCP Server Could Look Like (for mcpkit)

MCP's resource/tool/prompt primitives map naturally to KG operations:

**Resources** (read-only graph data):
- `kg://entities/{type}` — List entities by type
- `kg://entity/{id}` — Entity with properties and relationships
- `kg://subgraph/{id}?depth=2` — Local neighborhood subgraph
- `kg://communities` — Hierarchical community summaries
- `kg://schema` — Current ontology (entity types, relationship types)

**Tools** (graph mutations and queries):
- `kg_query` — Execute graph queries (Cypher/SPARQL/custom DSL)
- `kg_add_entity` — Create entity with properties
- `kg_add_relationship` — Create typed relationship between entities
- `kg_merge` — Upsert entity/relationship with conflict resolution
- `kg_search` — Hybrid search (semantic + keyword + traversal)
- `kg_ingest` — Parse unstructured text into graph triples
- `kg_temporal_query` — Query facts valid at a specific point in time

**Prompts** (KG-aware prompt templates):
- `kg_context` — Generate context window from relevant subgraph
- `kg_summarize` — Summarize a community or subgraph region
- `kg_reason` — Multi-hop reasoning over graph paths

**Integration with mcpkit packages:**
- `registry` — Tool/resource registration for KG operations
- `resources` — URI-based access to graph entities and subgraphs
- `memory` — KG as the backing store for agent memory (replacing or augmenting current pluggable backends)
- `ralph` — Loop runner could build/query KG during iterative execution
- `sampling` — Use KG context to inform sampling requests

### Key Design Decisions

1. **Graph database agnostic**: Abstract over Neo4j, FalkorDB, Memgraph, etc. via a driver interface
2. **Temporal by default**: Follow Graphiti's bi-temporal model for agent memory use cases
3. **Schema flexibility**: Support both prescribed ontologies and emergent/learned schemas
4. **Hybrid retrieval**: Combine vector similarity (for entry point discovery) with graph traversal (for relational context)
5. **Incremental updates**: Never require full graph recomputation; merge new facts with conflict resolution

---

## 7. Graph Neural Networks + LLMs

### Research Landscape (2024-2025)

GNNs and LLMs address complementary aspects of graph understanding:
- **GNNs** excel at learning structural patterns through message passing
- **LLMs** excel at semantic understanding and zero-shot generalization
- Neither alone solves the full problem

### Key Papers and Models

**GOFA: Generative One-For-All Model** (ICLR 2025)
- Interleaves randomly initialized GNN layers into a frozen pre-trained LLM
- Combines structural modeling (GNN) with semantic modeling (LLM) organically
- Pre-trained on graph-level next-word prediction, QA, structural understanding, and information retrieval
- Demonstrates strong zero-shot ability on unseen downstream datasets
- Paper: [arxiv.org/abs/2407.09709](https://arxiv.org/abs/2407.09709)

**LLMs as Zero-shot Graph Learners** (NeurIPS 2024)
- Aligns GNN representations with LLM token embeddings
- Enables LLMs to process graph-structured data without graph-specific training
- Paper: [NeurIPS 2024 proceedings](https://proceedings.neurips.cc/paper_files/paper/2024/hash/0b77d3a82b59e9d9899370b378087faf-Abstract-Conference.html)

**RAGraph: Retrieval-Augmented Graph Learning** (NeurIPS 2024)
- General framework applying retrieval-augmented generation principles to graph learning tasks
- Paper: [NeurIPS 2024 proceedings](https://proceedings.neurips.cc/paper_files/paper/2024/file/34d6c7090bc5af0b96aeaf92fa074899-Paper-Conference.pdf)

### Integration Approaches

| Approach | Method | Trade-off |
|----------|--------|-----------|
| **LLM as feature extractor** | LLM generates text features; GNN processes graph structure | Simple but disconnected |
| **GNN as graph encoder** | GNN output tokens injected into LLM sequence | Graph-aware LLM but requires alignment training |
| **Interleaved layers** (GOFA) | GNN layers interleaved within LLM transformer | Best integration but most complex |
| **LLM as graph reasoner** | Prompt LLM to write/execute code for graph problems | Reduces hallucination on large graphs; no GNN needed |
| **Explanation as features** | LLM generates text explanations; these become GNN node features | Novel; leverages LLM reasoning as structured input |

### Relevance to Agent Systems

For MCP-based agent systems, the practical takeaway is:
- **GNN+LLM fusion models are not yet production-ready** for general agent use — they require specialized training and hardware
- **Graph-structured prompting** (representing KG subgraphs as text for LLM consumption) is the pragmatic approach today
- **Code-based graph reasoning** (LLM writes Cypher/SPARQL queries) is the most reliable pattern for production agents
- **Retrieval-augmented graph learning** (RAGraph) suggests a future where agent memory retrieval could benefit from learned graph representations

### Curated Resources
- [Awesome-Graph-LLM](https://github.com/XiaoxinHe/Awesome-Graph-LLM) — collection of graph-related LLM research
- [Awesome-Foundation-Models-on-Graphs](https://github.com/Zehong-Wang/Awesome-Foundation-Models-on-Graphs) — graph foundation model papers, codes, datasets
- [Awesome-GraphRAG](https://github.com/DEEP-PolyU/Awesome-GraphRAG) — curated GraphRAG resources

---

## 8. Summary: Key Takeaways for Agent Systems

1. **Property graphs dominate** over RDF for AI agent use cases due to flexibility, performance, and developer ergonomics.

2. **Hybrid vector+graph retrieval** is the industry consensus for 2025+ agent knowledge bases. Neither approach alone is sufficient.

3. **Microsoft GraphRAG** proved that graph-based community detection + hierarchical summarization dramatically outperforms naive RAG for global queries. **LazyGraphRAG** (2025) made this practical by eliminating expensive up-front indexing.

4. **Graphiti/Zep** is the most significant innovation for agent memory — temporal knowledge graphs that incrementally evolve, with bi-temporal tracking and P95 retrieval at 300ms without LLM calls.

5. **MCP is a natural interface for knowledge graphs**. Multiple implementations already exist (Neo4j, FalkorDB, Memgraph, Anthropic Memory Server). The resource/tool/prompt primitives map cleanly to graph read/write/query operations.

6. **Dynamic KG construction is production-viable** — agents can build and update knowledge graphs during execution using patterns like conversation-to-graph, tool-output integration, and temporal conflict resolution.

7. **GNN+LLM integration** is advancing rapidly in research (GOFA at ICLR 2025, RAGraph at NeurIPS 2024) but the pragmatic production pattern remains LLM-generated graph queries (Cypher/SPARQL) rather than end-to-end neural graph reasoning.

8. **KG adoption is still early**: only 27% of AI adopters had KGs in production by late 2025. Implementation complexity remains the primary barrier, which standards like MCP could help reduce.

---

## Sources

### Microsoft GraphRAG
- [From Local to Global: A GraphRAG Approach (arXiv)](https://arxiv.org/abs/2404.16130)
- [Project GraphRAG - Microsoft Research](https://www.microsoft.com/en-us/research/project/graphrag/)
- [LazyGraphRAG announcement](https://www.microsoft.com/en-us/research/blog/lazygraphrag-setting-a-new-standard-for-quality-and-cost/)
- [DRIFT Search introduction](https://www.microsoft.com/en-us/research/blog/introducing-drift-search-combining-global-and-local-search-methods-to-improve-quality-and-efficiency/)
- [GraphRAG improving global search](https://www.microsoft.com/en-us/research/blog/graphrag-improving-global-search-via-dynamic-community-selection/)

### KG + LLM Integration
- [LLM-empowered KG construction survey (arXiv)](https://arxiv.org/abs/2510.20345)
- [Survey on augmenting KGs with LLMs (Springer)](https://link.springer.com/article/10.1007/s44163-024-00175-8)
- [KG-LLM-Papers repository](https://github.com/zjukg/KG-LLM-Papers)
- [LLM-TEXT2KG 2025 Workshop](https://aiisc.ai/text2kg2025/)

### Agent Memory / Dynamic KG
- [Zep: Temporal KG Architecture for Agent Memory (arXiv)](https://arxiv.org/abs/2501.13956)
- [Graphiti repository](https://github.com/getzep/graphiti)
- [Graphs Meet AI Agents: Taxonomy (arXiv)](https://arxiv.org/html/2506.18019v1)

### MCP + Knowledge Graphs
- [Neo4j MCP integrations](https://neo4j.com/developer/genai-ecosystem/model-context-protocol-mcp/)
- [mcp-neo4j repository](https://github.com/neo4j-contrib/mcp-neo4j)
- [FalkorDB MCP + Graphiti integration](https://www.falkordb.com/blog/mcp-knowledge-graph-graphiti-falkordb/)
- [Memgraph MCP Server](https://memgraph.com/blog/introducing-memgraph-mcp-server)
- [MCP servers repository](https://github.com/modelcontextprotocol/servers)

### Graph Databases
- [Neo4j: RDF vs Property Graphs](https://neo4j.com/blog/knowledge-graph/rdf-vs-property-graphs-knowledge-graphs/)
- [Benchmarking Neo4j vs Neptune vs ArangoDB (ResearchGate)](https://www.researchgate.net/publication/389357088_Benchmarking_Graph_Databases_Neo4j_vs_Amazon_Neptune_vs_ArangoDB)
- [FalkorDB repository](https://github.com/FalkorDB/FalkorDB)
- [Amazon Neptune Graph and AI](https://aws.amazon.com/neptune/graph-and-ai/)

### GNN + LLM Research
- [GOFA: Generative One-For-All Model (ICLR 2025)](https://arxiv.org/abs/2407.09709)
- [LLMs as Zero-shot Graph Learners (NeurIPS 2024)](https://proceedings.neurips.cc/paper_files/paper/2024/hash/0b77d3a82b59e9d9899370b378087faf-Abstract-Conference.html)
- [RAGraph: Retrieval-Augmented Graph Learning (NeurIPS 2024)](https://proceedings.neurips.cc/paper_files/paper/2024/file/34d6c7090bc5af0b96aeaf92fa074899-Paper-Conference.pdf)
- [Awesome-Graph-LLM repository](https://github.com/XiaoxinHe/Awesome-Graph-LLM)
- [Awesome-GraphRAG repository](https://github.com/DEEP-PolyU/Awesome-GraphRAG)

### Agent Knowledge Bases
- [Vector Databases vs Graph RAG for Agent Memory](https://machinelearningmastery.com/vector-databases-vs-graph-rag-for-agent-memory-when-to-use-which/)
- [Neo4j: KG vs Vector RAG Benchmarking](https://neo4j.com/blog/developer/knowledge-graph-vs-vector-rag/)
- [LightRAG repository (EMNLP 2025)](https://github.com/HKUDS/LightRAG)
