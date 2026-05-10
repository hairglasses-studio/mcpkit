// Package workflow provides a cyclical graph engine for building stateful
// agent workflows with conditional branching, checkpoints, rollback, and
// reusable multi-agent orchestration templates.
//
// A [Graph] is constructed by adding named [NodeFunc] nodes with
// [Graph.AddNode], setting transition conditions via [Graph.AddEdge], and
// registering fork nodes ([AddForkNode]) for parallel branches. The [Engine]
// executes the graph step-by-step, persisting intermediate [State] snapshots
// via an optional [CheckpointStore]. Template helpers such as [FanOutTemplate],
// [SwarmBroadcastTemplate], and [HierarchicalDelegationTemplate] wrap common
// multi-agent topologies from the orchestrator package as workflow graphs. When
// [EngineConfig.CompensateOnFailure] is enabled, nodes registered with
// [AddCompensableNode] are rolled back in LIFO order on failure, implementing
// the saga/compensation pattern.
//
// Example:
//
//	g := workflow.NewGraph()
//	g.AddNode("fetch", fetchData)
//	g.AddNode("process", processData)
//	g.AddEdge("fetch", "process")
//	g.AddEdge("process", workflow.EndNode)
//	g.SetStart("fetch")
//	engine, _ := workflow.NewEngine(g)
//	result, _ := engine.Run(ctx, "run-1", workflow.NewState())
package workflow
