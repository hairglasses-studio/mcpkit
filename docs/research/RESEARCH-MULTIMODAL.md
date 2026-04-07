# Multi-Modal Capabilities and Patterns for AI Agent Systems

Research compiled March 2026. Covers current state of multi-modal AI models, agent frameworks, and implications for MCP toolkit development.

---

## 1. Multi-Modal Models — Current Landscape

### Claude 4 Family (Anthropic)
- **Input**: Text, images (base64 or URL). No native audio or video input.
- **Output**: Text only. No image, audio, or video generation.
- **Vision**: Strong analytical depth for document analysis, chart reading, diagram understanding. Claude Sonnet 4 (May 2025) added refined computer-use capabilities.
- **Limitation**: Audio requires external transcription (e.g., Whisper). No native video processing. Anthropic's strength is analytical reasoning over visual inputs rather than generative multi-modal output.

### GPT-5 / GPT-4o (OpenAI)
- **Input**: Text, images, audio, video frames, PDFs. Natively multimodal — trained from scratch on multiple modalities simultaneously.
- **Output**: Text, images (GPT Image 1 replaced DALL-E 3, March 2025), audio (native speech synthesis).
- **Key advance**: GPT-5 (August 2025) made multimodality a first-class citizen. Sub-150ms latencies for real-time interactive use. Excels across visual, video-based, spatial, and scientific reasoning benchmarks.
- **Voice**: GPT-4o pioneered speech-to-speech with 320ms average latency. `gpt-realtime` model is the latest dedicated voice model.

### Gemini 2.0 / 2.5 (Google)
- **Input**: Text, images, audio, video (up to hours of content), code. Broadest native multi-modal input support.
- **Output**: Text, images, audio (native). Native image and audio output introduced with Gemini 2.0.
- **Key advance**: Gemini 2.5 Flash processes raw audio natively through a single low-latency model, eliminating the traditional STT-LLM-TTS pipeline. Hour-long video processing capability.
- **Project Astra**: Research prototype for universal AI assistant with real-time multimodal interaction. Powers experiences in Search, Gemini app, and third-party developer products (announced Google I/O 2025). Smart glasses integration expected 2026.

### Comparative Summary

| Capability | Claude 4 | GPT-5 | Gemini 2.5 |
|---|---|---|---|
| Text input/output | Yes/Yes | Yes/Yes | Yes/Yes |
| Image input | Yes | Yes | Yes |
| Image output | No | Yes | Yes |
| Audio input | No (transcribe first) | Yes (native) | Yes (native) |
| Audio output | No | Yes (native) | Yes (native) |
| Video input | No | Yes (frames) | Yes (native, hours) |
| Video output | No | No | No |
| PDF input | Yes (100 pages) | Yes | Yes |

---

## 2. Image/Vision Tools for Agents

### Screenshot Analysis and Visual QA
- Claude's vision analyzes images passed as base64-encoded data or URLs. Strong at document analysis, chart interpretation, and visual reasoning.
- GPT-4o/GPT-5 support visual question answering with high accuracy on complex scenes.
- All major models support multi-image inputs for comparative analysis.

### OCR and Document Understanding
- **Mistral OCR**: Purpose-built for document understanding — processes text, tables, and images while maintaining structure and hierarchy.
- **Claude PDF Support**: Processes PDFs up to 100 pages, analyzing both text and visual elements (charts, graphics).
- **NVIDIA NeMo Retriever**: Multi-stage pipeline using object detection to locate specific elements (charts, tables), then applies specialized OCR and structure-aware models per element type.

### Agent Framework Patterns
- Image inputs are typically passed as content blocks alongside text in conversation turns.
- Base64 encoding is the universal interchange format for images in API calls.
- Frameworks like LangChain and LlamaIndex provide image-aware tool abstractions that route visual content to vision-capable models.
- Accessibility tree snapshots (used by Playwright MCP) offer a non-visual alternative: 2-5KB text vs 100KB+ screenshots.

---

