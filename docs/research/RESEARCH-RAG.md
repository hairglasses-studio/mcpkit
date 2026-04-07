# RAG Research: Patterns for AI Agent Systems

Research date: 2026-03-14

---

## 1. RAG Architectures

RAG has evolved through three major paradigms, each building on the limitations of the previous.

### Naive RAG

The baseline "retrieve-then-read" pipeline:
1. **Indexing** -- Split documents into chunks, embed them, store in vector DB
2. **Retrieval** -- Embed user query, find top-k similar chunks via ANN search
3. **Generation** -- Concatenate retrieved chunks into prompt, generate answer

Limitations: low precision (irrelevant chunks pollute context), low recall (misses relevant info with simple similarity), no query understanding, no answer verification.

### Advanced RAG

Adds pre-retrieval and post-retrieval optimization stages:

**Pre-retrieval:**
- Query rewriting / expansion (HyDE -- hypothetical document embeddings)
- Query decomposition (break complex queries into sub-queries)
- Query routing (direct queries to appropriate indexes)

**Post-retrieval:**
- Re-ranking (cross-encoder models score query-document relevance)
- Compression (extract only relevant sentences from retrieved chunks)
- Fusion (reciprocal rank fusion across multiple retrieval strategies)

### Modular RAG

The current state-of-the-art treats RAG as a system of composable modules rather than a linear pipeline. Components include Retrievers, Generators, Evaluators, and Routers, orchestrated by an AI agent that decides which modules to invoke and in what order. This is the architecture that maps most naturally to MCP tool composition.

### Graph RAG (Microsoft)

Combines knowledge graphs with LLM-based summarization:

1. **Entity extraction** -- LLM extracts entities and relationships from source documents
2. **Graph construction** -- Build knowledge graph from extracted triples
3. **Community detection** -- Leiden algorithm partitions graph into hierarchical communities
4. **Community summarization** -- LLM generates summaries for each community at multiple granularity levels
5. **Query processing** -- Three search modes:
   - **Local Search** -- Fan out from specific entities to neighbors and associated concepts
   - **Global Search** -- Leverage community summaries for holistic corpus-level questions
   - **DRIFT Search** -- Local search augmented with community context

Performance: 72-83% higher comprehensiveness and 62-82% higher diversity vs traditional RAG, while requiring up to 97% fewer tokens for root-level summaries.

