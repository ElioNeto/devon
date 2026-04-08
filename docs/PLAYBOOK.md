# Playbook do Devon

Guia prático para executar, configurar e obter bons resultados com o Devon Go.

---

## 1. O Que Você Tem

- Binário único estático (`devon`) sem dependências externas
- Loop de agente com TUI (Bubble Tea) — lê/escreve arquivos, executa comandos, conecta a qualquer LLM
- Suporte a qualquer provider compatível com OpenAI: OpenRouter, Gemini, Groq, Ollama, DeepSeek
- Modos de permissão `auto`, `safe` e `yolo`

---

## 2. Instalação

```bash
# via install.sh
curl -fsSL https://raw.githubusercontent.com/ElioNeto/devon/main/install.sh | bash

# ou compile do fonte
git clone https://github.com/ElioNeto/devon.git
cd devon
make build
```

---

## 3. Inicialização Diária

```bash
# inicia o Devon no diretório atual
devon

# com modo de permissão explícito
devon --mode safe    # toda ferramenta pede confirmação
devon --mode yolo    # executa tudo sem perguntar
```

---

## 4. Configuração de Provider

Crie um arquivo `.env` na raiz do projeto:

```bash
DEVON_API_KEY=sk-or-sua-chave-aqui
DEVON_BASE_URL=https://openrouter.ai/api/v1
DEVON_MODEL=mistralai/devstral-2512:free
```

### Providers suportados

| Provider | DEVON_BASE_URL | Modelos recomendados |
|---|---|---|
| OpenRouter | `https://openrouter.ai/api/v1` | `mistralai/devstral-2512:free`, `qwen/qwen3-coder:free` |
| Google Gemini | `https://generativelanguage.googleapis.com/v1beta/openai` | `gemini-2.5-flash` |
| Groq | `https://api.groq.com/openai/v1` | `llama-3.3-70b-versatile` |
| Ollama (local) | `http://localhost:11434/v1` | `qwen2.5-coder:32b`, `llama3.3:70b` |
| OpenAI | `https://api.openai.com/v1` | `gpt-4o` |
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |

---

## 5. Contexto de Projeto (DEVON.md)

Crie um `DEVON.md` na raiz do projeto para injetar contexto ao agente:

```markdown
# meu-projeto

API REST em Go para gestão de pedidos.

## Comandos
- Build: `make build`
- Testes: `go test ./...`
- Lint: `golangci-lint run ./...`

## Convenções
- Erros com `fmt.Errorf("...: %w", err)`
- Não usar `panic` em código de produção
```

O Devon lê este arquivo automaticamente ao iniciar.

---

## 6. Diagnósticos

```bash
# verifica configuração e conectividade
devon doctor

# build limpo
make build

# rodar testes
go test ./...
```

---

## 7. Ferramentas do Agente

O Devon executa um loop `prompt → LLM → tool call → resultado → LLM` com:

- **Filesystem:** `read_file`, `write_file`, `edit_file`, `list_dir`, `glob`, `grep`
- **Shell:** `bash` com timeout, captura de stdout/stderr e controle de permissão
- **Contexto:** `DEVON.md` injetado como system prompt adicional

---

## 8. Playbook Prático de Prompts

### Entendimento de código
- "Mapeie a arquitetura deste repositório e explique o fluxo de execução."
- "Encontre os 5 módulos mais arriscados e explique o porquê."

### Refatoração
- "Refatore este módulo para maior clareza sem alterar o comportamento, depois execute os testes e resuma o diff."
- "Extraia lógica duplicada e adicione testes mínimos."

### Depuração
- "Reproduza a falha, identifique a causa raiz, implemente a correção e valide com `go test`."
- "Trace este caminho de erro e liste os pontos de falha prováveis com níveis de confiança."

### Revisão de código
- "Faça uma revisão das alterações não preparadas, priorize bugs/regressões e sugira patches concretos."

---

## 9. Regras de Trabalho Seguro

- Prefira `--mode safe` em projetos com código crítico
- Use `DEVON.md` para fixar convenções — o agente vai respeitá-las
- Revise tool calls destrutivas antes de confirmar no modo `auto`
- Mantenha `.env` local (já está no `.gitignore`)

---

## 10. Referência de Comandos

```bash
# iniciar
devon
devon --mode safe
devon --mode yolo

# diagnósticos
devon doctor

# build e testes
make build
go test ./...
golangci-lint run ./...
```
