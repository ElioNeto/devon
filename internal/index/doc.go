// Package index provides semantic code search indexing using TF-IDF and BM25 scoring.
//
// It enables the Devon agent to search a codebase for relevant files based on
// natural-language queries. The index is built in-memory using a pure-Go TF-IDF
// implementation with BM25 relevance scoring.
//
// Types:
//   - Config — holds indexer configuration (extensions, excludes, file size limits, etc.).
//   - Indexer — main file indexing engine with AddFile, Rebuild, Search methods.
//   - Stats — snapshot of indexer statistics (document count, term count, indexed state).
//   - Manager — high-level coordinator that wraps Indexer with enable/disable and tool creation.
//   - Index — the in-memory searchable index (term frequencies, document frequencies).
//   - Document — represents a single indexed document (file) with its tokens.
//   - Token — a single token from text with position metadata.
//   - Tokenizer — splits text into tokens with stopword removal.
//   - BM25Calculator — computes BM25 relevance scores for ranked search results.
//   - Searcher — provides search functionality (Search, SearchByPath, SearchRegex, SearchPrefix).
//   - SearchCodebaseTool — an agent tool for searching the codebase.
//   - DocumentWithScore — a document with its BM25 relevance score.
//
// Public functions:
//   - NewIndexer — creates a new Indexer for a work directory.
//   - NewManager — creates a high-level Manager.
//   - NewIndex — creates an empty in-memory index.
//   - NewTokenizer — creates a tokenizer with default settings.
//   - NewBM25Calculator — creates a BM25 calculator with standard parameters.
//   - NewSearcher — creates a Searcher for a given Index.
//   - NewSearchCodebaseTool — creates the search_codebase agent tool.
//   - DefaultConfig — returns the default indexer configuration.
//
// Integration mode:
//   - Memory (semantic memory for facts) is always active when the agent runs.
//   - Index (codebase search) is opt-in and requires the --index flag or config
//     setting index.enabled=true. See docs/adr/integration-mode.md for rationale.
//
// Coverage note:
//   - The index package does not yet implement persistent index storage (Load/Save
//     are no-ops). Real embedding/vector search is not integrated. These features
//     are tracked in separate issues.
package index
