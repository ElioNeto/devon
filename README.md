# Devon

Agente de código para o terminal. Conecta a qualquer LLM com API compatível com OpenAI e executa tarefas de engenharia de software com visibilidade total de cada ação.

> **Status:** Versão atual funcional em TypeScript (Node.js). Reescrita em Go com TUI nativa em andamento na branch [`go-migration`](https://github.com/ElioNeto/devon/tree/go-migration).

---

## Como funciona

```mermaid
sequenceDiagram
    actor U as Usuário
    participant T as TUI (Terminal)
    participant A as Agente
    participant L as LLM
    participant F as Ferramentas

    U->>T: digita prompt
    T->>A: envia turno
    A->>L: histórico + tools disponíveis
    L-->>A: texto ou tool_call
    alt tool_call
        A->>F: executa ferramenta
        F-->>A: resultado
        A->>L: resultado da ferramenta
        L-->>A: resposta final
    end
    A-->>T: stream de eventos
    T-->>U: exibe resposta + tool calls
```

---

## Arquitetura (versão Go)

```mermaid
graph TD
    CLI["cmd/devon\nCLI · Cobra"] --> CFG["internal/config\nCarrega .env / devon.toml"]
    CLI --> TUI["internal/tui\nBubble Tea"]
    TUI --> AGT["internal/agent\nLoop do agente"]
    AGT --> LLM["internal/llm\nCliente HTTP SSE"]
    AGT --> REG["internal/tools\nRegistro de ferramentas"]
    AGT --> MEM["internal/memory\nSQLite — fatos e decisões"]
    AGT --> IDX["internal/index\nTF-IDF — busca no codebase"]
    AGT --> HST["internal/history\nHistórico de sessões"]
    REG --> FS["Filesystem\nread · write · edit · glob"]
    REG --> SH["Shell\nbash com timeout"]
    LLM --> CAC["internal/cache\nCache por hash de contexto"]

    style CLI fill:#1c1b19,color:#cdccca,stroke:#393836
    style TUI fill:#1c1b19,color:#cdccca,stroke:#393836
    style AGT fill:#01696f,color:#f9f8f4,stroke:#0c4e54
    style LLM fill:#1c1b19,color:#cdccca,stroke:#393836
    style REG fill:#1c1b19,color:#cdccca,stroke:#393836
    style MEM fill:#1c1b19,color:#cdccca,stroke:#393836
    style IDX fill:#1c1b19,color:#cdccca,stroke:#393836
    style HST fill:#1c1b19,color:#cdccca,stroke:#393836
    style FS fill:#1c1b19,color:#cdccca,stroke:#393836
    style SH fill:#1c1b19,color:#cdccca,stroke:#393836
    style CAC fill:#1c1b19,color:#cdccca,stroke:#393836
    style CFG fill:#1c1b19,color:#cdccca,stroke:#393836
```

---

## Funcionalidades

### Controle de permissões

```mermaid
stateDiagram-v2
    [*] --> Auto: padrão

    Auto --> Confirma: escrita ou shell
    Auto --> Executa: leitura

    Safe --> Confirma: qualquer ferramenta

    Yolo --> Executa: qualquer ferramenta

    Confirma --> Executa: usuário aprova
    Confirma --> Cancela: usuário recusa
    Executa --> [*]
    Cancela --> [*]
```

| Modo | Comportamento |
|---|---|
| `auto` (padrão) | Leitura livre · escrita e shell pedem confirmação |
| `safe` | Toda ferramenta pede confirmação |
| `yolo` | Executa tudo sem perguntar |

### Otimização de tokens

```mermaid
flowchart LR
    P[Prompt do usuário] --> MEM[(Memória\nSQLite)]
    P --> IDX[(Índice\nTF-IDF)]
    MEM -->|fatos relevantes| CTX[Contexto\nmontado]
    IDX -->|top-K arquivos| CTX
    CTX --> WIN[Sliding window\ndo histórico]
    WIN --> LLM[LLM]
    LLM --> CAC[(Cache\nde respostas)]
```

- **Memória persistida** — fatos e decisões do projeto entre sessões
- **Indexação semântica** — injeta só os arquivos relevantes *(opt-in)*
- **Compressão de histórico** — sliding window de N mensagens
- **Cache de respostas** — zero tokens para prompts repetidos *(modo one-shot)*

---

## Início Rápido (versão atual — TypeScript)

### Pré-requisitos

- [Bun](https://bun.sh) 1.3+
- Node.js 18+

### Instalação

```bash
git clone https://github.com/ElioNeto/devon.git
cd devon
bun install
bun run build
npm link
```

### Configuração

Crie um `.env` na raiz do projeto que quiser usar:

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=sk-or-sua-chave-aqui
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_MODEL=mistralai/devstral-2512:free
```

```bash
echo ".env" >> .gitignore
```

### Iniciar

```bash
# carregar .env e iniciar
set -a && source .env && set +a && devon

# se receber erro de heap (sessões longas)
NODE_OPTIONS="--max-old-space-size=8192" devon
```

> **Nota:** O erro `JavaScript heap out of memory` ocorre em sessões longas por limitação do runtime Node/V8. A versão Go elimina esse problema — veja [#1](https://github.com/ElioNeto/devon/issues/1).

---

## Providers suportados

| Provider | Base URL | Modelos recomendados |
|---|---|---|
| [OpenRouter](https://openrouter.ai) | `https://openrouter.ai/api/v1` | `mistralai/devstral-2512:free`, `qwen/qwen3-coder:free` |
| Google Gemini | `https://generativelanguage.googleapis.com/v1beta/openai` | `gemini-2.5-flash` |
| Groq | `https://api.groq.com/openai/v1` | `llama-3.3-70b-versatile` |
| Ollama (local) | `http://localhost:11434/v1` | `qwen2.5-coder:32b` |
| OpenAI | `https://api.openai.com/v1` | `gpt-4o` |
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |

Veja o [Guia de Configuração](docs/advanced-setup.md) para configuração detalhada de cada provider.

---

## Roadmap

```mermaid
gantt
    title Devon — Migração para Go
    dateFormat  YYYY-MM-DD
    axisFormat  %b

    section Base
    Estrutura Go + config + LLM client :done, 2026-04-04, 1d
    Ferramentas filesystem e shell     :active, fs, 2026-04-05, 7d
    Loop do agente                     :after fs, 5d

    section Interface
    TUI com Bubble Tea                 :tui, 2026-04-15, 14d
    Interrupção segura Ctrl+C          :after tui, 3d

    section Memória e contexto
    Histórico de sessões (SQLite)      :2026-04-20, 7d
    Memória persistida                 :2026-04-25, 7d
    Indexação semântica (opt-in)       :2026-05-01, 10d
    Cache de respostas                 :2026-05-05, 5d

    section Providers e config
    Multi-provider + perfis            :2026-04-20, 10d
    Redução de tokens                  :2026-04-25, 7d

    section Distribuição
    Build estático + goreleaser        :2026-05-10, 5d
```

| Issue | Título | Status |
|---|---|---|
| [#1](https://github.com/ElioNeto/devon/issues/1) | Estrutura base Go | ✅ Implementada |
| [#4](https://github.com/ElioNeto/devon/issues/4) | TUI com Bubble Tea | 🔲 Planejada |
| [#5](https://github.com/ElioNeto/devon/issues/5) | Histórico de sessões | 🔲 Planejada |
| [#6](https://github.com/ElioNeto/devon/issues/6) | Sistema de permissões | 🔲 Planejada |
| [#7](https://github.com/ElioNeto/devon/issues/7) | Build e distribuição | 🔲 Planejada |
| [#8](https://github.com/ElioNeto/devon/issues/8) | Redução de tokens | 🔲 Planejada |
| [#9](https://github.com/ElioNeto/devon/issues/9) | Multi-provider e perfis | 🔲 Planejada |
| [#16](https://github.com/ElioNeto/devon/issues/16) | Ferramentas filesystem e shell | 🔲 Planejada |
| [#17](https://github.com/ElioNeto/devon/issues/17) | Modo one-shot `devon run` | 🔲 Planejada |
| [#18](https://github.com/ElioNeto/devon/issues/18) | Interrupção segura Ctrl+C | 🔲 Planejada |
| [#19](https://github.com/ElioNeto/devon/issues/19) | Sandbox de execução | 🔲 Planejada |
| [#20](https://github.com/ElioNeto/devon/issues/20) | `devon init` wizard | 🔲 Planejada |
| [#22](https://github.com/ElioNeto/devon/issues/22) | Memória com SQLite | 🔲 Planejada |
| [#23](https://github.com/ElioNeto/devon/issues/23) | Indexação semântica | 🔲 Planejada |
| [#24](https://github.com/ElioNeto/devon/issues/24) | Cache de respostas | 🔲 Planejada |

---

## Licença

Código Go (Devon): MIT.

A base TypeScript atual é derivada do [openclaude](https://github.com/ElioNeto/openclaude), fork educacional de um snapshot do Claude Code. O código original da Anthropic está sujeito aos termos da Anthropic. Este repositório não é afiliado nem endossado pela Anthropic.
