# Roadmap

Este documento define a ordem de implementação das features, priorizando dependências técnicas e estabilidade do core antes de funcionalidades avançadas.

---

## ✅ Concluídas

| Issue | Título |
|---|---|
| [#1](https://github.com/ElioNeto/devon/issues/1) | Estrutura base Go |
| [#2](https://github.com/ElioNeto/devon/issues/2) | Sistema de configuração e providers |
| [#3](https://github.com/ElioNeto/devon/issues/3) | Loop do agente e sistema de ferramentas |
| [#5](https://github.com/ElioNeto/devon/issues/5) | Histórico de conversa: `/history /load /clear`, compactação, custo na statusbar |
| [#6](https://github.com/ElioNeto/devon/issues/6) | Modo de permissões (`checker.go`, `blocklist.go`, `audit.go`) |
| [#12](https://github.com/ElioNeto/devon/issues/12) | Modo one-shot `devon run` (não-interativo, stdin pipe, exit codes) |
| [#13](https://github.com/ElioNeto/devon/issues/13) | Interrupção segura Ctrl+C |
| [#16](https://github.com/ElioNeto/devon/issues/16) | Ferramentas de filesystem e shell (`read_file`, `write_file`, `edit_file`, `bash`, `glob`, `grep`, `list_dir`) |
| [#25](https://github.com/ElioNeto/devon/issues/25) | Loop autônomo do agente |
| [#26](https://github.com/ElioNeto/devon/issues/26) | Multi-agent / multi-model orchestration com SQLite embarcado |
| [#27](https://github.com/ElioNeto/devon/issues/27) | Bug teclas no input + filtragem no Command Palette |
| [#35](https://github.com/ElioNeto/devon/issues/35) | Remoção e limpeza do código legado Python/Node |
| [#36](https://github.com/ElioNeto/devon/issues/36) | Retry com backoff exponencial em HTTP 429/5xx |
| [#37](https://github.com/ElioNeto/devon/issues/37) | `data: [DONE]` tratado corretamente no `parseSSE` |
| [#38](https://github.com/ElioNeto/devon/issues/38) | `DEVON_MAX_LOOPS` configurável via env |
| [#39](https://github.com/ElioNeto/devon/issues/39) | System prompt orientado à entrega de artefatos |
| [#40](https://github.com/ElioNeto/devon/issues/40) | UX de permissões: confirm inline `[y/n/a]` + sumário de sessão |

---

## ⚠️ Em Progresso

### [#4 — TUI multi-painel](https://github.com/ElioNeto/devon/issues/4)

Layout, statusbar e command palette implementados. **Pendente:**
- `views/` com painéis dinâmicos por seleção
- `input.go` multi-linha com histórico de navegação
- Gráficos ASCII (barras + sparklines)
- Render de Markdown via Glamour

---

## 🔨 Próximas — Core

### 1. [#15 — Testes de integração do loop do agente](https://github.com/ElioNeto/devon/issues/15)
> Com ferramentas e permissões prontas, mocks cobrem o fluxo completo.
- `MockClient` e `MockTool` reutilizáveis
- Cenários: tool call simples, múltiplas calls, erro, cancelamento, `MaxTurns`

### 2. [#8 — Redução de consumo de tokens](https://github.com/ElioNeto/devon/issues/8)
> Otimizar sessões longas após histórico (#5) estar pronto.
- Sliding window no histórico
- Truncamento de resultados de tool calls
- Cache de leitura de arquivos por turno

### 3. [#19 — Sandbox de execução](https://github.com/ElioNeto/devon/issues/19)
> Complementa #6 com blocklist absoluta e limite de processos.
- Blocklist/allowlist configurável via `devon.toml`
- Timeout específico por padrão de comando
- `max_processes` para limitar processos simultâneos

### 4. [#9 — Multi-provider e multi-model](https://github.com/ElioNeto/devon/issues/9)
> Com retry (#36) e sandbox (#19) prontos, adicionar perfis e fallback entre providers.
- Perfis nomeados em `devon.toml`
- Fallback automático em erros 429/5xx
- `devon profiles list/test/add`

### 5. [#7 — Build, distribuição e instalação](https://github.com/ElioNeto/devon/issues/7)
> Com o core estável, formalizar o pipeline de release.
- `Makefile` completo com cross-compile
- GitHub Actions CI + release via GoReleaser
- `install.sh` com detecção automática de OS/arch

### 6. [#10 — Padronizar textos em pt-BR](https://github.com/ElioNeto/devon/issues/10)
> Varredura de strings após as features principais estarem implementadas.
- CLI, TUI, mensagens de erro e tool calls em pt-BR
- System prompt permanece em inglês

---

## 🔮 Futuro

| Issue | Título | Depende de |
|---|---|---|
| [#20](https://github.com/ElioNeto/devon/issues/20) | `devon init` — wizard para criar `DEVON.md` | #1 |
| [#22](https://github.com/ElioNeto/devon/issues/22) | Memória persistida com SQLite | #16 |
| [#23](https://github.com/ElioNeto/devon/issues/23) | Indexação semântica do codebase | #22 |
| [#24](https://github.com/ElioNeto/devon/issues/24) | Cache de respostas por hash de contexto | #22 |
| [#28](https://github.com/ElioNeto/devon/issues/28) | Multimodal input — imagens no prompt | #26 |
| [#31](https://github.com/ElioNeto/devon/issues/31) | `patch_file` / `str_replace_file` atômico | #16 |
| [#32](https://github.com/ElioNeto/devon/issues/32) | MCP — suporte a servidores externos de ferramentas | #16 |
| [#33](https://github.com/ElioNeto/devon/issues/33) | Gerenciamento de sessões | #22 |
| [#34](https://github.com/ElioNeto/devon/issues/34) | Protocolo extensão VSCode ↔ Devon | #9 |
