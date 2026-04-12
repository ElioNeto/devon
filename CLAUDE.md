# CLAUDE.md — Contexto do projeto Devon

> Este arquivo existe para que assistentes de IA (Claude, Copilot, etc.) entendam
> rapidamente a arquitetura, convenções e estado atual do projeto antes de qualquer tarefa.

---

## O que é o Devon

Devon é um **agente de código com TUI** que conecta a qualquer LLM compatível com a
API OpenAI. Escrito em Go puro (zero CGO), cross-compilável, com SQLite embarcado.

```
devon [flags]               → abre TUI interativa
devon run "<tarefa>"        → executa tarefa não-interativa (one-shot)
devon doctor                → valida config e testa conexão com o provider
devon profiles list|show    → gerencia perfis de provider
devon memory clear          → limpa memória semântica do projeto atual
```

---

## Branch principal de desenvolvimento

`go-migration` — toda feature nova vai aqui antes de ser mergeada em `main`.

---

## Estrutura de pacotes

```
cmd/devon/
  main.go               Entry point, define todos os subcomandos Cobra

internal/
  agent/
    agent.go            Loop principal: recebe tarefa → chama LLM → executa tools → persiste
    compact.go          Auto-compactação do histórico quando contexto fica grande
    context.go          buildSystemMessages: monta system prompt + DEVON.md + memória
    prompts/            System prompt base (texto estático)

  config/
    config.go           Struct Config, Load(), Doctor(), ParseMode(), ParseExecutionMode()
    profiles.go         Carrega devon.toml, resolve perfis de provider

  db/
    schema.go           DDL SQLite: todas as tabelas (ver seção abaixo)
    store.go            Interface Store + SQLiteStore (modernc.org/sqlite, WAL mode)

  memory/
    memory.go           Manager: Remember, Recall, Clear, ContextFor, ProjectIDFromWorkDir
    context.go          ContextFor com scoring de relevância por keyword + confiança
    tools.go            RememberTool ("remember") e RecallTool ("recall")
    memory_test.go      Testes com banco :memory:

  orchestrator/
    orchestrator.go     Coordena múltiplos agentes
    planner.go          Divide tarefa em subtarefas por agente
    scheduler.go        Executa agentes (sequential/parallel/async/pipeline)
    aggregator.go       Agrega resultados dos agentes

  llm/
    (cliente OpenAI-compatible, streaming)

  tools/
    bash.go             Executa comandos shell com timeout e sandbox
    read.go             Lê arquivos com suporte a range de linhas
    write.go            Escreve arquivos (cria diretórios se necessário)
    edit.go             Edita arquivo (str_replace simples)
    patch.go            PatchTool: replace_mode, patch_mode (GNU diff), insert_mode — atômico
    glob.go             Busca arquivos por padrão glob
    grep.go             Busca regex em arquivos
    list_dir.go         Lista diretório com filtros
    builtin.go          RegisterBuiltin, RegisterBuiltinWithConfig, RegisterMemoryTools

  permissions/          Checker de permissões: auto/safe/yolo, blocklist
  cost/                 Tracking de custo por sessão
  tui/                  Interface Bubble Tea (modo interativo)
```

---

## Schema do banco de dados (SQLite)

**Hot path** (escrita frequente):
- `sessions(id, created_at, status)`
- `messages(id, session_id, agent_id, role, content, timestamp)`
- `agent_states(agent_id, session_id, snapshot, updated_at)`
- `tool_calls(id, agent_id, session_id, tool_name, arguments, status, result, error, timestamp)`

**Cold path** (leitura sob demanda):
- `session_history(id, session_id, agent_id, role, content, archived_at)`
- `artifacts(key, session_id, data BLOB, created_at)`
- `cost_summary(id, session_id, total_cost, token_usage JSON, created_at)`

**Memória semântica**:
- `facts(id, project_id, category, content, context, confidence, created_at, updated_at)`
- `file_access(id, session_id, file_path, access_type, timestamp)`
- `error_patterns(id, project_id, pattern, context, occurrences, first_seen, last_seen)`

---

## Interface `db.Store` — métodos disponíveis

```go
// Sessions
CreateSession, GetSession, ListSessions

// Messages
PutMessage, GetMessages, SlidingWindow

// Agent State
PutAgentState, GetAgentState

// Tool Calls
PutToolCall, GetToolCalls

// History (cold)
ArchiveMessages, GetSessionHistory

// Artifacts
PutArtifact, GetArtifact

// Cost
GetCostSummary, UpdateCostSummary

// Facts (memória semântica)
PutFact, QueryFacts, ListFacts, DeleteFacts
GetFacts(ctx, projectID, category, limit)

// File access
RecordFileAccess, GetFileAccess

// Error patterns
PutErrorPattern, IncrementErrorPattern, GetErrorPatterns

// Pub/Sub
Subscribe, Publish

// Close
Close
```

