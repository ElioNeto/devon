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
| [#12](https://github.com/ElioNeto/devon/issues/12) | Modo one-shot `devon run` (não-interativo, stdin pipe, exit codes) |
| [#13](https://github.com/ElioNeto/devon/issues/13) | Interrupção segura Ctrl+C (cancela turno na TUI, SIGINT no `run`) |
| [#37](https://github.com/ElioNeto/devon/issues/37) | `data: [DONE]` tratado corretamente no `parseSSE` |
| [#38](https://github.com/ElioNeto/devon/issues/38) | `DEVON_MAX_TURNS` configurável via env (padrão 50) |
| [#39](https://github.com/ElioNeto/devon/issues/39) | System prompt orientado à entrega do artefato (`buildSystemMessages`) |
| [#36](https://github.com/ElioNeto/devon/issues/36) | Retry com backoff exponencial em HTTP 429/5xx (`DEVON_TURN_DELAY`) |
| [#27](https://github.com/ElioNeto/devon/issues/27) | Bug teclas no input + filtragem no Command Palette (`!`) |
| [#6](https://github.com/ElioNeto/devon/issues/6) | Modo de Permissões (`checker.go`, `blocklist.go`, `audit.go`) |
| [#40](https://github.com/ElioNeto/devon/issues/40) | UX de permissões: confirm inline `[y/n/a]` + sumário de sessão |

---

## ⚠️ Parcialmente implementadas (pendências conhecidas)

### [#4 — TUI multi-painel](https://github.com/ElioNeto/devon/issues/4)
> Layout, statusbar e command palette implementados. **Pendente:** `views/` com painéis dinâmicos, `input.go` multi-linha, gráficos ASCII, render Markdown, integração com #5/#22.

### [#5 — Histórico de conversa](https://github.com/ElioNeto/devon/issues/5)
> `internal/history/` existe. **Pendente:** persistência JSONL em `~/.devon/sessions/`, comandos `/history /load /clear`, compactação de contexto, rastreamento de custo.

---

## 🔨 Em andamento / Próximas

### 1. [#5 — Histórico de conversa e contexto de projeto](https://github.com/ElioNeto/devon/issues/5)
> **Por quê agora:** Base para sessões persistentes e recuperação após crash.
- Persistência JSONL em `~/.devon/sessions/`
- Comandos `/history /load /clear`
- Compactação de contexto a 80%
- Rastreamento de custo na statusbar

---

### 2. [#4 — TUI multi-painel completa](https://github.com/ElioNeto/devon/issues/4)
> **Por quê agora:** Depende de #5 (histórico) para painéis integrados.
- `views/` com painéis dinâmicos por seleção
- `input.go` multi-linha com histórico
- Gráficos ASCII (barras + sparklines)
- Render Markdown via Glamour

---

### 3. [#15 — Testes de integração do loop do agente](https://github.com/ElioNeto/devon/issues/15)
> **Por quê agora:** Com ferramentas e permissões prontas, mocks cobrem o fluxo completo.
- `MockClient` e `MockTool` reutilizáveis
- Cenários: tool call simples, múltiplas calls, erro, cancelamento, MaxTurns

---

### 4. [#8 — Redução de Consumo de Tokens](https://github.com/ElioNeto/devon/issues/8)
> **Por quê agora:** Otimizar consumo para sessões longas após histórico pronto (#5).
- Sliding window no histórico
- Truncamento de resultados de tool calls
- Cache de leitura de arquivos por turno

---

### 5. [#19 — Sandbox de Execução](https://github.com/ElioNeto/devon/issues/19)
> **Por quê agora:** Complementa #6 com blocklist absoluta e limite de processos.
- Blocklist/allowlist configurável via `devon.toml`
- Timeout específico por padrão de comando
- `max_processes` para limitar processos simultâneos

---

### 6. [#9 — Multi-Provider e Multi-Model](https://github.com/ElioNeto/devon/issues/9)
> **Por quê agora:** Com retry (#36) e sandbox (#19) prontos, perfis e fallback entre providers.
- Perfis nomeados em `devon.toml`
- Fallback automático em erros 429/5xx
- `devon profiles list/test/add`

---

### 7. [#7 — Build, Distribuição e Instalação](https://github.com/ElioNeto/devon/issues/7)
> **Por quê agora:** Com o core estável, formalizar o pipeline de release.
- `Makefile` completo com cross-compile
- GitHub Actions CI + Release via GoReleaser
- `install.sh` com detecção automática de OS/arch

---

### 8. [#21 — CONTRIBUTING.md](https://github.com/ElioNeto/devon/issues/21)
> **Por quê agora:** Com CI pronto (#7), documentar o fluxo de contribuição.
- Setup local, convenções de código e commit
- Fluxo de PR e estrutura de pacotes

---

### 9. [#10 — Padronizar textos do terminal em pt-BR](https://github.com/ElioNeto/devon/issues/10)
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
