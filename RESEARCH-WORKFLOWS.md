# Workflow Engine & State Machine Research for Agent Systems

Research findings for mcpkit `workflow/` package design. Covers workflow engines, state machines, durable execution, and agent-specific patterns.
Last updated: 2026-03-14.

---

## 1. Workflow Engine Architectures

### Temporal / Cadence

Temporal (and its predecessor Cadence) pioneered durable execution for distributed systems. Core abstractions:

- **Workflows**: Deterministic orchestration functions that coordinate activities. Must be side-effect-free — all I/O goes through activities. State is implicitly persisted via event history replay.
- **Activities**: Units of work that perform actual I/O (API calls, DB writes, LLM inference). Independently retryable with configurable timeouts and retry policies.
- **Signals**: External events injected into a running workflow. Used for human-in-the-loop approvals and async notifications.
- **Queries**: Read-only introspection of workflow state without side effects.
- **Timers**: Durable sleeps that survive process restarts (`workflow.Sleep()` instead of `time.Sleep()`).
- **Child Workflows**: Hierarchical composition — a workflow can spawn child workflows with independent lifecycles.
- **Task Queues**: Decouple workflow scheduling from worker execution. Workers poll queues for tasks.

Temporal positions itself as "the orchestrator for AI applications." Replit migrated their entire AI coding agent to Temporal. Their key insight: wrap each LLM/tool call in an Activity so retries, timeouts, and crash recovery are handled transparently.

Source: temporal.io/ai, temporal.io/blog/the-heros-journey-to-ai-durability-with-temporal

### AWS Step Functions

State machine-as-a-service using Amazon States Language (JSON-based DSL). Core state types:

- **Task**: Invoke a Lambda, ECS task, or AWS service
- **Choice**: Conditional branching based on state data
- **Wait**: Pause for a duration or until a timestamp
- **Parallel**: Execute branches concurrently, wait for all
- **Map**: Iterate over a collection with fan-out/fan-in
- **Pass**: Transform data without side effects
- **Succeed/Fail**: Terminal states

Two execution models: Standard (up to 1 year, exactly-once, full history) and Express (up to 5 minutes, high throughput, at-least-once). Standard workflows persist state between transitions; Express workflows do not.

Key limitation for agents: Step Functions are inherently acyclic (DAG-based). Loops require workarounds via Choice states that branch back, and there is no native concept of an open-ended agent loop.

Source: docs.aws.amazon.com/step-functions

### Inngest

Developer-friendly workflow engine built on event-driven functions with step-based durable execution. Core abstractions:

- **Functions**: Event-triggered handlers containing one or more steps
- **Steps**: Named, independently retryable units within a function (`step.run()`, `step.ai.wrap()`, `step.ai.infer()`)
- **Events**: Triggers that start functions or signal between them
- **AgentKit**: Framework for single-model to multi-agent systems

Three principles of durable execution per Inngest: (1) incremental execution — each step runs independently, (2) state persistence — outputs saved externally, (3) fault tolerance — retries skip completed steps.

Inngest introduced the "agent harness" concept: rather than a prescriptive framework, provide connective infrastructure (durable execution, event routing, state persistence) that orchestrates existing components without doing the work itself. This maps well to MCP's tool-centric model.

Source: inngest.com/ai, inngest.com/blog/your-agent-needs-a-harness-not-a-framework

### Hatchet

Open-source task orchestration engine with three primary abstractions:

- **Tasks**: Individual function units wrapped for scheduling and monitoring
- **Workers**: Long-running processes that poll task queues
- **Durable Workflows**: Task collections with dependencies, retries, and checkpointing

Every task invocation is durably persisted, enabling debugging, replays, and automatic checkpoint recovery. Supports DAGs, child task spawning, and event-driven triggering. Focuses on slot-based worker capacity management and concurrency control.

Source: docs.hatchet.run

---

## 2. State Machines for Agents

### XState / Statecharts

XState implements Harel statecharts, extending finite state machines with:

- **Hierarchical states**: Nested state machines (compound states) for managing sub-behaviors
- **Parallel regions**: Concurrent state machines executing simultaneously
- **History states**: Remember and restore previous state configurations
- **Guards**: Conditional transition logic based on context data
- **Actions**: Side effects on entry, exit, or during transitions
- **Actors**: Spawned entities that communicate via message passing (actor model)
- **Context**: Mutable data separate from discrete state identity
- **Invocations**: Child actors (promises, machines, callbacks) managed by parent lifecycle

