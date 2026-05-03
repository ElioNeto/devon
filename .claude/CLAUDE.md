# Devon — Instruções persistentes para o assistente

> Leia este arquivo inteiro antes de qualquer tarefa. É conciso por design.

---

## Comandos essenciais

```bash
go build ./...              # compila tudo (equivale a make build)
go test ./...               # roda todos os testes
go test ./internal/memory/... # testa apenas um pacote
go vet ./...                # análise estática
golangci-lint run           # lint completo
make build                  # gera binário ./devon
```

> Sempre rode `go build ./...` antes de declarar que uma tarefa está pronta.

---

## Estrutura de diretórios (resumida)

```
cmd/devon/main.go           Entry point: subcomandos Cobra (run, doctor, profiles, memory)
internal/
  agent/      Loop: buildSystemMessages → Stream → executeToolWithPermission
  config/     Config struct + Load() + profiles (devon.toml)
  db/         Interface Store + SQLiteStore (modernc.org/sqlite, WAL)
  llm/        Cliente HTTP SSE compatível com OpenAI
  memory/     Manager (Remember/Recall/Clear/ContextFor) + tools + context
  orchestrator/ Multi-agent: planner → scheduler → aggregator
  permissions/ Checker de permissões: auto/safe/yolo + blocklist
  tools/      bash, read_file, write_file, edit_file, patch_file, glob, grep, list_dir
  tui/        Interface Bubble Tea (modo interativo)
```

Branch de desenvolvimento ativo: **`go-migration`**

---

## Estilo de código

- `gofmt` e `goimports` — sem negociação
- Erros sempre com contexto: `fmt.Errorf("contexto: %w", err)`
- Sem `panic()` em código de produção
- Escrita atômica em arquivos: `os.CreateTemp` → escreve → `os.Rename`
- Sem `init()` nos pacotes
- Zero CGO: sempre `modernc.org/sqlite` (nunca `mattn/go-sqlite3`)
- Nomes de variáveis em camelCase, tipos exportados em PascalCase
- Interfaces pequenas e focadas (princípio de segregação)

---

## Testes

- Banco de dados: sempre `db.New(":memory:")` + `defer store.Close()`
- Sem dependências externas de teste — apenas stdlib `testing`
- Use `t.Fatalf` para erros fatais, `t.Errorf` para assertions
- Todo arquivo `*_test.go` usa `package <pkg>_test` (caixa preta)

---

## Interface Store — atenção crítica

Ao **adicionar métodos** na interface `db.Store` (`internal/db/store.go`),
obrigatoriamente adicione stubs correspondentes no `fakeDB` em
`cmd/devon/main.go`, caso contrário o `go build` quebra.

---

## Ferramentas do agente

| Tool | Arquivo | Observação |
|---|---|---|
| `bash` | tools/bash.go | Timeout via cfg; respeita sandbox blocklist |
| `read_file` | tools/read.go | Aceita `start_line`/`end_line` |
| `write_file` | tools/write.go | Cria diretórios automaticamente |
| `edit_file` | tools/edit.go | str_replace simples |
| `patch_file` | tools/patch.go | replace_mode / patch_mode / insert_mode |
| `glob` | tools/glob.go | Padrão glob relativo ao WorkDir |
| `grep` | tools/grep.go | Regex em arquivos |
| `list_dir` | tools/list_dir.go | Lista com filtros |
| `remember` | memory/tools.go | Salva fato semântico (PermRead) |
| `recall` | memory/tools.go | Recupera fatos por categoria/keyword |

---

## Modos de permissão

| Modo | Comportamento |
|---|---|---|
| `auto` | Pede confirmação apenas para ops destrutivas |
| `safe` | Pede confirmação para qualquer tool call |
| `yolo` | Executa tudo sem perguntar |

---

## O que NÃO existe (não tente importar)

- `internal/mcp/` — sem suporte MCP ainda
- `internal/server/` — sem servidor headless
- `internal/cache/` — sem cache de respostas
- `internal/index/` — sem indexação semântica de codebase
- `internal/tools/web/` — sem web_search / web_fetch
- Subcomando `devon sessions` — só infra DB existe
- Subcomando `devon init` — não implementado

---

## Variáveis de ambiente

```
DEVON_API_KEY          Chave do provider (obrigatório se não for local)
DEVON_BASE_URL         URL base da API (default: https://api.openai.com/v1)
DEVON_MODEL            Modelo a usar (obrigatório)
DEVON_MODE             auto | safe | yolo
DEVON_MAX_TURNS        Máximo de turnos por sessão (default: 50)
DEVON_TIMEOUT          Timeout por chamada em segundos (default: 30)
DEVON_EXECUTION_MODE   sequential | parallel | async | pipeline
DEVON_MAX_AGENTS       Máximo de agentes simultâneos (default: 4)
DEVON_DB_PATH          Caminho do SQLite (default: .devon/state.db)
DEVON_CONTEXT_WINDOW   Tamanho da janela de contexto (default: 20)
```
