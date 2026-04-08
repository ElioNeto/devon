# Configuração Avançada

Este guia cobre configuração de providers, perfis multi-provider, fallback automático, diagnósticos e modo não-interativo.

---

## Configuração Básica (Variáveis de Ambiente)

O modo mais simples de configurar o Devon é via variáveis de ambiente. Crie um `.env` na raiz do projeto:

```bash
DEVON_API_KEY=sk-or-sua-chave-aqui
DEVON_BASE_URL=https://openrouter.ai/api/v1
DEVON_MODEL=mistralai/devstral-2512:free
```

| Variável | Obrigatória | Descrição |
|---|---|---|
| `DEVON_API_KEY` | Sim* | Chave de API (`*` não necessária para modelos locais) |
| `DEVON_BASE_URL` | Sim | Endpoint da API |
| `DEVON_MODEL` | Sim | Nome do modelo |

O Devon carrega automaticamente o `.env` do diretório atual ao iniciar.

---

## Arquivo `devon.toml`

Para cenários com múltiplos providers ou ambientes, o Devon suporta um arquivo `devon.toml` com perfis nomeados.

### Localização

O Devon procura o `devon.toml` na seguinte ordem:

1. Diretório atual — `./devon.toml`
2. Home do usuário — `~/.devon.toml`

Se nenhum for encontrado, o Devon usa as variáveis de ambiente.

### Estrutura

```toml
[defaults]
profile = "fast"   # perfil padrão quando --profile não é especificado
mode    = "auto"   # auto | safe | yolo

[[profiles]]
name        = "fast"
provider    = "openai"
api_key_env = "OPENROUTER_KEY"               # nome da env var — nunca o valor
base_url    = "https://openrouter.ai/api/v1"
model       = "mistralai/devstral-2512:free"
fallback    = ["local"]                      # perfis de fallback (opcional)

[[profiles]]
name     = "local"
provider = "ollama"
base_url = "http://localhost:11434/v1"
model    = "qwen2.5-coder:32b"

[[profiles]]
name        = "power"
provider    = "anthropic"
api_key_env = "ANTHROPIC_KEY"
base_url    = "https://api.anthropic.com/v1"
model       = "claude-sonnet-4-5"
```

### Resolução de Perfil

A resolução segue esta prioridade (maior para menor):

1. Flag `--profile` na linha de comando
2. Campo `defaults.profile` no `devon.toml`
3. Variáveis de ambiente (`DEVON_API_KEY` / `DEVON_BASE_URL` / `DEVON_MODEL`)

---

## Gerenciando Perfis

```bash
# Usar um perfil específico
devon --profile power
devon -p local

# Sobrescrever o modelo do perfil ativo
devon --model gpt-4o

# Combinar perfil + override de modelo
devon --profile fast --model gpt-4o

# Listar perfis e status das API keys
devon profiles list

# Testar conectividade de cada perfil
devon profiles test
```

### `devon profiles list`

```
Perfis configurados (devon.toml):

  ● fast    openai      mistralai/devstral-2512:free      key: ✔
  ● local   ollama      qwen2.5-coder:32b                 key: —
  ● power   anthropic   claude-sonnet-4-5                 key: ✔

Padrão: fast
```

### `devon profiles test`

```
Testando perfis...

  fast    → https://openrouter.ai/api/v1  [PASS] HTTP 200
  local   → http://localhost:11434/v1     [FAIL] connection refused
  power   → https://api.anthropic.com/v1 [PASS] HTTP 200

Resultado: 2/3 perfis acessíveis.
```

---

## Fallback Automático

Quando um perfil define o campo `fallback`, o Devon tenta automaticamente o próximo perfil em caso de erro HTTP 429 (rate limit) ou 5xx (erro do servidor):

```toml
[[profiles]]
name     = "fast"
fallback = ["local"]  # se "fast" der 429 ou 5xx, tenta "local"
```

O fallback é tentado uma vez por perfil da lista. Se todos falharem, o erro original é retornado.

---

## Exemplos por Provider

### OpenRouter

```toml
[[profiles]]
name        = "openrouter"
api_key_env = "OPENROUTER_KEY"
base_url    = "https://openrouter.ai/api/v1"
model       = "mistralai/devstral-2512:free"
```

```bash
export OPENROUTER_KEY=sk-or-sua-chave
devon --profile openrouter
```

### Ollama (local, sem chave)

```bash
ollama pull qwen2.5-coder:32b
```

```toml
[[profiles]]
name     = "local"
provider = "ollama"
base_url = "http://localhost:11434/v1"
model    = "qwen2.5-coder:32b"
```

```bash
devon --profile local
```

### Anthropic

```toml
[[profiles]]
name        = "anthropic"
api_key_env = "ANTHROPIC_KEY"
base_url    = "https://api.anthropic.com/v1"
model       = "claude-sonnet-4-5"
```

```bash
export ANTHROPIC_KEY=sk-ant-sua-chave
devon --profile anthropic
```

---

## Segurança

> ⚠️ **Nunca coloque chaves de API diretamente no `devon.toml`.** Use sempre `api_key_env` apontando para o nome de uma variável de ambiente.

- O `devon.toml` **pode ser commitado com segurança** — contém apenas nomes de variáveis, nunca valores.
- Defina as chaves via shell (`export KEY=...`) ou em um `.env` local (já no `.gitignore`).
- O Devon não armazena nem faz proxy das suas chaves — todo tráfego vai direto do seu terminal para o endpoint configurado.

---

## Modos de Permissão

| Modo | Comportamento |
|---|---|
| `auto` *(padrão)* | Leitura livre; escrita e shell pedem confirmação |
| `safe` | Toda ferramenta pede confirmação |
| `yolo` | Execução autônoma sem interrupções |

```bash
devon --mode auto
devon --mode safe
devon --mode yolo
```

---

## Diagnósticos

```bash
devon doctor
```

Valida a configuração e testa a conexão com o provider antes de iniciar o agente.

---

## Modo Não-Interativo (`devon run`)

O subcomando `run` executa uma tarefa sem abrir a TUI. Ideal para scripts, hooks de git e CI/CD.

```bash
# Argumento direto
devon run "crie a função main.go"

# Via stdin
echo "refatore auth.go" | devon run

# Com perfil e modo
devon run "adicione testes" --mode yolo --profile fast
```

### Exit Codes

| Código | Significado |
|---|---|
| `0` | Sucesso |
| `1` | Erro na execução (falha do agente ou LLM) |
| `2` | Erro de configuração (variáveis ausentes, `.env` inválido) |
| `130` | Cancelado pelo usuário (SIGINT / Ctrl+C) |

### Flags do `devon run`

| Flag | Descrição |
|---|---|
| `--profile`, `-p` | Perfil definido em `devon.toml` |
| `--model` | Sobrescreve o modelo do perfil ativo |
| `--mode` | Modo de permissão: `auto` (padrão), `safe`, `yolo` |
| `--env` | Caminho para o arquivo `.env` (padrão: `.env` no diretório atual) |
