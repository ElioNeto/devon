---
description: Detalhes sobre as ferramentas do agente Devon (filesystem, shell, memória)
paths: "internal/tools/**"
---

# Ferramentas do agente — Devon

## Registro de ferramentas

As ferramentas são registradas em `tools/builtin.go`:

```go
// Ferramentas de filesystem e shell
tools.RegisterBuiltin(registry, cfg.WorkDir, cfg.Timeout, cfg.Sandbox)

// Ferramentas de memória semântica
tools.RegisterMemoryTools(registry, mem, projectID)
```

`RegisterMemoryTools` é no-op se `mem == nil` (modo one-shot sem banco).

## Interface Tool

Toda ferramenta implementa:
```go
type Tool interface {
    Name()        string
    Description() string
    Schema()      json.RawMessage  // JSON Schema dos parâmetros
    Execute(ctx context.Context, params json.RawMessage) (string, error)
    Permission()  permissions.Level
}
```

## Ferramentas disponíveis

### bash
- Executa comandos shell com `exec.CommandContext` (respeita `ctx`)
- Timeout vem de `cfg.Timeout` (padrão 30s)
- Sandbox: comandos na blocklist são recusados antes da execução
- Stdout + stderr concatenados no resultado
- Permission: `PermWrite` (requer confirmação em modo `auto` se destrutivo)

### read_file
- Parâmetros: `path` (obrigatório), `start_line` e `end_line` (opcionais)
- Lê arquivo completo ou trecho; retorna conteúdo como string
- Permission: `PermRead`

### write_file
- Parâmetros: `path`, `content`
- Cria diretórios pai automaticamente (`os.MkdirAll`)
- Escrita atômica: `os.CreateTemp` + `os.Rename`
- Permission: `PermWrite`

### edit_file
- Parâmetros: `path`, `old_str`, `new_str`
- str_replace simples — substitui primeira ocorrência
- Falha se `old_str` não for encontrado
- Permission: `PermWrite`

### patch_file
- Três modos: `replace_mode`, `patch_mode` (GNU diff), `insert_mode`
- Escrita atômica
- Permission: `PermWrite`

### glob
- Parâmetros: `pattern` (relativo ao `WorkDir`)
- Retorna lista de caminhos que casam com o padrão
- Permission: `PermRead`

### grep
- Parâmetros: `pattern` (regex), `path` (opcional), `recursive` (bool)
- Busca regex em arquivos; retorna matches com número de linha
- Permission: `PermRead`

### list_dir
- Parâmetros: `path`, `show_hidden` (bool), `max_depth` (int)
- Lista arquivos e diretórios com metadados
- Permission: `PermRead`

### remember
- Parâmetros: `category` (string), `content` (string)
- Salva fato no SQLite via `Manager.Remember`
- Permission: `PermRead` (não modifica o filesystem, apenas o banco)
- No-op em modo one-shot (manager nil)

### recall
- Parâmetros: `category` (string, opcional), `keyword` (string, opcional)
- Recupera fatos por categoria e/ou keyword
- Permission: `PermRead`

## Sistema de permissões

```go
const (
    PermRead    Level = iota  // Leitura — sempre permitido
    PermWrite                 // Escrita — confirmação em modo auto se destrutivo
    PermExecute               // Execução — confirmação em modo safe
    PermDestruct              // Destrutivo — confirmação em todos os modos exceto yolo
)
```

O `Checker` mantém uma sessão de aprovações (`Session map[string]bool`):
- Approve(toolName) → adiciona à sessão, não pede mais confirmação para aquela tool
- Blocklist permanente: comandos listados em `permissions.DefaultBlocklist`

## Adicionar nova ferramenta

1. Crie `internal/tools/<nome>.go` implementando a interface `Tool`
2. Registre em `internal/tools/builtin.go` dentro de `RegisterBuiltin`
3. Adicione no `fakeDB` de `cmd/devon/main.go` se a tool precisar do Store
4. Adicione testes em `internal/tools/<nome>_test.go`
5. Documente na tabela do `CLAUDE.md`