## 3. Audio/Voice Agents

### Architecture Paradigms

**Speech-to-Speech (S2S) — Single Model:**
- OpenAI Realtime API and Gemini Live API both process audio end-to-end in a single model.
- Eliminates STT-LLM-TTS pipeline latency. Preserves speech nuance (tone, emotion, background noise).
- OpenAI `gpt-realtime`: Best-in-class for instruction following and tool calling with natural speech output.
- Gemini 2.5 Flash Native Audio: Processes raw audio natively, supports 24 languages, affective dialog.

**Chained Pipeline — Multiple Models:**
- Audio -> STT (Whisper) -> LLM -> TTS pipeline.
- More modular and debuggable. Each component can be independently upgraded.
- Higher latency but offers more control over each stage.

### Transport Protocols
- **WebRTC**: Peer-to-peer, low-latency audio/video. Used by OpenAI Realtime API.
- **WebSocket**: Common protocol for real-time data transfer. Used by both OpenAI and Google.
- **SIP**: OpenAI Realtime API supports phone calling through Session Initiation Protocol.

### Key Capabilities
- **Barge-in/Interruption**: Both OpenAI and Google support users interrupting the model mid-speech.
- **Tool Calling**: Voice agents can invoke tools during conversation. OpenAI Realtime API now supports remote MCP servers.
- **Affective Dialog**: Gemini adapts response style and tone to match user expression.
- **Image + Voice**: OpenAI Realtime API supports image inputs alongside audio, enabling visual-conversational agents.

### Developer Frameworks
- **OpenAI Agents SDK (TypeScript)**: Recommended for building voice agents on Realtime API.
- **Google ADK (Agent Development Kit)**: Production-ready framework for bidi-streaming with Gemini. Provides `LiveRequestQueue` for unified text/audio/control message handling, plus session management and tool orchestration abstractions.

---

## 4. Document Processing

### Current Capabilities
- **Multi-modal PDF analysis**: Claude processes 100-page PDFs analyzing text and visual elements. GPT-5 and Gemini handle PDFs as native input.
- **Table extraction**: Specialized models detect table structures, cell boundaries, and hierarchical relationships. NVIDIA's approach uses object detection + specialized OCR per element type.
- **Chart understanding**: Vision models can interpret bar charts, line graphs, pie charts, and scatter plots — extracting data points and trends.

### Agentic Document Extraction
- Moving from static OCR to agentic workflows where AI agents iteratively process documents, ask clarifying questions, and self-correct extraction errors.
- 66% of enterprises replacing legacy document processing with AI-powered solutions (SER Group IDP Survey 2025).

### Key Tools
| Tool | Capability |
|---|---|
| Claude Vision | PDF analysis, chart/table understanding, visual QA |
| Mistral OCR | Structure-preserving text/table/image extraction |
| NVIDIA NeMo Retriever | Multi-stage pipeline with element-specific models |
| LlamaIndex Document AI | Agentic OCR with iterative extraction workflows |
| Klippa DocHorizon | Enterprise document processing with agentic OCR |

### Patterns for Agent Integration
- Document is loaded as a resource (MCP resource or tool input).
- Vision model extracts structured data (JSON schema-validated output).
- Iterative refinement: agent re-examines unclear sections with targeted prompts.
- Multi-page documents processed with sliding window or chunked approaches.

---

## 5. Code + Visual — Artifact Generation

### Generative UI Pattern
The most significant emerging pattern: AI agents generate the interface itself on the fly. Users describe goals, and agents produce interactive dashboards, charts, or UI components.

### Key Platforms and Approaches
- **Vercel v0**: Generates actual React code from natural language descriptions. Powers rapid web prototyping.
- **Google Stitch** (I/O 2025): Uses Gemini 2.5 Pro to convert text prompts or uploaded images into UI designs and frontend code.
- **A2UI (Google)**: Lightweight protocol for cross-platform declarative UI. Designed for multi-agent systems where safe rendering from untrusted agents is required.
- **CopilotKit**: Framework for building generative UI with agent-powered interfaces.

