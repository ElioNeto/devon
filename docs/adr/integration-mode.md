# ADR-001: Integration Mode for Memory and Index Packages

## Status

Accepted (2026-05-04)

## Context

The Devon agent has two related but distinct knowledge subsystems:

1. **Memory** (`internal/memory`) — a persistent semantic memory for facts about the project
   (conventions, architecture decisions, errors, etc.). It stores facts in the SQLite database
   and retrieves relevant context for the system prompt.

2. **Index** (`internal/index`) — a codebase search index using TF-IDF/BM25 for finding relevant
   source files based on natural-language queries.

Both subsystems help the agent understand the project, but they have different operational
characteristics and maturity levels.

## Decision

### Memory — Active by Default

- The `internal/memory` package is **always active** when the agent runs.
- It is imported and wired in `cmd/devon/main.go` and `internal/agent/agent.go`.
- The agent automatically stores and retrieves facts during normal operation.
- Rationale: The memory subsystem is lightweight (SQLite-backed, no external dependencies),
  has no performance overhead when empty, and provides immediate value for context retention.

### Index — Opt-in (Disabled by Default)

- The `internal/index` package is **NOT wired by default**.
- It requires explicit activation via:
  - CLI flag: `--index`
  - Config setting: `index.enabled = true`
- Rationale:
  - The index subsystem is more resource-intensive (file scanning, tokenization, in-memory index).
  - It is still maturing — persistence (Load/Save) is not yet implemented (no-ops).
  - Real embedding/vector search is not integrated.
  - Making it opt-in avoids surprising users with unexpected CPU/memory usage.

## Consequences

1. **Memory** will always be available for context injection and fact storage.
2. **Index** requires explicit opt-in, keeping the default agent lightweight.
3. Future work should:
   - Implement index persistence (Load/Save) — tracked in issue #75
   - Add real embedding support for semantic code search — tracked in issue #75
   - Integrate GitignoreMatcher with the file watcher — tracked in issue #75
4. When index is enabled, the agent gains the `search_codebase` tool for finding
   relevant files during conversations.

## Related

- Issue #75 — Audit, document, test, and stabilize memory and index packages
- `internal/memory/memory.go` — Manager, New, Remember, Recall, Clear, ContextFor
- `internal/index/doc.go` — Package documentation
