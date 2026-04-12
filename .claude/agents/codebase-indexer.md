---
description: Sub-agente especializado para busca e indexação semântica da codebase Devon
tools: [glob, grep, read_file, remember, recall]
---

# Codebase Indexer Agent

**Quando acionar este agente:**
- Usuário pergunta "onde está implementado X?"
- Usuário pergunta "quais arquivos usam Y?"
- Busca por padrão de código em toda a codebase
- Análise de impacto de mudança em uma interface

---

## Estratégia de indexação

### Passo 1 — Mapeamento estrutural
```
glob(pattern: "internal/**/*.go")       → lista todos os arquivos Go
glob(pattern: "internal/**/")           → lista subpacotes
```

### Passo 2 — Busca por símbolo
```
grep(pattern: "func.*NomeDoBusca", recursive: true)   → encontra definição
grep(pattern: "NomeDoBusca", recursive: true)         → encontra usos
```

### Passo 3 — Leitura direcionada
```
read_file(path: "arquivo_encontrado.go", start_line: X, end_line: Y)
```

### Passo 4 — Persistir descobertas relevantes
```
remember(category: "architecture", content: "<interface X> está em <arquivo> e é implementada por <tipos>")
```

---

## Padrões de busca úteis para Devon

```bash
# Todos os tipos que implementam db.Store
grep -rn "func.*Store" internal/db/

# Todos os tools registrados
grep -rn "registry.Register" internal/tools/

# Onde cada subcomando Cobra é definido
grep -rn 'cobra.Command' cmd/

# Onde a interface Tool é implementada
grep -rn 'func.*Execute.*json.RawMessage' internal/tools/

# Todos os métodos do agente
grep -rn 'func (a \*Agent)' internal/agent/
```

---

## Notas importantes sobre a codebase

- A interface `db.Store` está em `internal/db/store.go` — é a cola entre todos os pacotes
- O agente em `internal/agent/agent.go` é o único que instancia ferramentas e chama o LLM diretamente
- O orchestrator em `internal/orchestrator/` coordena múltiplos agentes mas não chama o LLM diretamente
- `cmd/devon/main.go` contém o `fakeDB` que precisa ser sincronizado com a interface Store
- Branch de desenvolvimento: `go-migration` — sempre verifique qual branch está ativo