### Diagram and Visualization Rendering
- **Mermaid**: Text-based diagramming language (flowcharts, sequence diagrams, Gantt charts). First-class Markdown citizen — AI agents can generate diagrams as text that renders visually.
- **Chart.js / D3 code generation**: Agents generate chart code that renders in sandboxed environments.
- **SVG generation**: Direct SVG output from models for simple diagrams and illustrations.

### Artifact Patterns
1. **Code artifact**: Agent generates complete runnable code (React component, HTML page, Python script).
2. **Declarative artifact**: Agent produces structured description (JSON, YAML, Mermaid) that a renderer interprets.
3. **Image artifact**: Model directly generates an image (GPT Image 1, Gemini image output).
4. **Hybrid artifact**: Structured data + rendering instructions + preview image.

---

## 6. MCP and Multi-Modal Content

### Current MCP Support (2025-06-18 Specification)

**Content Types Supported:**
- `TextContent`: Plain text with optional MIME type.
- `ImageContent`: Base64-encoded image data with MIME type (e.g., `image/png`, `image/jpeg`).
- `AudioContent`: Base64-encoded audio data with MIME type. **Added in 2025-06-18 spec** — servers and clients must handle audio alongside text and image.
- `ResourceContent`: Embedded resources with URI references.

**Resources:**
- Resources are identified by unique URIs.
- Can contain text or binary (blob) data.
- Binary resources use base64 encoding with MIME type specification.
- Can serve images, audio files, or other binary formats.

**Structured Content (2025-06-18):**
- Tool results can include `structuredContent` field with JSON schema validation.
- For backward compatibility, structured results should also include serialized JSON in a TextContent block.
- Output schema validation ensures type safety for tool results.

**Annotations:**
- All content blocks support annotations for metadata: audience targeting, priority levels, modification timestamps.

### What is Missing for Full Multi-Modal Support

1. **Video content type**: No native `VideoContent` block. Video must be handled as binary resources or external URLs.
2. **Streaming binary data**: Base64 encoding adds ~33% overhead. No streaming binary protocol for large media files.
3. **Native audio I/O for tools**: While `AudioContent` exists in the spec, practical tooling for audio processing (real-time voice, audio analysis) is underdeveloped.
4. **Multi-modal tool schemas**: No standard way to declare that a tool accepts or returns images/audio in its input schema. Tool parameters are JSON-only.
5. **Real-time streaming**: MCP's request-response model does not naturally fit real-time audio/video streaming use cases. WebSocket/WebRTC transport would be needed.
6. **Render hints**: No standard way to indicate how a client should display multi-modal content (inline image vs. download link, audio player vs. transcription).
7. **Content negotiation**: No mechanism for client to declare which content types it supports, potentially leading to unsupported content being sent.

### Implications for mcpkit

The `resources` package already supports MIME types and binary data via base64. Key areas for extension:
- Audio content type support in resource and tool results.
- Content type negotiation middleware.
- Streaming resource support for large binary content.
- Multi-modal tool parameter validation (accepting image/audio inputs).

---

## 7. Computer Use and Browser Automation

### Anthropic Computer Use
- Available via API with beta headers (`computer-use-2025-01-24`, `computer-use-2025-11-24`).
- Operates through continuous feedback loop: analyze screen -> decide action -> observe result.
- Pixel-perfect accuracy: counts pixels from screen edges for exact cursor positioning.
- Works across any screen resolution and application layout.
- Supported on Claude Sonnet 4 and later models.

### Playwright MCP (Microsoft)
- Released March 2025. One of the most widely adopted MCP servers.
- Uses browser's **accessibility tree** instead of screenshots — structured, text-based representation of web pages.
- Accessibility snapshots: 2-5KB vs 100KB+ for screenshots. Dramatically reduces token consumption.
- No vision model needed — operates purely on structured data.
- Supports Claude Desktop, Cursor IDE, Cherry Studio, GitHub Copilot.
- Integrated into GitHub Copilot's Coding Agent for AI-assisted browser automation.

