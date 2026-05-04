// Package orchestrator provides an experimental multi-agent orchestration framework.
//
// # Purpose
//
// The orchestrator package is designed to coordinate multiple AI agents (AgentWorkers)
// to decompose, plan, and execute complex tasks in parallel. It is intended as a future
// multi-agent framework alternative to the single-agent approach used by internal/agent.
//
// # Relationship with internal/agent
//
// This package is completely orphaned — it is not imported by any other package in the
// codebase. It exists as an experimental staging ground for multi-agent coordination
// patterns that may eventually be integrated into the main agent loop.
//
// # Overlap with internal/agent
//
// There is significant functional overlap between this package and internal/agent:
//   - AgentWorker.executeTask duplicates agent.go's runLoop: both send a prompt to
//     an LLM and process the streaming response. The orchestrator version is a
//     simplified placeholder that does not perform real tool execution.
//   - Planner overlaps with agent task classification (resolveTaskType in agent.go):
//     both use the LLM to analyse a user request and produce a structured plan. The
//     orchestrator version produces a DAG of subtasks while agent.go classifies a
//     single task type.
//
// # Rationale for Keeping Both
//
//   - internal/agent is the current single-agent production path. It is stable, tested,
//     and handles real tool execution with permission checks, caching, and truncation.
//   - internal/orchestrator is a future multi-agent framework that explores DAG-based
//     task decomposition, parallel worker execution, and result aggregation. It is not
//     yet ready for production use.
//   - Keeping them separate avoids destabilising the production agent while allowing
//     exploratory development of multi-agent patterns.
//
// # Public Types
//
//   - Planner — uses an LLM to decompose a user task into a DAG of subtasks (Plan).
//     Provides Plan() for LLM-based planning and PlanSimple() for a fallback plan.
//   - Plan — a set of Task items organised as a DAG with root tasks identified.
//   - Task — a single subtask with an ID, description, dependency list, and priority.
//   - Scheduler — executes tasks according to an ExecutionMode (Sequential, Parallel,
//     Async, Pipeline) respecting dependency ordering.
//   - ExecutionMode — defines how AgentWorkers are scheduled (sequential, parallel, etc.).
//   - AgentWorker — wraps an LLM client and a task channel to execute individual tasks.
//     Analogous to a simplified Agent from internal/agent.
//   - WorkerConfig — configuration for an AgentWorker (ID, role, enabled tools, DB).
//   - WorkerResult — the output of a single task execution, including errors and metrics.
//   - Aggregator — collects WorkerResult values from multiple workers, aggregates, and
//     summarises them.
//   - EventAggregator — a simple pub/sub event collector keyed by topic.
//   - Orchestrator — top-level coordinator that creates a plan, spawns workers, dispatches
//     tasks via the Scheduler, and aggregates results.
package orchestrator
