# Advanced Setup

## Web Search & Fetch Tools

Devon provides two web tools: `web_search` (search the web) and `web_fetch` (download a URL as Markdown).

### Configuration

Enable web tools in `devon.toml`:

```toml
[web]
enabled = true
backend = "auto"   # "auto" | "duckduckgo" | "firecrawl"
```

### Backend Selection

| Backend      | API Key Required | Cost  | Notes                                      |
|-------------|-----------------|-------|--------------------------------------------|
| DuckDuckGo  | No              | Free  | Uses lite.duckduckgo.com scraping          |
| Firecrawl   | Yes (`DEVON_FIRECRAWL_KEY`) | Paid | Better results, Markdown-native responses |

#### Auto mode

When `backend = "auto"` (default):
- If `DEVON_FIRECRAWL_KEY` is set → uses Firecrawl
- Otherwise → uses DuckDuckGo Lite (no key needed)

#### Explicit backend

```toml
[web]
enabled = true
backend = "duckduckgo"  # always use DuckDuckGo
```

```toml
[web]
enabled = true
backend = "firecrawl"   # requires DEVON_FIRECRAWL_KEY
```

### Environment Variables

| Variable                 | Required | Description                     |
|-------------------------|----------|---------------------------------|
| `DEVON_FIRECRAWL_KEY`   | For Firecrawl | Firecrawl API key          |

### Verification

Run `devon doctor` to verify web configuration:

```
[Web]
  Enabled:         true
  Backend:         auto (DuckDuckGo ou Firecrawl)
  Firecrawl Key:   sk-****abcd
```

### How It Works

1. **web_search**: selects backend → sends query → parses results → returns titles, URLs, snippets
2. **web_fetch**: selects backend → downloads page → converts HTML to Markdown (using built-in converter) → returns formatted content

Both tools respect contexts with timeout; DuckDuckGo backend uses 15s timeout, Firecrawl uses 30s.