### Emerging Patterns

**Two competing approaches:**

| Approach | How it works | Pros | Cons |
|---|---|---|---|
| **Vision-based** (Anthropic Computer Use) | Screenshots + pixel coordinates | Works with any application, handles canvas/games | High token cost, slower, requires vision model |
| **Accessibility-based** (Playwright MCP) | DOM/accessibility tree text | Fast, cheap, deterministic | Limited to web, misses visual-only elements |

**Hybrid pattern emerging**: Use accessibility tree for standard web interactions, fall back to vision-based approach for visual-only elements (canvas, images, custom renderers).

### Agent Skills (Anthropic, December 2025)
- Open standard for teaching agents **how** to use tools (procedural knowledge).
- Complements MCP which provides **what** tools are available.
- Organized as folders of instructions, scripts, and resources loaded dynamically.
- MCP = plumbing (tool access), Agent Skills = brain (procedural memory).

---

## 8. Video Understanding

### Current State (Early but Advancing Rapidly)

**Model Capabilities:**
- Gemini: Native video understanding up to hours of content. Industry-leading for long-form video.
- GPT-5: Video frame analysis with spatial and temporal reasoning.
- Claude: No native video support. Requires external frame extraction.

### Agentic Video Understanding Frameworks

**Key Research (2025-2026):**

- **EGAgent**: Integrates temporally-annotated entity scene graphs into the tool-calling loop. Each node is annotated with temporal information, allowing incremental construction as new data arrives.
- **VideoExplorer**: Decomposes complex video tasks into sub-questions, adaptively grounds relevant temporal spans, and dynamically adjusts perceptual granularity.
- **VideoDeepResearch**: Uses agentic tool calling for long video understanding.

### Multi-Agent Architectures for Video

Emerging pattern uses specialized agents coordinated through "Agents as Tools":
1. **Frame Extraction Agent**: Selects key frames based on scene changes, motion, or semantic relevance.
2. **Visual Analysis Agent**: Processes individual frames with vision models.
3. **Temporal Reasoning Agent**: Connects observations across time, tracking entities and events.
4. **Summarization Agent**: Synthesizes findings into coherent narrative.

### Core Challenges
1. **Visual token redundancy**: Video frames contain massive redundancy. Smart sampling/compression needed.
2. **Context window limits**: Long videos produce more tokens than any model can process. Requires hierarchical summarization.
3. **Multi-modal temporal reasoning**: Connecting visual observations with audio/text across time spans remains difficult.

### Trajectory
- 2024: Basic frame extraction + vision model per frame.
- 2025: Agentic approaches with temporal awareness and adaptive sampling.
- 2026+: Native end-to-end video understanding (Gemini leading), real-time video analysis agents.

---

## Key Takeaways for mcpkit

### Near-Term Opportunities
1. **Audio content support**: MCP 2025-06-18 added `AudioContent`. Ensure mcpkit resources and tool results handle audio content type.
2. **Content negotiation**: Middleware for clients to declare supported content types.
3. **Playwright MCP patterns**: Accessibility-tree-based browser tools as reference implementation for efficient computer use.
4. **Document processing tools**: PDF/table/chart extraction as tool patterns using vision model capabilities.

### Medium-Term Considerations
5. **Streaming resources**: Large binary content (images, audio, video) needs streaming support beyond base64 encoding.
6. **Multi-modal tool schemas**: Declaring image/audio parameters in tool input schemas.
7. **Voice agent integration**: MCP tools callable from real-time voice sessions (OpenAI Realtime API already supports remote MCP servers).
8. **Generative UI patterns**: Tools that return renderable artifacts (Mermaid diagrams, React components, structured UI descriptions).

### Long-Term Watch
9. **Video content type**: As video understanding matures, MCP will need native video content support.
10. **Real-time streaming transport**: WebSocket/WebRTC transport for MCP to support live audio/video agent interactions.
11. **Agent Skills standard**: Anthropic's Agent Skills as complement to MCP tool definitions — procedural knowledge layer.
12. **Content modality routing**: Gateway/orchestrator patterns that route multi-modal content to appropriate specialized models.

