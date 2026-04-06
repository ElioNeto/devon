# Roadmap de Desenvolvimento

Este documento define a ordem de implementação das issues abertas, priorizando dependências técnicas e segurança mínima antes de features avançadas.

---

## ✅ Concluídas

| Issue | Título |
|-------|--------|
| [#1](https://github.com/ElioNeto/devon/issues/1) | Estrutura base Go |
| [#16](https://github.com/ElioNeto/devon/issues/16) | Ferramentas de filesystem e shell (read, write, edit, bash, glob, search, list_dir) |

---

## 🔨 Em andamento / Próximas

### 1. [#6 — Modo de Permissões e Confirmação de Ações Destrutivas](https://github.com/ElioNeto/devon/issues/6)
> **Por quê agora:** `bash` e `write` já existem mas executam sem controle. Esta issue adiciona a camada `Checker` entre o agente e as ferramentas.
- Modos `auto`, `yolo`, `safe`
- Prompt inline de confirmação na TUI
- Blocklist padrão embutida
- Audit log em `~/.devon/audit.log`

---

### 2. [#15 — Testes de Integração do Loop do Agente](https://github.com/ElioNeto/devon/issues/15)
> **Por quê agora:** Com ferramentas e permissões prontas, os mocks cobrem o fluxo completo. CI confiável antes de crescer.
- `MockClient` e `MockTool` reutilizáveis
- Cenários: tool call simples, múltiplas calls, erro, cancelamento, MaxTurns
- Testes sem conexão externa

---

### 3. [#17 — Modo One-Shot `devon run`](https://github.com/ElioNeto/devon/issues/17)
> **Por quê agora:** Habilita uso em scripts, CI e hooks de git. Depende de #6 (modo `yolo` como padrão sem TTY).
- `devon run "tarefa"` sem TUI
- Output limpo no stdout, tool calls no stderr
- Exit codes corretos (0/1/2/130)
- Suporte a stdin via pipe

---

### 4. [#18 — Interrupção Segura Ctrl+C](https://github.com/ElioNeto/devon/issues/18)
> **Por quê agora:** Com `bash` matando process groups e o loop do agente funcionando, o cancelamento pode ser implementado corretamente.
- `Ctrl+C` cancela o turno sem sair do Devon
- Duplo `Ctrl+C` em < 2s encerra
- Histórico preservado após cancelamento

---

### 5. [#36 — Retry em HTTP 429 (Rate Limit)](https://github.com/ElioNeto/devon/issues/36)
> **Por quê agora:** Crítico para uso com modelos `:free` do OpenRouter. Sem retry, qualquer sessão longa falha silenciosamente.
- Backoff exponencial com leitura de `Retry-After`
- `DEVON_TURN_DELAY` entre turnos
- Evento `rate_limited` na TUI

---

### 6. [#19 — Sandbox de Execução](https://github.com/ElioNeto/devon/issues/19)
> **Por quê agora:** Complementa #6 com blocklist absoluta, timeout por padrão de comando e limite de processos paralelos.
- Blocklist/allowlist configurável via `devon.toml`
- Timeout específico por padrão de comando
- `max_processes` para limitar processos simultâneos

---

### 7. [#9 — Multi-Provider e Multi-Model](https://github.com/ElioNeto/devon/issues/9)
> **Por quê agora:** Com retry e sandbox prontos, adicionar perfis e fallback entre providers é o próximo salto de robustez.
- Perfis nomeados em `devon.toml`
- Fallback automático em erros 429/5xx
- `devon profiles list/test/add`

---

### 8. [#8 — Redução de Consumo de Tokens](https://github.com/ElioNeto/devon/issues/8)
> **Por quê agora:** Com multi-provider pronto, otimizar o consumo é o próximo passo natural para sessões longas.
- Sliding window no histórico (`DEVON_MAX_HISTORY_MESSAGES`)
- Truncamento de resultados de tool calls
- Cache de leitura de arquivos por turno
- Estimador de tokens sem lib externa

---

### 9. [#37 — Fix: sseHandler nos testes não envia `data: [DONE]`](https://github.com/ElioNeto/devon/issues/37)
> **Por quê agora:** Bug de teste que pode mascarar falhas reais. Simples e rápido.
- Adicionar `data: [DONE]` no `sseHandler` de `agent_test.go`

---

### 10. [#38 — MaxTurns configurável via env](https://github.com/ElioNeto/devon/issues/38)
> **Por quê agora:** Complementa #8 e permite tarefas de maior escopo sem alterar o código.
- Valor padrão maior (sugestão: 20)
- `DEVON_MAX_TURNS` como override

---

### 11. [#39 — Prompt do sistema orientado à entrega do artefato](https://github.com/ElioNeto/devon/issues/39)
> **Por quê agora:** Melhoria de qualidade direta no comportamento do agente. Pequena e impactante.
- Reescrever `buildSystemMessages()` para focar na entrega do artefato
- Instruir o agente a listar arquivos criados/modificados ao finalizar

---

### 12. [#7 — Build, Distribuição e Instalação](https://github.com/ElioNeto/devon/issues/7)
> **Por quê agora:** Com o core estável, formalizar o pipeline de release e o script de instalação.
- `Makefile` completo com cross-compile
- GitHub Actions CI + Release via GoReleaser
- `install.sh` com detecção automática de OS/arch
- `//go:embed` do system prompt

---

### 13. [#21 — CONTRIBUTING.md](https://github.com/ElioNeto/devon/issues/21)
> **Por quê agora:** Com CI pronto (#7), documentar o fluxo de contribuição faz sentido.
- Setup local, convenções de código e commit
- Fluxo de PR e estrutura de pacotes

---

### 14. [#10 — Padronizar textos do terminal em pt-BR](https://github.com/ElioNeto/devon/issues/10)
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
