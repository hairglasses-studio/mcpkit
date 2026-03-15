// Package finops provides token accounting, budget policies, dollar-cost
// estimation, and usage tracking for MCP servers. It includes a Tracker for
// per-request token counting, a CostEstimator with configurable model pricing,
// scoped budget tracking per tenant/user/session, and time-windowed usage
// rotation. A middleware is provided to integrate tracking into the registry
// tool chain.
package finops
