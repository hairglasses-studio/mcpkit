# DAG Execution Patterns for Agent Task Orchestration

Research conducted 2026-03-14. Covers production DAG engines, agent framework patterns,
dynamic DAG modification, failure/retry strategies, parallel execution, and MCP relevance.

---

## 1. DAG Execution Engines

### Key Abstractions Across Systems

| System | Node | Edge | Executor | State Machine |
|--------|------|------|----------|---------------|
| **Airflow** | Operator/Task | `>>` / `set_downstream` | LocalExecutor, CeleryExecutor, KubernetesExecutor | Per-task trigger rules (all_success, one_success, all_done, etc.) |
| **Dagster** | Op | Data dependency (inputs/outputs) or explicit `Nothing` | In-process, multiprocess, Dagster+Celery | Asset-centric materialization state |
| **Prefect** | Task (`@task`) | Implicit via Python control flow | ThreadPoolTaskRunner, ProcessPoolTaskRunner, DaskTaskRunner, RayTaskRunner | Per-task/per-flow state with retries + timeouts |
| **Temporal** | Activity | Workflow code (imperative) | Worker pools with task queues | Event History with deterministic replay |

### Architectural Observations

**Declarative vs Imperative DAGs.** Airflow and Dagster define DAGs declaratively (the
graph structure is known before execution). Temporal and Prefect 2.0+ take an imperative
approach where the "DAG" emerges from standard code execution. Prefect explicitly dropped
the requirement that workflows be "written explicitly as DAGs," embracing native Python
control flow instead.

**Data Dependencies vs Control Dependencies.** Dagster's primary edge type is data
dependency -- an op's inputs automatically depend on upstream outputs. Airflow uses
control dependencies (`>>` operator). This distinction matters for agent systems: LLM
task flows are usually control-flow oriented ("do A before B") rather than data-flow
oriented ("B needs A's output as a parameter"), though the best agent DAGs combine both.

**Trigger Rules.** Airflow's trigger rules are the most sophisticated for DAG execution
control. Beyond the default `all_success`, rules like `one_success`, `none_failed`,
`all_done`, and `always` enable complex branching, error recovery, and optional task
patterns. These are directly relevant to agent task orchestration where some tasks may
fail without blocking the overall flow.

### Temporal's Unique Approach

Temporal deserves special attention because it avoids DAGs entirely in favor of
**durable execution** -- workflows are ordinary code that the runtime makes fault-tolerant
through event sourcing and deterministic replay. Key properties:

- Workflows must follow **deterministic constraints** to ensure consistent replay
- Activities (side effects) have configurable retry policies: InitialInterval,
  BackoffCoefficient, MaximumInterval, MaximumAttempts, NonRetryableErrorTypes
- **Heartbeats** let long-running activities report progress; failed activities can resume
  from heartbeat details on retry
- Child workflows enable fan-out with configurable Parent Close Policies
  (TERMINATE, REQUEST_CANCEL, ABANDON)

This model is compelling for agent loops because LLM-driven execution is inherently
imperative -- the agent decides what to do next based on results so far.

---

## 2. DAG in Agent Frameworks

### LangGraph (LangChain)

LangGraph is the most DAG-aware agent framework. Core concepts:

- **StateGraph**: Parameterized by a user-defined State type (TypedDict or Pydantic).
  Nodes are functions that receive state and return updated state.
- **Edges**: Normal edges (`add_edge`) for fixed transitions; conditional edges
  (`add_conditional_edges`) with routing functions for dynamic transitions.
- **Reducers**: Each state key has an independent reducer determining how updates merge.
  Default is overwrite; `operator.add` for list concatenation; `add_messages` for
  conversation history.
- **Super-steps**: Execution proceeds in discrete steps. When a node has multiple outgoing
  edges, all destination nodes execute in parallel as part of the next super-step.
- **Send primitive**: Enables dynamic fan-out by returning node-state pairs for parallel
  invocation (map-reduce pattern).
- **Command primitive**: Unifies state updates with routing (`Command(update={...},
  goto="node")`), enabling dynamic control flow.
