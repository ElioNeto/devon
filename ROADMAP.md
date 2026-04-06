# Roadmap de Desenvolvimento

Este documento define a ordem de implementação das issues abertas, priorizando dependências técnicas e segurança mínima antes de features avançadas.

---

## ✅ Concluídas

| Issue | Título |
|-------|--------|
| [#1](https://github.com/ElioNeto/devon/issues/1) | Estrutura base Go |
| [#2](https://github.com/ElioNeto/devon/issues/2) | Sistema de configuração e providers |
| [#3](https://github.com/ElioNeto/devon/issues/3) | Loop do agente e sistema de ferramentas |
| [#16](https://github.com/ElioNeto/devon/issues/16) | Ferramentas de filesystem e shell (read, write, edit, bash, glob, search, list_dir) |
| [#25](https://github.com/ElioNeto/devon/issues/25) | Loop autônomo do agente |

---

## ⚠️ Parcialmente implementadas (reabertas)

Issues que tinham código base mas acceptance criteria incompletos detectados na revisão de 06/04/2026.

### [#6 — Modo de Permissões](https://github.com/ElioNeto/devon/issues/6)
> `checker.go`, `blocklist.go`, `audit.go` implementados. **Pendente:** prompt inline de confirmação na TUI e sumário de sessão. Rastreado em [#40](https://github.com/ElioNeto/devon/issues/40).

### [#4 — TUI multi-painel](https://github.com/ElioNeto/devon/issues/4)
> Layout, statusbar e command palette implementados. **Pendente:** `views/` com painéis dinâmicos, `input.go` multi-linha, gráficos ASCII, render Markdown, integração com #5/#13/#22.

### [#5 — Histórico de conversa](https://github.com/ElioNeto/devon/issues/5)
> `internal/history/` existe. **Pendente:** persistência JSONL em `~/.devon/sessions/`, comandos `/history /load /clear`, compactação de contexto, rastreamento de custo.

### [#13 — Interrupção segura Ctrl+C](https://github.com/ElioNeto/devon/issues/13)
> Cancelamento via `ctx` existe no agente. **Pendente:** `cancelTurn` exposto para a TUI, SIGTERM→SIGKILL no `bash`, timer de 2s para duplo Ctrl+C, `/stop` no input.

### [#27 — Bug teclas / Command Palette](https://github.com/ElioNeto/devon/issues/27)
> `cmdmenu.go` existe. **Pendente:** correção do bug `space`, proteção das teclas `1-4/e/x` no input, `Ctrl+P` filtrável, `Ctrl+1-9` para sessões.

---

## 🔨 Em andamento / Próximas

### 1. [#40 — UX de permissões: confirm inline + sumário de sessão](https://github.com/ElioNeto/devon/issues/40)
> **Por quê agora:** Finaliza a #6. Depende apenas do `AuditLogger` e `Checker` já implementados.
- Prompt `[y] [n] [a]` inline na TUI com preview de conteúdo
- Sumário de ações ao encerrar sessão

---

### 2. [#27 — Bug teclas / Command Palette](https://github.com/ElioNeto/devon/issues/27)
> **Por quê agora:** Bug de UX crítico — teclas no input quebram navegação. Fix cirúrgico em `update.go`.
- Correção do `space` por guarda de foco
- Proteção das teclas `1-4/e/x` no input
- `Ctrl+P` com filtragem de comandos

---

### 3. [#13 — Interrupção segura Ctrl+C](https://github.com/ElioNeto/devon/issues/13)
> **Por quê agora:** Segurança básica do loop. Cancela turno sem perder sessão.
- `cancelTurn` exposto para a TUI
- SIGTERM → 3s → SIGKILL no `bash`
- Timer de duplo Ctrl+C
- `/stop` no input

---

### 4. [#5 — Histórico de conversa e contexto de projeto](https://github.com/ElioNeto/devon/issues/5)
> **Por quê agora:** Depende de #13 para salvar histórico antes de encerrar forçado.
- Persistência JSONL em `~/.devon/sessions/`
- Comandos `/history /load /clear`
- Compactação de contexto a 80%
- Rastreamento de custo na statusbar

---

### 5. [#4 — TUI multi-painel completa](https://github.com/ElioNeto/devon/issues/4)
> **Por quê agora:** Depende de #5 (histórico) e #13 (Ctrl+C) para painéis integrados.
- `views/` com painéis dinâmicos por seleção
- `input.go` multi-linha com histórico
- Gráficos ASCII (barras + sparklines)
- Render Markdown via Glamour

---

### 6. [#12 — Modo one-shot `devon run`](https://github.com/ElioNeto/devon/issues/12)
> **Por quê agora:** `internal/runner/` não existe. Com TUI e histórico prontos, one-shot é natural.
- `devon run "tarefa"` sem TUI
- Flag `--json` emitindo JSONL
- Stdin via pipe (`devon run -`)
- Exit codes corretos

---

### 7. [#15 — Testes de integração do loop do agente](https://github.com/ElioNeto/devon/issues/15)
> **Por quê agora:** Com ferramentas e permissões prontas, mocks cobrem o fluxo completo.
- `MockClient` e `MockTool` reutilizáveis
- Cenários: tool call simples, múltiplas calls, erro, cancelamento, MaxTurns

---

### 8. [#36 — Retry em HTTP 429 (Rate Limit)](https://github.com/ElioNeto/devon/issues/36)
> **Por quê agora:** Crítico para uso com modelos `:free` do OpenRouter.
- Backoff exponencial com leitura de `Retry-After`
- `DEVON_TURN_DELAY` entre turnos
- Evento `rate_limited` na TUI

---

### 9. [#19 — Sandbox de Execução](https://github.com/ElioNeto/devon/issues/19)
> **Por quê agora:** Complementa #6 com blocklist absoluta e limite de processos.
- Blocklist/allowlist configurável via `devon.toml`
- Timeout específico por padrão de comando
- `max_processes` para limitar processos simultâneos

---

### 10. [#9 — Multi-Provider e Multi-Model](https://github.com/ElioNeto/devon/issues/9)
> **Por quê agora:** Com retry e sandbox prontos, perfis e fallback entre providers.
- Perfis nomeados em `devon.toml`
- Fallback automático em erros 429/5xx
- `devon profiles list/test/add`

---

### 11. [#8 — Redução de Consumo de Tokens](https://github.com/ElioNeto/devon/issues/8)
> **Por quê agora:** Com multi-provider pronto, otimizar consumo para sessões longas.
- Sliding window no histórico
- Truncamento de resultados de tool calls
- Cache de leitura de arquivos por turno

---

### 12. [#37 — Fix: sseHandler nos testes não envia `data: [DONE]`](https://github.com/ElioNeto/devon/issues/37)
> **Por quê agora:** Bug de teste que pode mascarar falhas reais. Simples e rápido.
- Adicionar `data: [DONE]` no `sseHandler` de `agent_test.go`

---

### 13. [#38 — MaxTurns configurável via env](https://github.com/ElioNeto/devon/issues/38)
> **Por quê agora:** Complementa #8 e permite tarefas de maior escopo.
- Valor padrão maior (sugestão: 20)
- `DEVON_MAX_TURNS` como override

---

### 14. [#39 — Prompt do sistema orientado à entrega do artefato](https://github.com/ElioNeto/devon/issues/39)
> **Por quê agora:** Melhoria de qualidade direta no comportamento do agente.
- Reescrever `buildSystemMessages()` para focar na entrega
- Instruir o agente a listar arquivos criados/modificados ao finalizar

---

### 15. [#7 — Build, Distribuição e Instalação](https://github.com/ElioNeto/devon/issues/7)
> **Por quê agora:** Com o core estável, formalizar o pipeline de release.
- `Makefile` completo com cross-compile
- GitHub Actions CI + Release via GoReleaser
- `install.sh` com detecção automática de OS/arch

---

### 16. [#21 — CONTRIBUTING.md](https://github.com/ElioNeto/devon/issues/21)
> **Por quê agora:** Com CI pronto (#7), documentar o fluxo de contribuição.
- Setup local, convenções de código e commit
- Fluxo de PR e estrutura de pacotes

---

### 17. [#10 — Padronizar textos do terminal em pt-BR](https://github.com/ElioNeto/devon/issues/10)
> **Por quê agora:** Varredura de strings após as features principais estarem implementadas.
- CLI, TUI, mensagens de erro e tool calls em pt-BR
- System prompt permanece em inglês

---

## 🔮 Futuro (Após core estável)

| Issue | Título | Depende de |
|-------|--------|------------|
| [#20](https://github.com/ElioNeto/devon/issues/20) | `devon init` — wizard para criar DEVON.md | #1 |
| [#22](https://github.com/ElioNeto/devon/issues/22) | Memória persistida com SQLite | #16 |
| [#23](https://github.com/ElioNeto/devon/issues/23) | Indexação semântica do codebase | #22 |
| [#24](https://github.com/ElioNeto/devon/issues/24) | Cache de respostas por hash de contexto | #17, #22 |
| [#26](https://github.com/ElioNeto/devon/issues/26) | Multi-agent / Multi-model orchestration | #22 |
| [#28](https://github.com/ElioNeto/devon/issues/28) | Multimodal input — imagens no prompt | #26 |
| [#29](https://github.com/ElioNeto/devon/issues/29) | Migrar providers Python → Go | #9 |
| [#30](https://github.com/ElioNeto/devon/issues/30) | CI/CD completo (lint, coverage, security) | #7 |
| [#31](https://github.com/ElioNeto/devon/issues/31) | `write_file` com patch/diff atômico | #16 |
| [#32](https://github.com/ElioNeto/devon/issues/32) | MCP — suporte a servidores externos de ferramentas | #16 |
| [#33](https://github.com/ElioNeto/devon/issues/33) | Gerenciamento de sessões | #22 |
| [#34](https://github.com/ElioNeto/devon/issues/34) | Atualizar protocolo extensão VSCode | #29 |
| [#35](https://github.com/ElioNeto/devon/issues/35) | Remoção e limpeza do código legado | #29 |