Key insight for agents: statecharts separate "what state am I in" (finite, discrete) from "what data do I have" (context, continuous). This maps naturally to agent control flow (planning/executing/reflecting/waiting) vs. agent memory (conversation history, tool results, accumulated knowledge).

Source: stately.ai/docs/xstate

### Petri Nets

Formal model with places (states), transitions (events), and tokens (resources). Advantages for agent systems:

- Natural modeling of concurrent, asynchronous processes
- Tokens represent available resources (API rate limit slots, budget remaining)
- Transitions fire only when all input places have tokens (synchronization)
- Well-studied mathematical properties for deadlock detection and reachability analysis

Petri nets are more expressive than FSMs for modeling concurrent agent systems but harder to implement and reason about in practice.

### LangGraph

LangGraph models agent workflows as directed graphs (explicitly supporting cycles, unlike DAGs):

- **Nodes**: Functions that process shared state (LLM calls, tool execution, evaluation)
- **Edges**: Fixed or conditional transitions between nodes
- **State**: Typed shared data structure (e.g., `MessagesState`) flowing through the graph
- **Conditional edges**: Route based on state evaluation (e.g., if tool call requested, go to tool node)
- **Checkpointer**: Saves graph state at every super-step boundary, organized into threads
- **START/END**: Sentinel nodes marking entry and exit points

Core agent patterns in LangGraph:
- **Agent loop**: Continuous feedback loop — LLM decides action, tool executes, result feeds back until done
- **Orchestrator-worker**: Dynamic task decomposition via `Send` API for parallel delegation
- **Evaluator-optimizer**: One node generates, another evaluates against criteria, loops until acceptable
- **Routing**: Direct inputs to specialized sub-graphs based on classification

Persistence model: Checkpoints save state at super-step boundaries. Threads accumulate state across runs (conversational memory). `Store` interface enables cross-thread shared memory with namespace isolation. Time travel debugging replays prior executions. Failed nodes preserve successful peer checkpoint writes.

Source: docs.langchain.com/oss/python/langgraph

---

## 3. Cyclical vs Acyclic Flows

DAGs (Directed Acyclic Graphs) enforce a topological ordering where each node executes at most once. Agent loops are fundamentally cyclical — an LLM decides, acts, observes, and decides again indefinitely.

### How Frameworks Handle Cycles

**LangGraph**: First-class cycle support. Conditional edges create loops naturally (e.g., tool_node -> llm_node -> conditional_edge -> tool_node). The graph is explicitly not a DAG. Cycles terminate when a conditional edge routes to END.

**Temporal**: No explicit graph — cycles emerge from workflow code. A `for` loop or `while` loop in a workflow function naturally creates cycles. Each iteration's activity results are persisted in event history. Temporal's continue-as-new mechanism handles unbounded loops by starting a fresh execution with carried-over state (prevents event history from growing without bound).

**Step Functions**: Pseudo-cycles via Choice states that branch backward. Not native — requires careful state management to avoid infinite loops. Map state provides bounded iteration over collections.

**Inngest**: Cycles through recursive step invocations or event-driven re-triggering. A function can emit an event that triggers itself with updated state.

### Agent Loop Patterns

1. **Bounded loops**: Fixed iteration limit (e.g., max 10 tool calls). Simple but may truncate useful work.
2. **Convergence detection**: Stop when output stabilizes or evaluation passes threshold. Requires an evaluator.
3. **Budget-bounded**: Stop when token/cost budget exhausted. Natural fit for finops integration.
4. **Continue-as-new**: Temporal pattern — checkpoint state, start fresh execution. Prevents unbounded history growth.
5. **Signal-based termination**: External signal (human, timer, event) stops the loop.

### Relevance to mcpkit

The existing `ralph` package implements bounded agent loops with budget controls. A `workflow/` package should support arbitrary cycles with configurable termination conditions, building on ralph's patterns but generalizing to multi-node graphs.

---

## 4. Durable Execution

Durable execution ensures workflows complete despite failures — processes can crash, restart on different machines, and resume from where they left off.

### Core Mechanisms