- **Cycles supported**: Unlike traditional DAGs, LangGraph explicitly supports cycles
  (agent loops). A configurable recursion limit (default 1000 super-steps) prevents
  infinite loops.
- **Checkpointing**: State persistence across invocations for resume-from-failure.
- **Subgraphs**: Nodes can navigate to parent graph nodes via
  `Command(graph=Command.PARENT)` for hierarchical agent handoff.

**Key insight**: LangGraph's graphs are technically not DAGs -- they are directed graphs
that allow cycles. This is intentional because agent execution is inherently iterative
(think-act-observe loops). The "DAG" aspect applies to the *task dependency structure*,
while the *execution flow* around each task may cycle.

### CrewAI Flows

CrewAI uses decorators to build dependency graphs:

- `@start()` marks entry points (can have multiple, run concurrently)
- `@listen(method)` establishes edges between methods
- `@router()` returns string labels for conditional branching
- `or_()` triggers when any dependency completes; `and_()` waits for all
- State management via unstructured (dict) or structured (Pydantic) approaches
- `@persist` decorator enables state recovery across restarts (SQLite backend)
- Human-in-the-loop via approval gates

**Key insight**: CrewAI's decorator-based approach is elegant for defining DAGs but limits
runtime modification. The graph structure is essentially static, with dynamism coming only
from router decisions.

### AutoGen (Microsoft)

AutoGen takes a fundamentally different approach -- no DAGs at all:

- Uses **topic-based pub/sub** with a Group Chat Manager
- Speaker selection via round-robin or **LLM-based selection** (the manager LLM analyzes
  conversation history to choose the next agent)
- Sequential execution only (one agent acts at a time)
- Termination via text triggers or message count limits

**Key insight**: AutoGen demonstrates that DAGs are not always necessary. For open-ended
multi-agent collaboration, dynamic LLM-driven routing can outperform predetermined
structures. However, this sacrifices predictability and makes progress tracking harder.

### MetaGPT

MetaGPT encodes **Standardized Operating Procedures (SOPs)** into prompt sequences rather
than using dynamic DAGs. It assigns roles to agents in an "assembly line paradigm."

**Key insight**: SOP-based approaches are essentially hardcoded DAGs with the structure
embedded in prompts. They reduce cascading hallucination errors by adding verification
checkpoints between steps.

### Pattern Comparison

| Pattern | Predictability | Flexibility | Parallelism | Progress Tracking |
|---------|---------------|-------------|-------------|-------------------|
| Static DAG (CrewAI) | High | Low | Yes (via and_/or_) | Easy |
| Cyclic Graph (LangGraph) | Medium | High | Yes (super-steps) | Medium |
| LLM-driven routing (AutoGen) | Low | Highest | No | Hard |
| SOP-embedded (MetaGPT) | High | Low | No | Easy |

---

## 3. Dynamic DAGs

### Runtime DAG Modification Patterns

**Dagster Dynamic Outputs.** Dagster's `DynamicOut` allows ops to yield multiple outputs
at runtime, each duplicating downstream graph portions. The `.map()` method clones
downstream ops per output, and `.collect()` merges results back (fan-in). This enables
data-driven parallelism where the number of branches is unknown at definition time.

```
load_pieces() -> DynamicOutput * N -> compute_piece.map() -> merge.collect()
```

**Airflow Dynamic DAGs.** Since DAGs are Python code, Airflow supports programmatic
construction (loops generating tasks). However, the DAG shape is fixed at parse time, not
at execution time. True runtime modification is not supported -- dynamic task mapping was
added in Airflow 2.3+ but remains limited compared to Dagster.

**Prefect's Approach.** Prefect 2.0+ abandoned DAG-first design entirely. Workflows use
native Python control flow (if/else, loops, generators). Tasks can be generated
dynamically at runtime via `.submit()` calls. The `.map()` method enables data-driven
fan-out similar to Dagster.

**LangGraph Send.** The `Send` primitive allows dynamic fan-out: a routing function can
return `[Send("node_name", state_1), Send("node_name", state_2), ...]` to dynamically
create parallel executions.

### LLM-Driven Dynamic DAGs

For agent systems, the most interesting pattern is **LLM-generated DAGs**:

