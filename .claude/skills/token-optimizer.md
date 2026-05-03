---
description: Estratégias de otimização de tokens usando os mecanismos nativos do Devon
---

# Token Optimizer — Devon

O Devon possui três mecanismos nativos para reduzir consumo de tokens.
Use-os em conjunto para sessões longas.

---

## 1. Sliding Window (compactação automática)

**Onde:** `internal/agent/agent.go` → `compactIfNeeded()`
**Como funciona:** O agente estima o tamanho do histórico em tokens antes de cada
chamada ao LLM. Se ultrapassar o limite do modelo, compacta automaticamente
removendo as mensagens mais antigas (mantendo system prompt + últimas N mensagens).

**Config:**
```bash
DEVON_CONTEXT_WINDOW_SIZE=20  # número de mensagens na janela (default: 20)
```

**Quando ativar compactação manual:**
```go
// Em db.Store
store.SlidingWindow(ctx, agentID, sessionID, windowSize)
```

---

## 2. Memória Semântica (fatos persistentes)

**Onde:** `internal/memory/`
**Como funciona:** Em vez de repetir contexto longo no início de cada sessão,
o agente persiste fatos relevantes no SQLite e os injeta apenas quando relevantes
para o prompt atual (via scoring de keyword + confidence).

**Benefício para tokens:** Um fato de 20 palavras substitui parágrafos de contexto
que seriam repetidos em cada sessão.

**Uso pelo LLM:**
```
remember(category: "convention", content: "usar fmt.Errorf com %w")
recall(category: "architecture")  → retorna apenas fatos relevantes
```

**Injeção no system prompt:** Automática via `Manager.ContextFor()` em
`agent.buildSystemMessages()` — apenas fatos com score de relevância > 0 são incluídos.

---

## 3. DEVON.md (contexto do projeto)

**Onde:** raiz do projeto
**Como funciona:** Carregado uma vez por `config.Load()` e injetado no system
prompt. Substitui prompts inline repetitivos.

**Boas práticas para manter DEVON.md enxuto:**
- Máximo 200 linhas
- Sem código-fonte — apenas decisões e convenções
- Atualize-o quando mudar arquitetura; delete seções obsoletas
- Facts muito específicos → use `remember` em vez do DEVON.md

---

## Guia de quando usar cada mecanismo

| Tipo de informação | Mecanismo recomendado |
|---|---|
| Convenções permanentes do projeto | `remember(category: "convention", ...)` |
| Decisões arquiteturais | `remember(category: "architecture", ...)` |
| Erros recorrentes já resolvidos | `remember(category: "error", ...)` |
| Contexto da sessão atual | Sliding window (automático) |
| Overview do projeto para o LLM | `DEVON.md` |
| Tasks one-shot sem memória | `devon run` (usa fakeDB em memória) |

---

## Estimativa de tokens por mecanismo

- DEVON.md (200 linhas): ~1.500 tokens (carregado uma vez, fixo)
- Fato médio de memória (20 palavras): ~30 tokens vs ~300 tokens de contexto inline
- Sliding window com 20 mensagens: ~4.000–8.000 tokens vs histórico completo (40k+)
