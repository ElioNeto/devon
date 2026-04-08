# Configuração Avançada do Devon

Este guia cobre configuração de providers, perfis multi-provider, diagnósticos e opções de runtime.

---

## 1. Configuração básica (variáveis de ambiente)

O modo mais simples de configurar o Devon continua sendo via variáveis de ambiente. **Zero breaking change** — `DEVON_API_KEY`, `DEVON_BASE_URL` e `DEVON_MODEL` continuam funcionando exatamente como antes.

Crie um `.env` na raiz do projeto:

```bash
DEVON_API_KEY=sk-or-sua-chave-aqui
DEVON_BASE_URL=https://openrouter.ai/api/v1
DEVON_MODEL=mistralai/devstral-2512:free
```

Inicie o Devon:

```bash
devon
```

O Devon carrega automaticamente o `.env` do diretório atual.

| Variável | Obrigatória | Descrição |
|---|---|---|
| `DEVON_API_KEY` | Sim* | Chave de API (`*` não necessária para modelos locais) |
| `DEVON_BASE_URL` | Sim | Endpoint da API (ex: `https://openrouter.ai/api/v1`) |
| `DEVON_MODEL` | Sim | Nome do modelo (ex: `mistralai/devstral-2512:free`) |

---

## 2. devon.toml

Para cenários com múltiplos providers ou ambientes, o Devon suporta um arquivo `devon.toml` com perfis nomeados.

### Onde colocar o arquivo

O Devon procura o `devon.toml` na seguinte ordem:

1. **Diretório atual** — `./devon.toml`
2. **Home do usuário** — `~/.devon.toml`

Se nenhum for encontrado, o Devon usa as variáveis de ambiente normalmente.

### Campos disponíveis

```toml
[defaults]
profile = "fast"   # string — perfil usado quando --profile não é especificado
mode    = "auto"   # string — auto | safe | yolo

[[profiles]]
name        = "fast"                              # string — nome único do perfil
provider    = "openai"                            # string — identificador do provider
api_key_env = "OPENROUTER_KEY"                    # string — nome da env var com a API key
base_url    = "https://openrouter.ai/api/v1"      # string — endpoint da API
model       = "mistralai/devstral-2512:free"      # string — modelo a usar
fallback    = ["local"]                           # []string — perfis de fallback (opcional)
```

### Como o Devon decide qual perfil usar

A resolução segue esta prioridade (maior para menor):

1. **Flag `--profile`** — `devon --profile power`
2. **`defaults.profile`** no `devon.toml`
3. **Variáveis de ambiente** — `DEVON_API_KEY` / `DEVON_BASE_URL` / `DEVON_MODEL`

Se `--profile` for passado, o perfil correspondente é carregado do `devon.toml`. Se não houver `--profile` nem `devon.toml`, as variáveis de ambiente são usadas diretamente.

---

## 3. Gerenciando perfis

```bash
# Usar perfil "power"
devon --profile power

# Usar Ollama local
devon --profile local
# Alias curto com -p
devon -p local

# Sobrescrever o modelo do perfil ativo
devon --model qwen3:latest

# Combinar perfil + override de modelo
devon --profile fast --model gpt-4o

# Listar perfis e status das keys
devon profiles list

# Testar conectividade de cada perfil
devon profiles test
```

### `devon profiles list`

Exibe todos os perfis configurados no `devon.toml` com o status de cada API key:

```
Perfis configurados (devon.toml):

  ● fast    openai      mistralai/devstral-2512:free      key: ✔
  ● local   ollama      qwen2.5-coder:32b                 key: —
  ● power   anthropic   claude-sonnet-4-5                 key: ✔

Padrão: fast
```

- `key: ✔` — a variável de ambiente referenciada em `api_key_env` está definida
- `key: —` — `api_key_env` vazio (provider local) ou variável não definida

### `devon profiles test`

Testa a conectividade com cada provider configurado:

```
Testando perfis...

  fast    → https://openrouter.ai/api/v1  [PASS] HTTP 200
  local   → http://localhost:11434/v1     [FAIL] connection refused
  power   → https://api.anthropic.com/v1 [PASS] HTTP 200

Resultado: 2/3 perfis acessíveis.
```

---

## 4. Fallback automático

Quando um perfil define o campo `fallback`, o Devon tenta automaticamente o próximo perfil da lista em caso de:

- **HTTP 429** — rate limit atingido
- **HTTP 5xx** — erro do servidor

Exemplo de configuração:

```toml
[[profiles]]
name     = "fast"
# ...
fallback = ["local"]   # se "fast" der 429 ou 5xx, tenta "local"

[[profiles]]
name     = "power"
# ...
fallback = ["fast"]    # se "power" falhar, tenta "fast"
```

O fallback é tentado uma vez por perfil da lista. Se todos falharem, o erro original é retornado ao usuário.

---

## 5. Exemplos práticos

### OpenRouter (modelos gratuitos + pagos)

Crie sua chave em [openrouter.ai/keys](https://openrouter.ai/keys).

```toml
[[profiles]]
name        = "openrouter"
provider    = "openai"
api_key_env = "OPENROUTER_KEY"
base_url    = "https://openrouter.ai/api/v1"
model       = "mistralai/devstral-2512:free"
```

```bash
export OPENROUTER_KEY=sk-or-sua-chave-aqui
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

### Anthropic (direto)

```toml
[[profiles]]
name        = "anthropic"
provider    = "anthropic"
api_key_env = "ANTHROPIC_KEY"
base_url    = "https://api.anthropic.com/v1"
model       = "claude-sonnet-4-5"
```

```bash
export ANTHROPIC_KEY=sk-ant-sua-chave-aqui
devon --profile anthropic
```

---

## 6. Segurança

- **Nunca coloque chaves de API diretamente no `devon.toml`**. Use sempre o campo `api_key_env` apontando para o nome de uma variável de ambiente.
- O `devon.toml` **pode ser commitado no repositório** com segurança, pois contém apenas nomes de variáveis de ambiente — nunca valores.
- Chaves devem ser exportadas via shell (`export OPENROUTER_KEY=...`) ou definidas em um `.env` local (que está no `.gitignore`).
- O Devon não armazena, envia ou faz proxy de suas chaves para nenhum serviço além do provider configurado. Todo tráfego vai direto do seu terminal para o endpoint da API.

---

## Modos de Permissão

Controle o que o Devon pode executar sem pedir confirmação:

```bash
devon --mode auto    # padrão: leitura livre, escrita/shell pedem confirmação
devon --mode safe    # toda ferramenta pede confirmação
devon --mode yolo    # executa tudo sem perguntar
```

---

## Diagnósticos

```bash
# verificar configuração e conexão com provider
devon doctor
```

O `doctor` valida a configuração e testa a conexão com o provider antes de iniciar o agente.

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

# com modo de permissão e perfil
devon run "adicione testes ao read.go" --mode yolo --profile fast
```

### Exit Codes

| Código | Significado |
|---|---|
| `0` | Sucesso |
| `1` | Erro na execução (falha do agente ou LLM) |
| `2` | Erro de configuração (`.env` ausente, variáveis faltando) |
| `130` | Cancelado pelo usuário (SIGINT / Ctrl+C) |

### Flags

| Flag | Descrição |
|---|---|
| `--profile`, `-p` | Perfil de provider definido em `devon.toml` |
| `--model` | Sobrescreve o modelo do perfil ativo |
| `--mode` | Modo de permissão: `auto` (padrão), `safe`, `yolo` |
| `--env` | Caminho para o arquivo `.env` (padrão: `.env` no diretório atual) |