> **Ao adicionar métodos novos na interface Store**, sempre adicionar stubs no
> `fakeDB` em `cmd/devon/main.go` para que o `go build` não quebre.

---

## Config struct — campos relevantes

```go
type Config struct {
    APIKey, BaseURL, Model string
    Mode      Mode          // auto | safe | yolo
    MaxTurns  int           // default 50
    Timeout   time.Duration // default 30s
    TurnDelay time.Duration

    ExecutionMode     ExecutionMode // sequential | parallel | async | pipeline
    MaxAgents         int           // default 4
    DBPath            string        // default ".devon/state.db"
    ContextWindowSize int           // default 20

    WorkDir    string  // os.Getwd() na inicialização
    ContextDoc string  // conteúdo de DEVON.md se existir
    Sandbox    SandboxConfig
}
```

Variáveis de ambiente: `DEVON_API_KEY`, `DEVON_BASE_URL`, `DEVON_MODEL`,
`DEVON_MODE`, `DEVON_MAX_TURNS`, `DEVON_TIMEOUT`, `DEVON_EXECUTION_MODE`,
`DEVON_MAX_AGENTS`, `DEVON_DB_PATH`, `DEVON_CONTEXT_WINDOW_SIZE`.

---

## Ferramentas disponíveis para o agente

| Nome | Arquivo | Descrição |
|---|---|---|
| `bash` | tools/bash.go | Shell com timeout e sandbox |
| `read_file` | tools/read.go | Lê arquivo com range de linhas |
| `write_file` | tools/write.go | Escreve/cria arquivo |
| `edit_file` | tools/edit.go | str_replace simples |
| `patch_file` | tools/patch.go | replace_mode / patch_mode / insert_mode (atômico) |
| `glob` | tools/glob.go | Busca por padrão glob |
| `grep` | tools/grep.go | Busca regex |
| `list_dir` | tools/list_dir.go | Lista diretório |
| `remember` | memory/tools.go | Salva fato semântico no banco |
| `recall` | memory/tools.go | Recupera fatos semânticos |

---

## Convenções do projeto

- **Zero CGO**: usar apenas `modernc.org/sqlite` (nunca `mattn/go-sqlite3`)
- **Sem panic** em código de produção
- **Erros sempre wrapped**: `fmt.Errorf("contexto: %w", err)`
- **Escrita atômica** em arquivos: `tempfile + os.Rename`
- **Testes de DB** sempre com `:memory:`: `db.New(":memory:")`
- Sem dependências externas de teste — apenas stdlib `testing`
- `go test ./...` deve passar sempre antes de commitar
- Sem `init()` nos pacotes

---

## Issues abertas (pendentes)

| # | Título | Prioridade sugerida |
|---|---|---|
| #20 | `devon init` wizard de setup | Baixa |
| #23 | Indexação semântica de codebase | Média |
| #24 | Cache de respostas por hash | Média |
| #28 | Multimodal input (imagens) | Baixa |
| #32 | MCP (Model Context Protocol) | Alta |
| #33 | Gerenciamento de sessões (CLI + TUI) | Alta |
| #34 | Extensão VSCode | Baixa |
| #49 | Servidor headless (gRPC/HTTP/SSE) | Alta |
| #50 | Roteamento de tarefas por tipo | Média |
| #51 | WebSearch (DuckDuckGo/Firecrawl) | Média |

---

## O que NÃO existe ainda (não tente importar)

- `internal/mcp/` — sem suporte a MCP ainda
- `internal/server/` — sem servidor headless
- `internal/cache/` — sem cache de respostas
- `internal/index/` — sem indexação semântica
- `internal/tools/web/` — sem web_search / web_fetch
- Subcomando `devon sessions` — apenas a infra DB existe
- Subcomando `devon init` — não existe

---

## Como rodar localmente

```bash
cp .env.example .env
# editar .env com DEVON_API_KEY e DEVON_MODEL

go run ./cmd/devon          # TUI interativa
go run ./cmd/devon run "sua tarefa aqui"
go test ./...               # rodar todos os testes
```

---

## Fluxo de uma tarefa (para entender o código)

1. `tui.Run(cfg)` ou `runOneShot()` → cria `db.New()`, `memory.New()`, `agent.New()`
2. `agent.New()` → chama `RegisterBuiltin` + `RegisterMemoryTools`, cria sessão no DB
3. `agent.Run(ctx, task)` → appenda mensagem do usuário, persiste no DB
4. Loop: chama `llm.Stream()` → recebe texto e/ou tool_calls
5. Para cada tool_call: verifica permissão → executa → persiste em `tool_calls`
6. Sem tool_calls → `turn_done`, persiste snapshot em `agent_states`, retorna
7. `memory.ContextFor()` é chamado em `buildSystemMessages()` para injetar fatos relevantes
