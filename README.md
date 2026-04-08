# Devon

<p align="center">
  <strong>Agente de código autônomo com TUI — escrito em Go, zero dependências.</strong>
</p>

<p align="center">
  <a href="https://github.com/ElioNeto/devon/actions/workflows/ci.yml"><img src="https://github.com/ElioNeto/devon/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/ElioNeto/devon/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <a href="https://github.com/ElioNeto/devon/releases"><img src="https://img.shields.io/github/v/release/ElioNeto/devon" alt="Release"></a>
</p>

---

Devon é um agente de código de linha de comando que roda inteiramente no terminal. Conecta a qualquer LLM com API compatível com OpenAI — sem lock-in, sem dependências externas, distribuído como binário único estático.

## Funcionalidades

- **TUI nativa** construída com [Bubble Tea](https://github.com/charmbracelet/bubbletea) — visibilidade total de cada tool call e arquivo tocado em tempo real
- **Qualquer provider** — OpenRouter, Gemini, Groq, Ollama, OpenAI, DeepSeek ou qualquer endpoint compatível com OpenAI
- **Binário único estático** — sem Node, sem Python, sem runtime externo
- **Controle de permissões** — modos `auto`, `safe` e `yolo` para diferentes níveis de autonomia
- **Modo não-interativo** — `devon run` para uso em scripts e CI/CD
- **Contexto de projeto** — lê `DEVON.md` automaticamente como system prompt adicional

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

## Início Rápido

**1. Configure o provider**

Crie um `.env` na raiz do projeto:

```bash
DEVON_API_KEY=sk-or-sua-chave-aqui
DEVON_BASE_URL=https://openrouter.ai/api/v1
DEVON_MODEL=mistralai/devstral-2512:free
```

**2. Inicie o Devon**

```bash
devon
```

Para configurações avançadas — múltiplos providers, perfis com fallback, modo não-interativo — veja o [Guia de Configuração Avançada](docs/advanced-setup.md).

## Providers Suportados

| Provider | Base URL | Modelos recomendados |
|---|---|---|
| [OpenRouter](https://openrouter.ai) | `https://openrouter.ai/api/v1` | `mistralai/devstral-2512:free`, `qwen/qwen3-coder:free` |
| Google Gemini | `https://generativelanguage.googleapis.com/v1beta/openai` | `gemini-2.5-flash` |
| Groq | `https://api.groq.com/openai/v1` | `llama-3.3-70b-versatile` |
| Ollama (local) | `http://localhost:11434/v1` | `qwen2.5-coder:32b`, `llama3.3:70b` |
| OpenAI | `https://api.openai.com/v1` | `gpt-4o` |
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |

## Ferramentas do Agente

Devon executa um loop `prompt → LLM → tool call → resultado → LLM` com as seguintes ferramentas:

| Categoria | Ferramentas |
|---|---|
| Filesystem | `read_file`, `write_file`, `edit_file`, `list_dir`, `glob`, `grep` |
| Shell | `bash` com timeout, captura de stdout/stderr e controle de permissão |
| Contexto | `DEVON.md` injetado como system prompt adicional |

## Controle de Permissões

| Modo | Comportamento |
|---|---|
| `auto` *(padrão)* | Leitura livre; escrita e shell pedem confirmação |
| `safe` | Toda ferramenta pede confirmação antes de executar |
| `yolo` | Execução autônoma sem interrupções |

```bash
devon --mode safe   # máximo controle
devon --mode yolo   # máxima velocidade
```

## Documentação

- [Guia de Configuração Avançada](docs/advanced-setup.md)
- [Playbook](docs/PLAYBOOK.md)
- [Início Rápido — macOS / Linux](docs/quick-start-mac-linux.md)
- [Início Rápido — Windows](docs/quick-start-windows.md)
- [Contribuindo](CONTRIBUTING.md)
- [Roadmap](ROADMAP.md)
- [Segurança](SECURITY.md)

## Licença

MIT — veja [LICENSE](LICENSE) para detalhes.