Reference: [From Local to Global: A Graph RAG Approach to Query-Focused Summarization](https://arxiv.org/abs/2404.16130)

### Key Components

| Component | Purpose | Options |
|-----------|---------|---------|
| Chunking | Split documents into retrievable units | Fixed-size, recursive, semantic, late chunking, contextual |
| Embedding | Convert text to dense vectors | OpenAI text-embedding-3, Voyage-3-large, Cohere embed-v3, BGE |
| Retrieval | Find relevant chunks | Dense (ANN), sparse (BM25), hybrid |
| Reranking | Improve precision of retrieved set | Cross-encoders (Cohere Rerank, BGE-reranker), ColBERT |
| Generation | Produce answer from context | Any LLM with retrieved context in prompt |

---

## 2. Agentic RAG

Agentic RAG incorporates LLM-based decision-making into the retrieval pipeline, allowing the system to self-correct, route, and adapt its retrieval strategy.

### Self-RAG

The LLM learns to self-reflect on its own generation:
- Generates **retrieval tokens** deciding whether retrieval is needed
- Generates **relevance tokens** assessing if retrieved passages are relevant
- Generates **support tokens** checking if the response is grounded in evidence
- Generates **utility tokens** rating overall response quality

The model is trained with these special tokens via instruction tuning on LLaMA-2 (7B). This makes retrieval adaptive -- the model only retrieves when it determines it needs external knowledge.

### Corrective RAG (CRAG)

Adds a lightweight retrieval evaluator (0.77B parameters) that scores document quality and triggers corrective actions:

- **Correct** (high confidence) -- Refine retrieved documents using decompose-then-recompose: segment into "knowledge strips," score each strip, filter irrelevant ones
- **Incorrect** (low confidence) -- Discard faulty retrievals, trigger web search for more reliable information
- **Ambiguous** (unclear confidence) -- Combine refined retrieval with web search results

CRAG is plug-and-play and can be coupled with any RAG-based approach.

Reference: [Corrective Retrieval Augmented Generation](https://arxiv.org/abs/2401.15884)

### Adaptive RAG

Routes queries to different processing pipelines based on query complexity:
- Simple factual queries -> direct LLM response (no retrieval)
- Moderate queries -> single-step RAG
- Complex queries -> multi-step retrieval with iterative refinement

Implemented in LangGraph as a tutorial combining query analysis, routing, and self-correction.

### Framework Implementations

**LlamaIndex Agentic RAG patterns:**
- **RouterQueryEngine** -- Routes queries to different indexes (e.g., summary index vs vector store) based on query intent. Simplest form of agentic RAG.
- **SubQuestionQueryEngine** -- Decomposes complex queries into sub-questions, routes each to the appropriate tool/index, synthesizes answers.
- **Per-document agents** -- Create an agent per document with search + summarize capabilities. A meta-agent performs tool retrieval over document agents using embeddings.
- **Composite Retrieval API** -- Lightweight agent layer with named/described sub-indexes for routing.

**LangGraph/LangChain patterns:**
- Routing-based architectures where multiple agents collaborate
- Function calling for tool-use RAG
- SQL generation for structured data retrieval
- Adaptive RAG with self-correction loops

**Haystack:**
- Pipeline-based architecture with composable components
- Built-in support for hybrid retrieval and re-ranking

---

## 3. RAG for MCP Servers

### MCP vs RAG: Complementary, Not Competing

RAG is about **knowing more** -- adding static information to a model's context. MCP is about **doing more** -- connecting models to live systems and tools. They are complementary:

- RAG excels at grounding responses in static, unstructured knowledge
- MCP enables secure, real-time access to structured, dynamic data
- Combined, they create a synergistic architecture for AI systems

### MCP Resources as RAG Data Sources

MCP Resources are read-only, addressable data entities exposed by servers (files, database records, API responses). Unlike tools (what the AI can do), resources represent what the AI should know. This maps directly to RAG's retrieval layer:

- Resources with `text/plain` or `text/markdown` content types are natural RAG source documents
- Resource URIs provide stable addressing for retrieved content
- Resource templates enable parameterized retrieval (e.g., `docs://{topic}/{section}`)
- Resource subscriptions enable real-time index updates when source data changes

### RAG-as-a-Tool Interface Design

A RAG MCP tool interface could expose:

```
Tool: "search_knowledge_base"
  Params:
    query: string          -- Natural language query
    collection: string     -- Which knowledge base to search
    top_k: int             -- Number of results (default: 5)
    filters: object        -- Metadata filters (date, source, category)
    search_type: enum      -- "dense" | "sparse" | "hybrid"
    rerank: bool           -- Whether to apply reranking
  Returns:
    results: []            -- Array of {content, score, metadata, source_uri}
```

Additional tools for a full RAG toolkit:
- `ingest_document` -- Add documents to the knowledge base (chunking + embedding)
- `list_collections` -- Discover available knowledge bases
- `get_chunk_context` -- Retrieve surrounding chunks for a given chunk ID

### RAG-MCP: Tool Selection via Retrieval

An emerging pattern (RAG-MCP) uses RAG to solve tool selection at scale. With 4,400+ MCP servers on mcp.so, an LLM cannot evaluate all tools in-context. RAG-MCP offloads tool discovery by using semantic retrieval to identify the most relevant MCP server(s) for a given query from an external index before engaging the LLM.

References:
- [RAG-MCP: Mitigating Prompt Bloat in LLM Tool Selection via Retrieval-Augmented Generation](https://arxiv.org/html/2505.03275v1)
- [Integrating Agentic RAG with MCP Servers](https://becomingahacker.org/integrating-agentic-rag-with-mcp-servers-technical-implementation-guide-1aba8fd4e442)

---

## 4. Vector Stores and Embeddings

### Embedding Models (2025-2026)

| Model | Dimensions | MTEB Score | Cost (per 1M tokens) | Notes |
|-------|-----------|------------|----------------------|-------|
| Voyage-3-large | 1024-2048 | Best overall | $0.06 | 9.7% better than OpenAI on 100 datasets |
| OpenAI text-embedding-3-large | 3072 (flexible) | Strong | $0.13 | Matryoshka dimensionality reduction |
| OpenAI text-embedding-3-small | 1536 | Good | $0.02 | Best budget option from OpenAI |
| Cohere embed-v3 | 1024 | Good | ~$0.10 | Strong multilingual support |
| Mistral-embed | 1024 | 77.8% accuracy | Moderate | Highest accuracy in some benchmarks |
| Voyage-3.5-lite | 512-1024 | 66.1% | Low | Best accuracy-cost balance for production |
| BGE-large-en-v1.5 | 1024 | Good | Free (OSS) | Best open-source option |

**Key finding:** Statistical tests show no significant difference within the same vendor at different dimensions (e.g., OpenAI 1536d vs 512d), but significant differences across vendors.

**Matryoshka embeddings:** Both OpenAI and Voyage support Matryoshka learning, allowing dimension reduction (e.g., 3072 -> 256) with minimal quality loss, dramatically reducing storage costs.

### Vector Databases

| Database | Type | Hybrid Search | Best For | Pricing |
|----------|------|--------------|----------|---------|
| **Qdrant** | Purpose-built | Yes (native) | Real-time search, rich payload filtering | 1GB free forever |
| **Weaviate** | Purpose-built | Yes (best-in-class) | AI-native apps, multi-modal | Cloud + OSS |
| **Pinecone** | Managed SaaS | Yes | Enterprise, low-latency at scale | Usage-based + free tier |
| **Milvus** | Purpose-built | Yes | 100M+ vectors, self-hosted | OSS (Zilliz Cloud managed) |
| **pgvector/pgvectorscale** | Postgres extension | Via custom queries | Teams already on Postgres, <100M vectors | Infrastructure cost only |
| **Chroma** | Embedded | No | Prototyping, local development | OSS |
| **Turbopuffer** | Managed | Yes | Cost-efficient at scale | From $64/mo |

**pgvector performance surprise:** pgvectorscale achieves 471 QPS at 99% recall on 50M vectors, significantly outperforming Qdrant (41.47 QPS) at that scale. Competitive with dedicated DBs under 100M vectors.

### Hybrid Search (BM25 + Dense)

Hybrid search combines sparse keyword matching (BM25) with dense vector similarity in a single query. This improves recall for most RAG workloads because:
- BM25 catches exact keyword matches that embedding similarity misses
- Dense retrieval captures semantic meaning that keywords miss
- Fusion methods (Reciprocal Rank Fusion, RelativeScoreFusion) combine results

Weaviate's implementation runs BM25 and vector searches in parallel with `relativeScoreFusion`, preserving nuances of both search metrics. Qdrant and Elasticsearch also support native hybrid search.

---

## 5. Evaluation

### RAGAS Framework

RAGAS (Retrieval Augmented Generation Assessment) provides reference-free evaluation. Core metrics:

| Metric | Measures | Range | Component |
|--------|----------|-------|-----------|
| **Faithfulness** | Are all claims in the answer supported by retrieved context? | 0-1 | Generation quality |
| **Answer Relevancy** | Is the generated answer relevant to the question? | 0-1 | Generation quality |
| **Context Precision** | Are relevant chunks ranked higher than irrelevant ones? | 0-1 | Retrieval quality |
| **Context Recall** | Are all ground truth statements covered by retrieved context? | 0-1 | Retrieval quality |

**Composite RAGAS Score** = harmonic mean of the four metrics above.

### Additional Evaluation Dimensions

- **Context Relevancy** -- What fraction of retrieved context is actually relevant to the query?
- **Noise Robustness** -- Does the system handle irrelevant retrieved documents gracefully?
- **Negative Rejection** -- Can the system refuse to answer when context is insufficient?
- **Counterfactual Robustness** -- Can the system detect and resist incorrect information in context?
- **Information Integration** -- Can the system synthesize information from multiple chunks?

### Evaluation Best Practices

1. **Retrieval metrics first** -- Fix retrieval before optimizing generation. Measure recall@k, precision@k, MRR, NDCG.
2. **LLM-as-judge** -- Use a strong LLM (e.g., GPT-4, Claude) to evaluate faithfulness and relevancy when human labels are unavailable.
3. **A/B testing in production** -- Compare RAG configurations on real user queries with human preference ratings.
4. **Domain-specific test sets** -- Build evaluation sets from real user questions with expert-annotated ground truth.
5. **Continuous monitoring** -- Track RAGAS metrics over time as documents and queries change.

Reference: [RAGAS: Automated Evaluation of Retrieval Augmented Generation](https://arxiv.org/abs/2309.15217)

---

## 6. Production Considerations

### Chunking Strategies

| Strategy | Chunk Size | Best For | Notes |
|----------|-----------|----------|-------|
| **Fixed-size** | 256-512 tokens, 10-20% overlap | Starting point | Use RecursiveCharacterTextSplitter |
| **Semantic** | Variable | When document structure varies | Up to 9% recall improvement over fixed-size |
| **Late Chunking** | Full document -> chunk embeddings | Preserving cross-chunk context | Encode full document first, then derive chunk embeddings |
| **Contextual (Anthropic)** | Variable + context prefix | High-accuracy production systems | LLM prepends context summary to each chunk; 35% fewer retrieval failures |
| **Document-aware** | Respect document structure | PDFs, HTML, Markdown | Split on headers, sections, paragraphs |

**Practical guidance:**
- Start with RecursiveCharacterTextSplitter at 400-512 tokens, 10-20% overlap
- Factoid queries: 256-512 token chunks optimal
- Analytical queries: 1024+ token chunks needed
- Move to semantic/contextual chunking only if metrics justify the cost

### Contextual Retrieval (Anthropic)

Anthropic's contextual retrieval prepends an LLM-generated context summary to each chunk before embedding. This captures document-level context (what document is this from, what section, how it relates to surrounding content) in the embedding. Results: 35% average reduction in retrieval failures across multiple domains. Trade-off: requires an LLM call per chunk during indexing.

### Late Chunking

Encodes entire documents through an embedding model first, then derives chunk embeddings from the full-document token embeddings. Chunk embeddings inherit semantic references from their document context without requiring LLM summarization. More computationally efficient than contextual retrieval but requires embedding models that support long-context encoding.

### Metadata Filtering

Essential for production RAG:
- **Source metadata** -- Document title, URL, author, creation date
- **Structural metadata** -- Section headers, page numbers, chunk position
- **Tenant metadata** -- User ID, organization, access level (for multi-tenant)
- **Custom tags** -- Domain, category, language, version

Vector indexes support filtered similarity search, restricting retrieval by source, time range, tenant, or permissions at query time.

### Multi-Tenant RAG

Two architectural approaches:

1. **Shared index with metadata filtering** -- Single vector index, tenant ID as metadata field. Simpler to manage, but requires careful access control and may have performance implications at scale.

2. **Separate index per tenant** -- Isolated knowledge bases per tenant. Allows different chunking strategies, embedding models, and configurations per tenant. Better security isolation but higher operational overhead.

AWS Bedrock Knowledge Bases supports multi-tenant RAG with metadata-based filtering, using a tenancy field at runtime to filter documents belonging to a specific tenant.

### Caching

- **Embedding cache** -- Cache embeddings for repeated documents to avoid re-computation
- **Query cache** -- Cache retrieval results for identical or near-identical queries
- **Semantic cache** -- Use embedding similarity to match cached results for semantically similar queries (e.g., Redis with vector similarity on cached query embeddings)
- **LLM response cache** -- Cache final generated answers keyed by (query, retrieved_context) hash

### Cost Optimization

- **Embedding costs** -- Voyage AI at $0.06/M tokens is 2.2x cheaper than OpenAI and 1.6x cheaper than Cohere
- **Dimension reduction** -- Matryoshka embeddings allow 3072 -> 256 dimensions with minimal quality loss, reducing storage 12x
- **Quantization** -- int8 and binary quantization reduce vector storage costs dramatically
- **RAG vs long-context** -- RAG is roughly two orders of magnitude cheaper than using an LLM's full long-context window
- **Tiered retrieval** -- Use cheap sparse search first, then expensive dense search, then costly reranking only on top candidates
- **Batch embedding** -- Batch document embedding during indexing to reduce API call overhead

---

## 7. Implications for mcpkit

### Potential `rag/` Package Design

Based on this research, a RAG package for mcpkit could provide:

1. **Document ingestion pipeline** -- Pluggable chunking strategies (fixed, semantic, contextual), metadata extraction, embedding via configurable providers
2. **Vector store interface** -- Abstract interface over Qdrant/Weaviate/pgvector/Chroma with hybrid search support
3. **Retrieval middleware** -- mcpkit middleware that automatically retrieves relevant context before tool execution
4. **RAG tool registration** -- Helper to register search/ingest/list tools following the MCP tool pattern
5. **Evaluation hooks** -- RAGAS-style metric computation for monitoring retrieval quality
6. **Multi-tenant support** -- Metadata-based filtering with tenant context from MCP auth

### MCP Resources Integration

The MCP resources protocol is a natural fit for RAG source data:
- Resource subscriptions can trigger re-indexing when source documents change
- Resource URIs can serve as chunk provenance tracking
- Resource templates can parameterize retrieval (e.g., `knowledge://{collection}/{query}`)

### RAG-MCP for Tool Discovery

As mcpkit's tool registry grows, RAG-based tool selection (the RAG-MCP pattern) could help scale beyond what fits in an LLM's context window. This aligns with the `gateway/` package's multi-server aggregation use case.

---

## Sources

### Surveys and Papers
- [Retrieval-Augmented Generation for Large Language Models: A Survey](https://arxiv.org/abs/2312.10997)
- [Agentic Retrieval-Augmented Generation: A Survey on Agentic RAG](https://arxiv.org/html/2501.09136v1)
- [Graph-Enhanced RAG: A Survey of Methods, Architectures, and Performance](https://www.researchgate.net/publication/393193258_Graph-Enhanced_RAG_A_Survey_of_Methods_Architectures_and_Performance)
- [Engineering the RAG Stack: Architecture and Trust Frameworks](https://arxiv.org/html/2601.05264v1)
- [From Local to Global: A Graph RAG Approach (Microsoft GraphRAG)](https://arxiv.org/abs/2404.16130)
- [Corrective Retrieval Augmented Generation (CRAG)](https://arxiv.org/abs/2401.15884)
- [RAGAS: Automated Evaluation of Retrieval Augmented Generation](https://arxiv.org/abs/2309.15217)
- [Late Chunking: Contextual Chunk Embeddings](https://arxiv.org/pdf/2409.04701)
- [RAG-MCP: Mitigating Prompt Bloat in LLM Tool Selection](https://arxiv.org/html/2505.03275v1)
- [Reconstructing Context: Evaluating Advanced Chunking Strategies](https://arxiv.org/abs/2504.19754)

### Framework Documentation
- [LlamaIndex: Agentic Retrieval Guide](https://www.llamaindex.ai/blog/rag-is-dead-long-live-agentic-retrieval)
- [LlamaIndex: Agentic RAG Architecture Guide](https://www.llamaindex.ai/blog/agentic-rag-with-llamaindex-2721b8a49ff6)
- [LangGraph: Adaptive RAG Tutorial](https://langchain-ai.github.io/langgraph/tutorials/rag/langgraph_adaptive_rag/)
- [LangGraph: Self-Reflective RAG](https://blog.langchain.com/agentic-rag-with-langgraph/)
- [LangChain: Build a Custom RAG Agent](https://docs.langchain.com/oss/python/langgraph/agentic-rag)
- [Microsoft GraphRAG](https://microsoft.github.io/graphrag/)
- [RAGAS Metrics Documentation](https://docs.ragas.io/en/stable/concepts/metrics/available_metrics/)

### MCP + RAG Integration
- [MCP for RAG and Agentic AI](https://medium.com/@tam.tamanna18/model-context-protocol-mcp-for-retrieval-augmented-generation-rag-and-agentic-ai-6f9b4616d36e)
- [How To Build RAG Applications Using MCP](https://thenewstack.io/how-to-build-rag-applications-using-model-context-protocol/)
- [How MCP Fits into RAG Workflows (Milvus)](https://milvus.io/ai-quick-reference/how-does-model-context-protocol-mcp-fit-into-retrievalaugmented-generation-rag-workflows)
- [Integrating Agentic RAG with MCP Servers](https://becomingahacker.org/integrating-agentic-rag-with-mcp-servers-technical-implementation-guide-1aba8fd4e442)

### Production and Embeddings
- [Anthropic: Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval)
- [Best Chunking Strategies for RAG 2025](https://www.firecrawl.dev/blog/best-chunking-strategies-rag)
- [Document Chunking for RAG: 9 Strategies Tested](https://langcopilot.com/posts/2025-10-11-document-chunking-for-rag-practical-guide)
- [Best Vector Databases in 2025](https://www.firecrawl.dev/blog/best-vector-databases)
- [Vector Database Comparison: Pinecone vs Weaviate vs Qdrant vs FAISS vs Milvus vs Chroma](https://liquidmetal.ai/casesAndBlogs/vector-comparison/)
- [Voyage-3-large Announcement](https://blog.voyageai.com/2025/01/07/voyage-3-large/)
- [Native RAG vs Advanced RAG vs Modular RAG (Zilliz)](https://zilliz.com/blog/advancing-llms-native-advanced-modular-rag-approaches)
- [RAG in 2025: 7 Proven Strategies to Deploy at Scale](https://www.morphik.ai/blog/retrieval-augmented-generation-strategies)
