# Devon

[![CI](https://github.com/ElioNeto/devon/actions/workflows/ci.yml/badge.svg)](https://github.com/ElioNeto/devon/actions/workflows/ci.yml)

Agente de código com TUI, escrito em Go. Use qualquer LLM com API compatível com OpenAI — OpenRouter, Gemini, Groq, Ollama ou qualquer provider local.

---

## O que é o Devon

O Devon é um agente de código de linha de comando que:

- Roda inteiramente no terminal com uma **TUI** construída em [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- Conecta a **qualquer provider** com API compatível com OpenAI — sem lock-in
- Distribui como **binário único estático** — sem Node, sem npm, sem dependências
- Dá **visibilidade total** do que está acontecendo: cada tool call, cada arquivo tocado, em tempo real
- Respeita seu controle: modos `auto`, `safe` e `yolo` para permissões de execução

---

## Instalação

```bash
curl -fsSL https://raw.githubusercontent.com/ElioNeto/devon/main/install.sh | bash
```

Ou compile do fonte:

```bash
git clone https://github.com/ElioNeto/devon.git
cd devon
make build
```

---

## Início Rápido

### 1. Configure o provider

Crie um `.env` na raiz do projeto que quiser usar:

```bash
DEVON_API_KEY=sk-or-sua-chave-aqui
DEVON_BASE_URL=https://openrouter.ai/api/v1
DEVON_MODEL=mistralai/devstral-2512:free
```

### 2. Inicie

```bash
devon
```

Veja o [Playbook](docs/PLAYBOOK.md) e o [Guia de Configuração](docs/advanced-setup.md) para mais detalhes.

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

- **Filesystem:** `read_file`, `write_file`, `edit_file`, `list_dir`, `glob`, `grep`
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

## Roadmap

Veja as [issues abertas](https://github.com/ElioNeto/devon/issues) para o roadmap completo de funcionalidades planejadas.

---

## Licença

MIT.