**Event History / Event Sourcing**: Every decision and activity outcome is recorded as an immutable event. On recovery, the workflow replays its event history to reconstruct state without re-executing side effects. This is Temporal's primary mechanism.

**Memoization / Step Caching**: Each step's output is cached externally. On retry, completed steps return cached results instead of re-executing. This is Inngest's primary mechanism.

**Checkpointing**: Periodic snapshots of complete state. On recovery, restore from last checkpoint rather than replaying from the beginning. This is LangGraph's primary mechanism.

### Exactly-Once Semantics

True exactly-once is achieved by combining at-least-once delivery with idempotent operations:
- Temporal: Workflow IDs prevent duplicate executions. Activities may execute multiple times but the workflow sees each result exactly once.
- Step Functions (Standard): Exactly-once state transitions via managed persistence.
- Inngest: Step result caching ensures each step runs precisely once despite multiple function invocations.

### The Complexity Cliff (Temporal)

Temporal identifies a "complexity cliff" where agent sophistication causes traditional approaches to break down:
- Failure probability increases with each marginal step (compounding)
- Iteration becomes prohibitively expensive (testing step 92 requires re-running steps 1-91)
- Restarts become dangerous (side effects already occurred) or impossible (external state changed)

Durable execution addresses this by: (1) automatic state persistence without manual checkpointing, (2) replay from event history without re-execution, (3) history branching for parallel experiments.

Testing 100 prompt variants at step 92 of a 100-step workflow: ~900 steps with durable execution vs ~10,000 without (91% reduction).

Source: temporal.io/blog/building-ai-agents-that-overcome-the-complexity-cliff

### Application to Agent Tasks

For MCP tool execution, the durable execution pattern maps cleanly:
- Each tool call is an Activity/Step — independently retryable with timeout
- Orchestration logic (which tools to call, in what order) is the Workflow
- LLM inference is an Activity — expensive, cacheable, retry-safe
- Human approval is a Signal/WaitForEvent — durable pause without resource consumption

---

## 5. Event-Driven Agent Flows

### Event Sourcing for Agents

Event sourcing stores agent decisions and observations as an append-only log rather than mutable state. Benefits:
- Complete audit trail of agent reasoning and actions
- Temporal queries ("what was the agent's state at time T?")
- Replay for debugging and testing
- Branch-and-explore for alternative strategies

### Inngest's Three Sub-Agent Delegation Patterns

Inngest identifies three essential patterns for agent-to-agent communication:

1. **Sync (blocking)**: Parent spawns sub-agent and waits for result. Implemented via `step.invoke()` — function-to-function RPC with durability. Use when parent needs the answer to proceed.

2. **Async (fire-and-forget)**: Parent sends event, continues immediately. Sub-agent runs independently and delivers results directly. Implemented via `step.sendEvent()`. Use for long-running tasks.

3. **Scheduled (deferred)**: Parent schedules sub-agent for future execution with fresh data. Implemented via `step.sendEvent()` with timestamp. Use for follow-ups and recurring checks.

Key insight: "Don't choose for the model. Give it all three tools with clear descriptions. It picks correctly."

Source: inngest.com/blog/three-patterns-you-need-for-agentic-systems

### Reactive Patterns

- **Event-driven function triggers**: Functions react to events rather than being called imperatively
- **Fan-out/fan-in**: Event triggers multiple functions; results aggregate back
- **Event chaining**: One function's completion event triggers the next function
- **Singleton concurrency**: Per-conversation serialization prevents message collisions

---

## 6. Human-in-the-Loop

Production agent systems require human intervention points. Common patterns:

### Approval Gates

- **Temporal Signals**: Workflow calls `workflow.WaitForSignal()`, pausing durably. External system (UI, Slack bot) sends signal to resume. Survives restarts — no resource consumption during wait.
- **Inngest waitForEvent**: `step.waitForEvent()` pauses function execution until matching event arrives. Supports timeout for escalation.
- **LangGraph Interrupts**: Checkpoint state, return control to caller. Caller inspects state, optionally modifies it, then resumes graph execution.
- **Step Functions Callback**: `.waitForTaskToken` pattern — send token to external system, workflow pauses until callback received.

### Escalation Patterns

- Timeout-based: If no human response within N minutes, auto-approve or escalate to manager
- Confidence-based: Only request approval when agent confidence is below threshold
- Risk-based: Destructive or high-cost actions always require approval; read-only actions proceed automatically
- Progressive: First N actions auto-approved, then require human confirmation until trust established