1. **Plan-then-execute**: LLM generates a complete task DAG upfront, then an executor
   runs it. Benefits: full dependency analysis, parallel execution planning. Drawbacks:
   brittle when tasks fail or produce unexpected results.

2. **Iterative re-planning**: LLM generates a partial DAG, executes available tasks,
   then re-plans based on results. Benefits: adaptive. Drawbacks: more LLM calls,
   potential inconsistency.

3. **Hybrid**: Static DAG skeleton with LLM-driven decisions at branch points.
   Benefits: predictable overall structure with flexibility at decision points.

The hybrid approach is most practical for production systems. Ralph's current design
(static task spec with LLM deciding which ready task to work on) is already a form of
this -- the task DAG is static, but execution order within ready tasks is LLM-driven.

### Dynamic DAG Design Considerations

- **Validation**: How do you validate a DAG that doesn't exist yet? Approaches include
  incremental validation (validate each addition) and constraint-based validation
  (define invariants that must hold).
- **Checkpointing**: Dynamic DAGs make checkpointing harder because the graph shape at
  resume time may differ from checkpoint time. Solutions: checkpoint the DAG structure
  itself alongside execution state.
- **Observability**: Dynamic DAGs require runtime graph visualization, not just
  static definition visualization.

---

## 4. Failure/Retry in DAGs

### Production Patterns

**Temporal Retry Policies** (most sophisticated):
- Configurable per-activity: InitialInterval, BackoffCoefficient (default 2.0),
  MaximumInterval, MaximumAttempts (0 = unlimited)
- NonRetryableErrorTypes to bypass retries for known-fatal errors
- Custom NextRetryDelay for context-aware backoff
- Heartbeat-based progress recovery: failed activities resume from last heartbeat
- Three timeout levels: Schedule-To-Close, Start-To-Close, Schedule-To-Start

**Airflow Trigger Rules for Error Handling**:
- `none_failed`: continue if no upstream task failed (skips OK)
- `all_done`: continue regardless of upstream status
- `one_success`: continue if any upstream succeeded
- These enable sophisticated error recovery DAGs (e.g., fallback paths)

**Prefect Retry Configuration**:
- `retries` and `retry_delay_seconds` on tasks and flows
- Timeout handling varies by runner (cooperative cancellation for async,
  process-based for sync blocking operations)

### Checkpointing Strategies

1. **Full state snapshot** (LangGraph): Serialize entire graph state at each super-step.
   Resume by loading snapshot and replaying from checkpoint.

2. **Event sourcing** (Temporal): Record all commands and events. Replay from start,
   fast-forwarding through completed activities. Determinism requirement ensures
   identical state reconstruction.

3. **Progress file** (Ralph current): Record completed task IDs and iteration log.
   Resume by filtering tasks and continuing from last state.

4. **Per-node checkpointing** (Dagster): Each op's output is materialized and persisted.
   Downstream ops can resume from persisted upstream outputs without re-execution.

### Failure Modes Specific to Agent DAGs

- **LLM hallucination in task completion**: Agent marks task done but work is incomplete.
  Mitigation: validation hooks, output verification steps.
- **Cascading context degradation**: As the context window fills with failed attempts,
  LLM performance degrades. Mitigation: context pruning, summarization of failed
  attempts.
- **Budget exhaustion mid-DAG**: Token budget runs out before all tasks complete.
  Mitigation: cost estimation per remaining path, prioritizing critical-path tasks.
- **Tool unavailability**: MCP server goes down mid-execution. Mitigation: retry with
  backoff, alternative tool selection, task skip with trigger rules.

---

## 5. Parallel Execution Within DAGs

### Fan-Out / Fan-In Patterns

**LangGraph Super-steps**: Multiple outgoing edges from a node trigger parallel execution.
State updates from parallel nodes are merged using reducers (per-key merge functions).
The `Send` primitive enables dynamic fan-out where the number of parallel branches is
determined at runtime.

**Dagster DynamicOut**: Fan-out via `.map()`, fan-in via `.collect()`. Each branch runs
independently with fault isolation -- if one partition fails, only that branch restarts.

