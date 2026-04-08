# Contribuindo com Devon

Devon é um assistente de código CLI escrito em Go, com uma interface TUI baseada no ecossistema Charm (Bubble Tea, Lip Gloss e Glamour). Este guia descreve como configurar o ambiente de desenvolvimento, as convenções adotadas e o fluxo de contribuição.

## Pré-requisitos

- **Go** 1.24.2 ou superior
- **make** (GNU Make)
- **git**
- **golangci-lint** (para rodar o lint localmente: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)

## Setup Local

```bash
# Clonar o repositório
git clone https://github.com/ElioNeto/devon.git
cd devon

# Baixar dependências
go mod download

# Compilar
make build

# Rodar (binário gerado em ./bin/devon)
make run
```

### Comandos do Makefile

| Target | Descrição |
|---|---|
| `make build` | Compila com `CGO_ENABLED=0`, vincula a versão do git via `-ldflags` |
| `make run` | Compila e executa imediatamente |
| `make test` | Roda os testes com `go test ./...` |
| `make lint` | Roda `golangci-lint` |
| `make install` | Copia o binário para `~/.local/bin` |
| `make build-all` | Cross-compila para linux/darwin (amd64/arm64) |

## Testes

```bash
make test
```

Todos os testes devem passar antes de abrir um PR. Use `go test ./... -v` para saída detalhada durante o desenvolvimento.

## Lint

```bash
make lint
```

O projeto usa `golangci-lint` como linter principal. Rode antes de commitar.

## Convenções de Código

### Estilo

- Formate o código com `gofmt` ou `go fmt ./...`
- Siga as convenções idiomáticas do Go (Effective Go, Go Code Review Comments)
- Nomeie pacotes com letras minúsculas e sem underscore
- Comentários de API pública são obrigatórios (godoc)

### Organização de Pacotes

O projeto usa o layout padrão do Go com código de aplicação no diretório `internal/`:

```
.
├── cmd/              Binário e ponto de entrada CLI (Cobra)
├── internal/
│   ├── agent/        Orquestração do agente — loop principal, eventos, contexto do projeto
│   ├── config/       Carregamento e gerenciamento de configuração
│   ├── context/      Gerenciamento de contexto e memória
│   ├── cost/         Rastreamento de custos de LLM (tokens, preço)
│   ├── history/      Persistência de sessões e mensagens
│   ├── llm/          Tipos e interfaces para modelos de linguagem
│   ├── permissions/  Sistema de permissões e confirmação de tools
│   ├── tools/        Implementações das ferramentas (read_file, write_file, etc.)
│   └── tui/          Interface TUI — Bubble Tea, painéis, renderização
```

Cada pacote dentro de `internal/` deve ser autocontido, com seus tipos, funções e testes no mesmo diretório.

### Mensagens do Commit

Usamos **conventional commits**. O formato é:

```
<tipo>: <descrição concisa>
```

Tipos comuns:

| Tipo | Quando usar |
|---|---|
| `feat` | Nova funcionalidade |
| `fix` | Correção de bug |
| `refactor` | Mudança que não corrige bug nem adiciona feature |
| `chore` | Manutenção, dependências, configuração |
| `docs` | Alterações na documentação |
| `test` | Adição ou mudança de testes |
| `style` | Mudanças de formatação (whitespace, linter) |

Exemplos:

```
feat: implement input history navigation for TUI chat messages
refactor: change Message.Content to a pointer to support optional JSON fields
docs: add CONTRIBUTING.md with setup and conventions
chore: update go.mod dependencies
```

### Idiomas

- Mensagens de commit: **inglês**
- Strings visíveis ao usuário na TUI: **português brasileiro (pt-BR)**
- Identificadores internos (tipos, funções, variáveis): **inglês**

## Fluxo de Pull Request

1. Crie um branch a partir de `main` (ou continue o branch da feature existente):
   ```bash
   git checkout -b feature/nome-da-feature
   ```

2. Faça suas alterações, rode os testes e o lint:
   ```bash
   make test && make lint
   ```

3. Commit com mensagens no formato conventional commit:
   ```bash
   git add .
   git commit -m "feat: descrição da mudança"
   ```

4. Push e abra o PR:
   ```bash
   git push -u origin feature/nome-da-feature
   ```

5. O PR será revisado antes do merge em `main`.

## Estrutura de um PR

Um bom PR deve:

- Ter um título descritivo e curto
- Incluir uma descrição explicando **o quê** e **por quê** (o **como** está no diff)
- Referenciar issues relacionadas (ex: `Closes #21`)
- Ser focado — evite misturar mudanças não relacionadas
- Garantir que todos os testes e o lint passam

## Roadmap

O roadmap do projeto está documentado no arquivo [ROADMAP.md](ROADMAP.md), com issues priorizadas e em andamento.