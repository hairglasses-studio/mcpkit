// Package eval provides an evaluation framework for scoring MCP tool accuracy.
//
// A [Suite] groups [Case] values (tool name + args + expected output) with
// one or more [Scorer] implementations ([ExactMatch], [Contains], [Regex],
// [JSONPath], [NotEmpty]) and [ResultScorer] implementations ([Latency],
// [ErrorRate]) that operate on the full [Result] including duration. [Run]
// and [RunT] execute a suite against a [registry.ToolRegistry] and return a
// [Summary] with per-case scores, pass/fail counts, and average score.
// [LoadSuiteJSON] deserializes a suite from a JSON file for data-driven CI
// pipelines.
//
// Example:
//
//	suite := eval.Suite{
//	    Name: "greet",
//	    Cases: []eval.Case{
//	        {Name: "basic", Tool: "greet", Args: map[string]any{"name": "world"}, Expected: "hello"},
//	    },
//	    Scorers: []eval.Scorer{eval.Contains()},
//	}
//	summary, _ := eval.Run(ctx, reg, suite)
//	fmt.Printf("pass rate: %.0f%%\n", summary.PassRate()*100)
package eval
