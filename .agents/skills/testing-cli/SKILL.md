# Testing the Devon CLI

## Prerequisites

- Go 1.24.2+ installed
- Repository cloned and on `go-migration` branch

## Building the Binary

```bash
go build -o /tmp/devon ./cmd/devon
```

Build from repo root. The binary is a single Go executable.

## Testing Profiles Commands

### Setup

1. Copy `.devon.toml.example` to a test directory as `devon.toml`
2. `LoadToml()` reads `devon.toml` from the current working directory (or `~/.devon.toml`)
3. Set env vars referenced in `api_key_env` fields to test key status display

### `devon profiles list`

- **Without devon.toml**: Run from a directory without `devon.toml` — should print "Nenhum devon.toml encontrado. Crie um a partir de .devon.toml.example"
- **With devon.toml**: Run from directory containing `devon.toml` — should list all profiles with key status (`✔` if env var set, `—` if not)
- Key status depends on `os.Getenv(profile.APIKeyEnv)` — set the env var to see `✔`

### `devon profiles test`

- Tests connectivity to each profile's `BaseURL + "/models"` endpoint
- Shows [PASS] for HTTP 2xx/3xx, [FAIL] for errors or 4xx/5xx
- Tests all profiles even if some fail (no early exit)
- `local` profile (Ollama) will show [FAIL] if Ollama isn't running locally — this is expected
- Timeout is 10 seconds per profile

### `--profile` and `--model` Flags

- `--profile` / `-p` resolves a profile from `devon.toml` and applies it to config
- `--model` overrides the model after profile resolution
- To test `--profile`, you need base env vars set first (`DEVON_API_KEY`, `DEVON_BASE_URL`, `DEVON_MODEL`) because `config.Load()` runs before `applyProfileFlags()`
- Non-existent profile names produce error: `perfil "<name>" não encontrado`

## Unit Tests

```bash
go test -timeout 30s -v ./internal/config/...
```

Covers: LoadToml, ResolveProfile, ValidateProfile, ApplyProfile, ResolveAPIKey, Doctor, ParseMode.

## Notes

- The CLI is in Portuguese (pt-BR) — error messages and output use Portuguese strings
- `config.Load()` requires `DEVON_MODEL` env var to be set (validates on load)
- For local providers (localhost URLs), API key is not required
- CI on `go-migration` has some pre-existing failures (data race in router_test.go, Windows PowerShell issues) — these are not related to profiles code

## Devin Secrets Needed

No secrets required for basic CLI testing. For full connectivity testing with `devon profiles test`:
- `OPENROUTER_KEY` — for testing OpenRouter profile connectivity
- `ANTHROPIC_KEY` — for testing Anthropic profile connectivity
