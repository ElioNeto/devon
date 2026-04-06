# Configuração Avançada do Devon

Este guia cobre configuração de providers, perfis por projeto, diagnósticos e opções de runtime.

---

## Instalação

### Opção A: npm (versão atual TypeScript)

```bash
git clone https://github.com/ElioNeto/devon.git
cd devon
bun install
bun run build
npm link
```

### Opção B: Executar direto com Bun

```bash
git clone https://github.com/ElioNeto/devon.git
cd devon
bun install
bun run dev
```

> **Em breve:** Versão Go com binário único estático. Sem Node, sem Bun, sem dependências. Veja [#7](https://github.com/ElioNeto/devon/issues/7).

---

## Configuração de Provider

O Devon conecta a qualquer API compatível com OpenAI. Configure via variáveis de ambiente ou arquivo `.env` local.

### Usando `.env` (recomendado)

Crie um `.env` na raiz do projeto que quiser usar:

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=sk-or-sua-chave-aqui
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_MODEL=mistralai/devstral-2512:free
```

Certifique-se que `.env` está no `.gitignore`:

```bash
echo ".env" >> .gitignore
```

Inicie carregando o `.env`:

```bash
set -a && source .env && set +a && devon
```

---

## Exemplos de Providers

### OpenRouter (modelos gratuitos)

Crie sua chave em [openrouter.ai/keys](https://openrouter.ai/keys) — sem cartão de crédito.

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=sk-or-sua-chave-aqui
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_MODEL=mistralai/devstral-2512:free
```

Modelos gratuitos recomendados para código:

| Modelo | Contexto | Destaque |
|---|---|---|
| `mistralai/devstral-2512:free` | 262K | Melhor para código |
| `qwen/qwen3-coder:free` | 262K | Forte em tool use |
| `deepseek/deepseek-r1:free` | 128K | Raciocínio em código |

### Google Gemini

Chave gratuita em [aistudio.google.com](https://aistudio.google.com).

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=AIzaSy-sua-chave-aqui
OPENAI_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai
OPENAI_MODEL=gemini-2.5-flash
```

### DeepSeek

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=sk-sua-chave-aqui
OPENAI_BASE_URL=https://api.deepseek.com/v1
OPENAI_MODEL=deepseek-chat
```

### Groq

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=gsk_sua-chave-aqui
OPENAI_BASE_URL=https://api.groq.com/openai/v1
OPENAI_MODEL=llama-3.3-70b-versatile
```

### Ollama (local)

```bash
ollama pull qwen2.5-coder:32b

CLAUDE_CODE_USE_OPENAI=1
OPENAI_BASE_URL=http://localhost:11434/v1
OPENAI_MODEL=qwen2.5-coder:32b
# OPENAI_API_KEY não é necessário para modelos locais
```

### LM Studio

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_BASE_URL=http://localhost:1234/v1
OPENAI_MODEL=nome-do-seu-modelo
```

### OpenAI

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=sk-sua-chave-aqui
OPENAI_MODEL=gpt-4o
```

### Azure OpenAI

```bash
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=sua-chave-azure
OPENAI_BASE_URL=https://seu-recurso.openai.azure.com/openai/deployments/seu-deployment/v1
OPENAI_MODEL=gpt-4o
```

---

## Configuração para OpenRouter :free

Os modelos gratuitos do OpenRouter (`:free`) têm limites rígidos de RPM e podem retornar **429** (rate limit) com frequência. O Devon já faz retry automático com backoff exponencial, mas duas variáveis ajudam a evitar os rate limits antes que aconteçam:

```bash
# Pausa entre turnos do agente (evita rajadas rápidas)
DEVON_TURN_DELAY=2s

# Máximo de turnos por conversa (padrão: 50)
DEVON_MAX_TURNS=30
```

- `DEVON_TURN_DELAY` — tempo de espera entre cada turno do agente. Use `2s` a `5s` para modelos `:free`. Aceita sufixos `s`, `m` (ex: `30s`, `1m`).
- `DEVON_MAX_TURNS` — limite de iterações LLM por conversa. Reduzir evita consumo excessivo em loops longos.

Isso somado ao retry automático (até 5 tentativas com backoff exponecial para 429/5xx) permite usar modelos gratuitos sem intervenção manual.

---

## BYOK (Bring Your Own Key)

O Devon não armazena ou envia sua chave para nenhum serviço além do provider configurado. Todo tráfego vai direto do seu terminal para o endpoint da API (definido em `DEVON_BASE_URL`). Não há telemetria, tracking, ou intermediários.

---

## Variáveis de Ambiente

| Variável | Obrigatória | Descrição |
|---|---|---|
| `CLAUDE_CODE_USE_OPENAI` | Sim | Defina como `1` para habilitar provider OpenAI-compatible |
| `OPENAI_API_KEY` | Sim* | Chave de API (`*` não necessária para modelos locais) |
| `OPENAI_MODEL` | Sim | Nome do modelo (ex: `mistralai/devstral-2512:free`) |
| `OPENAI_BASE_URL` | Não | Endpoint da API. Padrão: `https://api.openai.com/v1` |

---

## Perfis por Projeto

O Devon salva um perfil local para não precisar configurar o ambiente toda vez:

```bash
# inicializar perfil com OpenRouter
bun run profile:init -- --provider openai --api-key sk-or-... --model mistralai/devstral-2512:free

# inicializar com Ollama
bun run profile:init -- --provider ollama --model qwen2.5-coder:32b

# iniciar usando perfil salvo (.devon-profile.json)
bun run dev:profile
```

O arquivo `.devon-profile.json` é criado na raiz do projeto e já está no `.gitignore` por padrão.

---

## Contexto de Projeto (DEVON.md)

Crie um `DEVON.md` na raiz do projeto para dar contexto permanente ao agente:

```markdown
# Contexto do Projeto

- Stack: Go 1.22, PostgreSQL, gRPC
- Convenções: todos os erros devem ser wrapped com `fmt.Errorf("...: %w", err)`
- Testes: usar `testify/assert`, sem mocks de terceiros
- Não alterar arquivos em `vendor/` sem permissão explícita
```

O Devon lê esse arquivo automaticamente ao iniciar em um diretório que o contém.

---

## Diagnósticos

```bash
# verificar configuração e conexão com provider
bun run doctor:runtime

# saída em JSON para scripts
bun run doctor:runtime:json

# persistir relatório em reports/doctor-runtime.json
bun run doctor:report
```

O `doctor:runtime` falha imediatamente se a chave de API estiver ausente ou o endpoint inacessível, antes de iniciar o agente.

---

## Modos de Permissão

Controle o que o Devon pode executar sem pedir confirmação:

```bash
devon --mode auto    # padrão: leitura livre, escrita/shell pedem confirmação
devon --mode safe    # toda ferramenta pede confirmação
devon --mode yolo    # executa tudo sem perguntar
```

---

## Modo Non-Interactive (`devon run`)

O subcomando `run` executa uma tarefa de forma não-interativa, sem abrir a TUI. Ideal para scripts, CI/CD e automações.

### Uso

```bash
# argumento direto
devon run "crie a função main.go"

# via stdin pipe
echo "refatore auth.go" | devon run

# combinando argumento + stdin
echo "adicione testes" | devon run "no arquivo read.go"

# com modo de permissão
devon run "adicione testes ao read.go" --mode yolo
```

O conteúdo do stdin é anexado à tarefa, separado por duas quebras de linha.

### Output

- **stdout**: texto da resposta do agente (para capturar ou pipear)
- **stderr**: informações de execução (tool calls, erros)

```bash
# capturar resposta
resultado=$(devon run "explique o código em auth.go")

# redirecionar tools para arquivo
devon run "refatore auth.go" 2> tools.log
```

### Exit Codes

| Código | Significado |
|---|---|
| `0` | Sucesso |
| `1` | Erro na execução (falha do agente ou LLM) |
| `2` | Erro de configuração (`.env` ausente, variáveis faltando) |
| `130` | Cancelado pelo usuário (SIGINT / Ctrl+C) |

```bash
devon run "tarefa impossível"
if [ $? -eq 2 ]; then
  echo "Configuração inválida — verifique o .env"
fi
```

### Flags

| Flag | Descrição |
|---|---|
| `--mode` | Modo de permissão: `auto` (padrão), `safe`, `yolo` |
| `--model` | Sobrescreve o modelo configurado no `.env` |
| `--env` | Caminho para o arquivo `.env` (padrão: `.env` no diretório atual) |
