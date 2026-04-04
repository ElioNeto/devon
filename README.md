# Devon

Agente de código com TUI, escrito em Go. Use qualquer LLM com API compatível com OpenAI — OpenRouter, Gemini, Groq, Ollama ou qualquer provider local.

> **Status:** Em reescrita ativa. A versão atual ainda usa a base TypeScript do OpenClaude. A versão Go com TUI está sendo desenvolvida nas [issues planejadas](https://github.com/ElioNeto/devon/issues).

---

## O que é o Devon

O Devon é um agente de código de linha de comando que:

- Roda inteiramente no terminal com uma **TUI** construída em [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- Conecta a **qualquer provider** com API compatível com OpenAI — sem lock-in
- Distribui como **binário único estático** — sem Node, sem npm, sem dependências
- Dá **visibilidade total** do que está acontecendo: cada tool call, cada arquivo tocado, em tempo real
- Respeita seu controle: modos `auto`, `safe` e `yolo` para permissões de execução

---

## Início Rápido (versão atual — TypeScript)

Enquanto a versão Go está sendo desenvolvida, a versão atual ainda usa a base TypeScript.

### 1. Instale as dependências

```bash
git clone https://github.com/ElioNeto/devon.git
cd devon
bun install
bun run build
npm link
```

### 2. Configure o provider

Crie um `.env` na raiz do projeto que quiser usar:

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=sk-or-sua-chave-aqui
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_MODEL=mistralai/devstral-2512:free
```

### 3. Inicie

```bash
# com .env local
set -a && source .env && set +a && openclaude

# ou com perfil persistido
bun run dev:profile
```

Veja o [Guia de Configuração](docs/advanced-setup.md) para todos os providers suportados.

---

## Providers Suportados

| Provider | Base URL | Modelos recomendados |
|---|---|---|
| [OpenRouter](https://openrouter.ai) | `https://openrouter.ai/api/v1` | `mistralai/devstral-2512:free`, `qwen/qwen3-coder:free` |
| Google Gemini | `https://generativelanguage.googleapis.com/v1beta/openai` | `gemini-2.5-flash` |
| Groq | `https://api.groq.com/openai/v1` | `llama-3.3-70b-versatile` |
| Ollama (local) | `http://localhost:11434/v1` | `llama3.3:70b`, `qwen2.5-coder:32b` |
| OpenAI | `https://api.openai.com/v1` | `gpt-4o` |
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |

---

## Ferramentas do Agente

O Devon executa um loop `prompt → LLM → tool call → resultado → LLM` com as seguintes ferramentas:

- **Filesystem:** `read_file`, `write_file`, `edit_file`, `list_dir`, `glob`, `search_files`
- **Shell:** `bash` com timeout, captura de stdout/stderr e controle de permissão
- **Contexto:** leitura de `DEVON.md` na raiz do projeto como system prompt adicional

---

## Controle de Permissões

| Modo | Comportamento |
|---|---|
| `auto` (padrão) | Leitura automática, escrita e shell pedem confirmação |
| `safe` | Toda ferramenta pede confirmação |
| `yolo` | Tudo executa sem perguntar |

```bash
devon --mode safe    # máximo controle
devon --mode yolo    # máxima velocidade
```

---

## Roadmap (versão Go)

- [#1 Estrutura base Go + Bubble Tea](https://github.com/ElioNeto/devon/issues/1)
- [#2 Sistema de config e providers](https://github.com/ElioNeto/devon/issues/2)
- [#3 Loop do agente e ferramentas](https://github.com/ElioNeto/devon/issues/3)
- [#4 TUI com visibilidade de tool calls em tempo real](https://github.com/ElioNeto/devon/issues/4)
- [#5 Histórico de conversas e contexto de projeto](https://github.com/ElioNeto/devon/issues/5)
- [#6 Modo de permissões e confirmações](https://github.com/ElioNeto/devon/issues/6)
- [#7 Build, distribuição e binário estático](https://github.com/ElioNeto/devon/issues/7)

---

## Licença

Código Go (Devon): MIT.

A base TypeScript atual é derivada do [openclaude](https://github.com/ElioNeto/openclaude), que por sua vez é um fork educacional do snapshot do Claude Code. O código original da Anthropic está sujeito aos termos da Anthropic. Este repositório não é afiliado nem endossado pela Anthropic.
