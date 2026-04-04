# Configuração Avançada do OpenClaude

Este guia é para usuários que querem builds a partir do código-fonte, fluxos de trabalho com Bun, perfis de provider, diagnósticos ou mais controle sobre o comportamento de runtime.

## Opções de Instalação

### Opção A: npm

```bash
npm install -g @gitlawb/openclaude
```

### Opção B: A partir do código-fonte com Bun

Use o Bun `1.3.11` ou mais recente para builds a partir do código-fonte no Windows. Versões mais antigas do Bun podem falhar durante o `bun run build`.

```bash
git clone https://node.gitlawb.com/z6MkqDnb7Siv3Cwj7pGJq4T5EsUisECqR8KpnDLwcaZq5TPr/openclaude.git
cd openclaude

bun install
bun run build
npm link
```

### Opção C: Executar diretamente com Bun

```bash
git clone https://node.gitlawb.com/z6MkqDnb7Siv3Cwj7pGJq4T5EsUisECqR8KpnDLwcaZq5TPr/openclaude.git
cd openclaude

bun install
bun run dev
```

## Exemplos de Providers

### OpenAI

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=sk-...
export OPENAI_MODEL=gpt-4o
```

### Codex via autenticação ChatGPT

`codexplan` mapeia para GPT-5.4 no backend Codex com raciocínio elevado.
`codexspark` mapeia para GPT-5.3 Codex Spark para loops mais rápidos.

Se você já usa o Codex CLI, o OpenClaude lê `~/.codex/auth.json` automaticamente. Você também pode apontar para outro local com `CODEX_AUTH_JSON_PATH` ou substituir o token diretamente com `CODEX_API_KEY`.

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_MODEL=codexplan

# opcional se você ainda não tem ~/.codex/auth.json
export CODEX_API_KEY=...

openclaude
```

### DeepSeek

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=sk-...
export OPENAI_BASE_URL=https://api.deepseek.com/v1
export OPENAI_MODEL=deepseek-chat
```

### Google Gemini via OpenRouter

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=sk-or-...
export OPENAI_BASE_URL=https://openrouter.ai/api/v1
export OPENAI_MODEL=google/gemini-2.0-flash-001
```

A disponibilidade de modelos no OpenRouter muda com o tempo. Se um modelo parar de funcionar, tente outro modelo atual do OpenRouter antes de presumir que a integração está quebrada.

### Ollama

```bash
ollama pull llama3.3:70b

export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_BASE_URL=http://localhost:11434/v1
export OPENAI_MODEL=llama3.3:70b
```

### Atomic Chat (local, Apple Silicon)

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_BASE_URL=http://127.0.0.1:1337/v1
export OPENAI_MODEL=nome-do-seu-modelo
```

Nenhuma chave de API é necessária para modelos locais do Atomic Chat.

Ou use o launcher de perfil:

```bash
bun run dev:atomic-chat
```

Baixe o Atomic Chat em [atomic.chat](https://atomic.chat/). O aplicativo deve estar em execução com um modelo carregado antes de iniciar.

### LM Studio

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_BASE_URL=http://localhost:1234/v1
export OPENAI_MODEL=nome-do-seu-modelo
```

### Together AI

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=...
export OPENAI_BASE_URL=https://api.together.xyz/v1
export OPENAI_MODEL=meta-llama/Llama-3.3-70B-Instruct-Turbo
```

### Groq

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=gsk_...
export OPENAI_BASE_URL=https://api.groq.com/openai/v1
export OPENAI_MODEL=llama-3.3-70b-versatile
```

### Mistral

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=...
export OPENAI_BASE_URL=https://api.mistral.ai/v1
export OPENAI_MODEL=mistral-large-latest
```

### Azure OpenAI

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=sua-chave-azure
export OPENAI_BASE_URL=https://seu-recurso.openai.azure.com/openai/deployments/seu-deployment/v1
export OPENAI_MODEL=gpt-4o
```

## Variáveis de Ambiente