### Breakpoints / Inspection

- LangGraph's checkpoint system allows inspecting agent state at any node boundary
- Temporal Queries provide read-only state inspection without interrupting execution
- Step-level observability (Inngest, Hatchet) shows each step's inputs/outputs in dashboards

### MCP Connection

MCP's elicitation feature (`handler.ElicitForm`) already provides a protocol-level mechanism for requesting user input during tool execution. A workflow engine could use elicitation as the transport for human-in-the-loop gates, making approval requests flow through the same MCP channel as tool calls.

---

## 7. Compensation and Rollback

### The Saga Pattern

When distributed transactions span multiple services/tools, the Saga pattern provides eventual consistency:

- **Forward flow**: Execute steps sequentially, each with its own local transaction
- **Compensation**: If step N fails, execute compensating transactions for steps N-1 through 1 in reverse
- **Compensating transaction**: An operation that semantically undoes a previous operation (not a database rollback — a new operation)

Two coordination approaches:
- **Choreography**: Services react to events autonomously. Decentralized but harder to reason about.
- **Orchestration**: Central coordinator directs the workflow. Easier to understand and modify.

Source: microservices.io/patterns/data/saga.html

### Application to Agent Systems

Agent actions often have real-world side effects that need reversal:
- Created a file? Delete it on failure
- Sent an API request? Send a cancellation
- Made a database change? Apply reverse migration
- Allocated resources? Release them

Compensation in agent workflows:
- Each tool call can register a compensating action (undo function)
- On workflow failure, compensations execute in reverse order
- Some actions are non-compensable (sent an email, posted a tweet) — these require confirmation gates before execution

### Implementation Pattern

```
type CompensableStep struct {
    Execute    func(ctx) (result, error)
    Compensate func(ctx, result) error
}
```

The workflow engine tracks completed steps and their results. On failure, it walks the completed list in reverse, calling each compensating function with the original result.

---

## 8. MCP Relevance

### How MCP Could Integrate with Workflow Engines

**MCP tools as workflow activities**: Each MCP tool call maps naturally to a workflow activity/step. The workflow engine wraps tool invocations with retry policies, timeouts, and result caching.

**Temporal's MCP integration**: Temporal explicitly lists "MCP integrations" as a supported use case and has a tutorial on "Building Durable MCP Tools with Temporal." The pattern: wrap MCP tool calls in Activities, orchestrate multi-step tool chains via Workflows, store intermediate results in Workflow state.

Source: temporal.io/ai, learn.temporal.io/tutorials/ai

**Vercel AI SDK + Temporal plugin**: The `AiSDKPlugin` wraps every LLM call in a Temporal Activity automatically. Tool functions that call external APIs run as Activities with durable retry. Single-line change from `openai('gpt-4o-mini')` to `temporalProvider.languageModel('gpt-4o-mini')`.

Source: temporal.io/blog/building-durable-agents-with-temporal-and-ai-sdk-by-vercel

### What a Workflow-Aware MCP Server Would Look Like

1. **Workflow as a resource**: Expose workflow definitions as MCP resources. Clients can list available workflows, inspect their structure (nodes, edges, state schema), and get execution status.

2. **Workflow execution as a tool**: A `workflow/execute` tool that starts a workflow, returning a task ID. Maps to MCP's Task pattern for async operations with status tracking.

3. **Step-level tool mapping**: Each workflow node corresponds to an MCP tool call. The workflow engine orchestrates which tools to call, handles retries, and manages state between calls.

4. **Durable sampling**: LLM inference steps within a workflow use MCP sampling with automatic persistence. If the server crashes mid-inference, it resumes from the last checkpoint rather than re-running the entire conversation.

5. **Event-driven notifications**: MCP notifications (`tools/list_changed`, custom notifications) trigger workflow transitions. External events (webhooks, schedules) start or signal workflows.

6. **Budget-aware orchestration**: Integration with finops for token/cost budgets per workflow execution. Workflows pause or terminate when budgets are exceeded.

### Architecture for mcpkit `workflow/`

Based on this research, a workflow package for mcpkit should provide:

**Core types**:
- `Graph`: Directed graph with nodes, edges, and typed state. Supports cycles.
- `Node`: Named function that transforms state. Maps to an MCP tool call, LLM inference, or arbitrary computation.
- `Edge`: Connection between nodes — fixed, conditional (guard function), or dynamic (computed at runtime).
- `State`: Typed, versioned state object flowing through the graph. Thread-safe with merge semantics for parallel branches.
- `Checkpoint`: Serializable snapshot of graph execution state at a node boundary.

**Execution model**:
- Step-based execution with checkpoint after each node completion
- Support for cycles via conditional edges with termination conditions
- Parallel branch execution with join semantics (wait-all, wait-any, wait-N)
- Continue-as-new for unbounded loops (carry state forward, reset history)

**Durability**:
- Pluggable `CheckpointStore` interface (in-memory, file, database)
- Memoized step results — skip completed nodes on replay
- Crash recovery: restore from last checkpoint, resume execution

**Integration points**:
- `registry.ToolHandlerFunc` as node implementations — each node is a tool call
- `sampling.Client` for LLM inference nodes
- `finops.Budget` for cost-bounded execution
- MCP elicitation for human-in-the-loop gates
- MCP notifications for event-driven transitions
- `ralph` loop as a single-node cyclical workflow (backward compatibility)

**Middleware**:
- Compensation registration per node (saga pattern)
- Timeout per node and per workflow
- Retry policies per node
- Observability (tracing spans per node, metrics per workflow)

---

## 9. Key Takeaways

1. **Durable execution is the foundation**: Every serious workflow engine (Temporal, Inngest, Hatchet) builds on durable execution. Without it, multi-step agent workflows are fragile at scale. The "complexity cliff" makes this non-negotiable for production agents.

2. **Graphs, not pipelines**: Agent workflows are inherently graph-shaped with cycles. LangGraph proves that directed graphs with conditional edges and cycle support are the right abstraction. DAG-only systems (Step Functions) require workarounds.

3. **Separation of orchestration and execution**: Temporal's workflow/activity split is the most proven pattern. The orchestrator (workflow) is deterministic and lightweight; the workers (activities) handle I/O and can fail independently.

4. **Checkpointing over event sourcing for simplicity**: LangGraph's checkpoint model is simpler to implement than Temporal's full event sourcing/replay. For an MCP toolkit, checkpoint-based persistence with pluggable stores is the pragmatic choice.

5. **Three delegation patterns**: Sync (blocking), async (fire-and-forget), and scheduled (deferred) cover the agent-to-agent communication space. All three should be supported.

6. **Human-in-the-loop is a first-class concern**: Not an afterthought. MCP elicitation provides the protocol-level mechanism; the workflow engine provides the durable pause/resume semantics.

7. **Compensation must be explicit**: The saga pattern requires developers to define undo operations. Not all actions are reversible — the workflow engine should distinguish compensable from non-compensable steps and gate non-compensable steps with approval.

8. **The harness pattern fits MCP**: Inngest's "harness not framework" philosophy aligns with mcpkit's middleware-centric design. The workflow engine provides orchestration infrastructure; MCP tools provide the capabilities. The engine connects them without prescribing how tools work.

---

## Sources

- temporal.io/ai — Temporal AI platform positioning
- temporal.io/blog/the-heros-journey-to-ai-durability-with-temporal — Durable AI patterns
- temporal.io/blog/building-ai-agents-that-overcome-the-complexity-cliff — Complexity cliff analysis
- temporal.io/blog/building-durable-agents-with-temporal-and-ai-sdk-by-vercel — Vercel AI SDK integration
- docs.aws.amazon.com/step-functions — AWS Step Functions state machine model
- inngest.com/ai — Inngest AI platform and AgentKit
- inngest.com/blog/your-agent-needs-a-harness-not-a-framework — Agent harness pattern
- inngest.com/blog/three-patterns-you-need-for-agentic-systems — Sub-agent delegation patterns
- inngest.com/blog/principles-of-durable-execution — Durable execution principles
- inngest.com/blog/durable-execution-key-to-harnessing-ai-agents — Durable execution for AI
- docs.hatchet.run — Hatchet task orchestration
- stately.ai/docs/xstate — XState statecharts
- docs.langchain.com/oss/python/langgraph — LangGraph graph-based agents
- microservices.io/patterns/data/saga.html — Saga pattern