**Prefect Task Runners**: `.submit()` returns `PrefectFuture` objects. Multiple submissions
create fan-out; `wait()` provides fan-in. Different runners (ThreadPool, ProcessPool,
Dask, Ray) provide different parallelism models.

**CrewAI**: Multiple `@start()` methods run concurrently. `and_()` provides fan-in
(wait for all). `or_()` provides race semantics (first to complete wins).

### Concurrency Controls

| System | Mechanism | Granularity |
|--------|-----------|-------------|
| Airflow | Pool slots, `max_active_tasks_per_dag` | Per-pool, per-DAG |
| Dagster | Op-level parallelism via executor config | Per-job |
| Prefect | `max_workers` on TaskRunner | Per-flow |
| Temporal | Worker MaxConcurrentActivityExecutionSize | Per-worker |
| LangGraph | Implicit (super-step parallelism) | Per-super-step |

### Resource Constraints

Production DAG engines handle resource constraints through:
- **Pool/slot-based limiting** (Airflow): Named pools with slot counts
- **Concurrency groups** (mcpkit dispatcher): Per-group concurrent execution limits
- **Worker affinity**: Tasks routed to specific workers based on resource needs
- **Backpressure**: Queue-based systems apply backpressure when workers are saturated

### Parallel Execution for Agent DAGs

Key considerations for agent-specific parallel execution:

1. **Shared context problem**: Parallel agent tasks may need shared context (conversation
   history, accumulated knowledge). LangGraph solves this with state reducers.
2. **Token budget splitting**: When running parallel LLM calls, the token budget must be
   divided. Each parallel branch consumes tokens independently.
3. **Result merging**: How are results from parallel tasks combined? Options:
   - Concatenation (simple but verbose)
   - Summarization (requires additional LLM call)
   - Structured merge (per-field reducers like LangGraph)
4. **Error isolation**: A failure in one parallel branch should not block others.
   Independent error handling per branch is essential.

---

## 6. Relevance to MCP and Ralph

### Current MCP Spec Gaps

The MCP specification (2025-03-26) has no built-in support for:

- **Tool chaining/sequencing**: Each `tools/call` is independent. No way to express
  "call A, then pass result to B."
- **Batch tool invocation**: No batch `tools/call` endpoint. Each tool call requires
  a separate JSON-RPC request/response cycle.
- **Dependency declarations**: Tools cannot declare dependencies on other tools or
  express prerequisite relationships.
- **Workflow/DAG primitives**: No protocol-level concept of task graphs, execution plans,
  or orchestration patterns.
- **Progress/checkpoint protocol**: While MCP has progress tokens for individual calls,
  there is no protocol for multi-step workflow progress.

These are intentional design choices -- MCP is a low-level protocol for tool exposure,
not a workflow engine. DAG orchestration belongs in the client/host layer, which is
exactly where ralph sits.

### What Ralph Already Has

Ralph's current architecture provides a solid foundation for DAG execution:

- **Task with DependsOn**: `Task.DependsOn []string` already models DAG edges
- **ReadyTasks()**: Computes which tasks have all dependencies satisfied (topological
  frontier)
- **ValidateDependencies()**: Kahn's algorithm for cycle detection and reference
  validation
- **Progress tracking**: CompletedIDs, iteration log, checkpoint to disk
- **LLM-driven task selection**: The sampler decides which ready task to work on

### Recommended DAG Enhancements for Ralph

Based on this research, the following enhancements would bring ralph's DAG support to
production grade:

#### Priority 1: Parallel Task Execution

Currently ralph executes tool calls sequentially within an iteration. For independent
ready tasks, parallel execution would significantly improve throughput.

Design approach (leveraging existing `dispatcher` package):
- When multiple tasks are ready and independent, execute them in parallel
- Use dispatcher's concurrency groups for resource-constrained tasks
- Merge results using configurable reducers (default: concatenation)
- Each parallel branch has independent error handling

#### Priority 2: DAG-Aware Scheduling

Enhance the prompt/decision system to expose DAG topology to the LLM:

- **Critical path highlighting**: Identify the longest path through remaining tasks
  and prioritize accordingly
- **Parallelism hints**: Tell the LLM which tasks can run in parallel so it can
  plan multi-task iterations