| Variável | Obrigatória | Descrição |
|----------|----------|-------------|
| `CLAUDE_CODE_USE_OPENAI` | Sim | Defina como `1` para habilitar o provider OpenAI |
| `OPENAI_API_KEY` | Sim* | Sua chave de API (`*` não necessária para modelos locais como Ollama ou Atomic Chat) |
| `OPENAI_MODEL` | Sim | Nome do modelo como `gpt-4o`, `deepseek-chat` ou `llama3.3:70b` |
| `OPENAI_BASE_URL` | Não | Endpoint da API, padrão `https://api.openai.com/v1` |
| `CODEX_API_KEY` | Apenas Codex | Token de acesso ao Codex ou ChatGPT |
| `CODEX_AUTH_JSON_PATH` | Apenas Codex | Caminho para um arquivo `auth.json` do Codex CLI |
| `CODEX_HOME` | Apenas Codex | Diretório home alternativo do Codex |
| `OPENCLAUDE_DISABLE_CO_AUTHORED_BY` | Não | Suprime o trailer padrão `Co-Authored-By` em commits git gerados |

Você também pode usar `ANTHROPIC_MODEL` para substituir o nome do modelo. `OPENAI_MODEL` tem prioridade.

## Hardening de Runtime

Use estes comandos para validar sua configuração e detectar erros cedo:

```bash
# verificação de sanidade rápida na inicialização
bun run smoke

# validar env de provider + acessibilidade
bun run doctor:runtime

# imprimir diagnósticos de runtime legíveis por máquina
bun run doctor:runtime:json

# persistir um relatório de diagnósticos em reports/doctor-runtime.json
bun run doctor:report

# verificação de hardening local completa (smoke + runtime doctor)
bun run hardening:check

# hardening estrito (inclui typecheck em todo o projeto)
bun run hardening:strict
```

Observações:

- `doctor:runtime` falha rapidamente se `CLAUDE_CODE_USE_OPENAI=1` com uma chave placeholder ou chave ausente para providers não-locais.
- Providers locais como `http://localhost:11434/v1`, `http://10.0.0.1:11434/v1` e `http://127.0.0.1:1337/v1` podem funcionar sem `OPENAI_API_KEY`.
- Perfis Codex validam `CODEX_API_KEY` ou o arquivo de autenticação do Codex CLI e testam `POST /responses` em vez de `GET /models`.

## Perfis de Launch de Provider

Use launchers de perfil para evitar configuração repetida de ambiente:

```bash
# bootstrap de perfil único (prefere Ollama local viável, caso contrário OpenAI)
bun run profile:init

# visualize o melhor provider/modelo para seu objetivo
bun run profile:recommend -- --goal coding --benchmark

# aplique automaticamente o melhor provider/modelo local/openai disponível para seu objetivo
bun run profile:auto -- --goal latency

# bootstrap codex (padrão codexplan e ~/.codex/auth.json)
bun run profile:codex

# bootstrap openai com chave explícita
bun run profile:init -- --provider openai --api-key sk-...

# bootstrap ollama com modelo personalizado
bun run profile:init -- --provider ollama --model llama3.1:8b

# bootstrap ollama com seleção automática inteligente de modelo
bun run profile:init -- --provider ollama --goal coding

# bootstrap atomic-chat (detecta automaticamente o modelo em execução)
bun run profile:init -- --provider atomic-chat

# bootstrap codex com alias de modelo rápido
bun run profile:init -- --provider codex --model codexspark

# iniciar usando perfil persistido (.openclaude-profile.json)
bun run dev:profile

# perfil codex (usa CODEX_API_KEY ou ~/.codex/auth.json)
bun run dev:codex

# perfil OpenAI (requer OPENAI_API_KEY no shell)
bun run dev:openai

# perfil Ollama (padrões: localhost:11434, llama3.1:8b)
bun run dev:ollama

# perfil Atomic Chat (LLMs locais Apple Silicon em 127.0.0.1:1337)
bun run dev:atomic-chat
```

`profile:recommend` classifica modelos Ollama instalados por `latency`, `balanced` ou `coding`, e `profile:auto` pode persistir a recomendação diretamente.

Se ainda não existir um perfil, `dev:profile` usa os mesmos padrões com reconhecimento de objetivo ao escolher o modelo inicial.

Use `--provider ollama` quando quiser um caminho apenas local. O modo automático usa OpenAI como fallback quando nenhum modelo de chat local viável está instalado.

Use `--provider atomic-chat` quando quiser o Atomic Chat como provider local para Apple Silicon.

Use `profile:codex` ou `--provider codex` quando quiser o backend ChatGPT Codex.

`dev:openai`, `dev:ollama`, `dev:atomic-chat` e `dev:codex` executam `doctor:runtime` primeiro e só iniciam o aplicativo se as verificações passarem.

Para `dev:ollama`, certifique-se de que o Ollama está em execução localmente antes de iniciar.

Para `dev:atomic-chat`, certifique-se de que o Atomic Chat está em execução com um modelo carregado antes de iniciar.