---

## Sources

### Multi-Modal Models
- [Claude Vision Documentation](https://platform.claude.com/docs/en/build-with-claude/vision)
- [Claude Models Overview](https://platform.claude.com/docs/en/about-claude/models/overview)
- [Hello GPT-4o](https://openai.com/index/hello-gpt-4o/)
- [Introducing GPT-5](https://openai.com/index/introducing-gpt-5/)
- [GPT-5 Multimodal Deep Dive](https://sparkco.ai/blog/gpt-5-multimodal-deep-dive-video-audio-processing)
- [Google Introduces Gemini 2.0](https://blog.google/innovation-and-ai/models-and-research/google-deepmind/google-gemini-ai-update-december-2024/)
- [Project Astra](https://deepmind.google/models/project-astra/)
- [Google I/O 2025: Gemini Universal AI Assistant](https://blog.google/technology/google-deepmind/gemini-universal-ai-assistant/)

### Voice and Audio
- [OpenAI Voice Agents Guide](https://developers.openai.com/api/docs/guides/voice-agents/)
- [Introducing gpt-realtime](https://openai.com/index/introducing-gpt-realtime/)
- [OpenAI Realtime API Documentation](https://platform.openai.com/docs/guides/realtime)
- [Gemini Live API Overview](https://ai.google.dev/gemini-api/docs/live-api)
- [Google ADK Streaming Guide](https://google.github.io/adk-docs/streaming/dev-guide/part1/)

### Computer Use and Browser Automation
- [Anthropic Computer Use Tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool)
- [Anthropic Advanced Tool Use](https://www.anthropic.com/engineering/advanced-tool-use)
- [Anthropic Agent Capabilities API](https://www.anthropic.com/news/agent-capabilities-api)
- [Playwright MCP (GitHub)](https://github.com/microsoft/playwright-mcp)
- [Agent Skills Announcement](https://thenewstack.io/agent-skills-anthropics-next-bid-to-define-ai-standards/)

### MCP Specification
- [MCP 2025-06-18 Tools Spec](https://modelcontextprotocol.io/specification/2025-06-18/server/tools)
- [MCP Spec Update Analysis (Cisco)](https://blogs.cisco.com/developer/whats-new-in-mcp-elicitation-structured-content-and-oauth-enhancements)
- [MCP Spec Update (Forge Code)](https://forgecode.dev/blog/mcp-spec-updates/)
- [Multi-Modal MCP Servers (HackerNoon)](https://hackernoon.com/multi-modal-mcp-servers-handling-files-images-and-streaming-data)

### Document Processing
- [Claude PDF Support](https://platform.claude.com/docs/en/build-with-claude/pdf-support)
- [NVIDIA PDF Extraction](https://developer.nvidia.com/blog/approaches-to-pdf-data-extraction-for-information-retrieval/)
- [LlamaIndex Document AI Guide](https://www.llamaindex.ai/blog/document-ai-the-next-evolution-of-intelligent-document-processing)

### Video Understanding
- [Agentic Very Long Video Understanding (arXiv)](https://arxiv.org/html/2601.18157v1)
- [VideoDeepResearch (arXiv)](https://arxiv.org/html/2506.10821v1)
- [Twelve Labs: Video Intelligence Going Agentic](https://www.twelvelabs.io/blog/video-intelligence-is-going-agentic)

### Code and Visual Generation
- [Generative UI (CopilotKit)](https://www.copilotkit.ai/generative-ui)
- [A2UI: AI Agent for UI Design](https://www.griddynamics.com/blog/ai-agent-for-ui-a2ui)
- [Agentic AI and Markdown (Visual Studio Magazine)](https://visualstudiomagazine.com/articles/2026/02/24/in-agentic-ai-its-all-about-the-markdown.aspx)