- **Dependency visualization**: Current prompt already shows Ready/Blocked/Completed
  sections -- this is good and should be preserved

#### Priority 3: Failure Policies per Task

Add per-task configuration for failure handling:

```json
{
  "id": "fetch-data",
  "description": "Fetch data from API",
  "depends_on": ["auth"],
  "retry": {"max_attempts": 3, "backoff": "exponential"},
  "on_failure": "skip",
  "required": false
}
```

Inspired by Airflow's trigger rules:
- `required: true` (default) -- downstream tasks fail if this fails
- `required: false` + `on_failure: "skip"` -- downstream tasks proceed if this fails
- Retry policies similar to Temporal's model

#### Priority 4: Dynamic Task Generation

Allow the LLM to propose new tasks at runtime:

```json
{
  "task_id": "research",
  "tool_name": "search",
  "arguments": {"query": "..."},
  "add_tasks": [
    {"id": "analyze-result-1", "description": "...", "depends_on": ["research"]},
    {"id": "analyze-result-2", "description": "...", "depends_on": ["research"]}
  ]
}
```

This is the hybrid approach: static skeleton with LLM-driven expansion. Key constraints:
- New tasks can only depend on existing tasks (no orphans)
- New tasks are validated incrementally (cycle check, reference check)
- Dynamic additions are logged in progress for resume support
- Optional: max dynamic tasks limit to prevent runaway expansion

#### Priority 5: Checkpoint-and-Resume

Enhance progress tracking for robust resume:

- Save DAG structure alongside progress (essential for dynamic DAGs)
- Per-task output caching: store tool results so resumed runs can skip re-execution
- Heartbeat support: long-running tasks report progress for partial recovery
- Resume modes: "from-last-checkpoint" vs "re-validate-and-continue"

### Integration with Existing mcpkit Packages

| Package | Integration Point |
|---------|-------------------|
| `dispatcher` | Parallel task execution with priority and concurrency groups |
| `finops` | Per-branch cost tracking, budget-aware scheduling (skip low-priority branches when budget is low) |
| `memory` | Cross-run task result caching for resume |
| `sampling` | Parallel sampling calls for independent task branches |
| `resilience` | Circuit breaker per tool, rate limiting for parallel calls |

---

## 7. Open Questions

1. **Cycle tolerance**: Should ralph support cyclic task graphs (retry-until-success
   patterns) or enforce strict DAGs? LangGraph's cycle support suggests value, but
   cycles complicate progress tracking and termination guarantees.

2. **DAG vs imperative**: Should ralph move toward Temporal-style imperative workflows
   where the LLM's code-like decisions define the execution graph? Or keep the
   declarative task spec approach?

3. **Multi-agent DAGs**: When ralph supports multiple agents (Phase 6 orchestrator),
   should the DAG be per-agent or cross-agent? LangGraph's subgraph model suggests
   hierarchical composition.

4. **Streaming within DAGs**: How should streaming tool results interact with DAG
   execution? Can downstream tasks begin processing before upstream tasks fully complete?

5. **DAG specification format**: Is JSON sufficient for complex DAGs, or should ralph
   support a DSL or visual editor? CrewAI's decorator approach and Airflow's Python
   DAGs suggest code-as-DAG has ergonomic advantages.

---

## References

- Apache Airflow DAG concepts: https://airflow.apache.org/docs/apache-airflow/stable/core-concepts/dags.html
- Dagster Graphs and Dynamic Graphs: https://docs.dagster.io/concepts/ops-jobs-graphs/
- Prefect Flows and Task Runners: https://docs.prefect.io/latest/
- Temporal Workflows and Failure Detection: https://docs.temporal.io/
- LangGraph Graph API and Concepts: https://docs.langchain.com/oss/python/langgraph/
- CrewAI Flows: https://docs.crewai.com/concepts/flows
- AutoGen Group Chat: https://microsoft.github.io/autogen/stable/
- MetaGPT (Guo et al. 2023): https://arxiv.org/abs/2308.00352
- MCP Specification 2025-03-26: https://modelcontextprotocol.io/specification/
- Anthropic Tool Use: https://platform.claude.com/docs/en/docs/build-with-claude/tool-use
