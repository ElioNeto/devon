# Contribuindo com Devon

Obrigado pelo interesse em contribuir! Devon é um agente de código CLI escrito em Go, com TUI baseada no ecossistema Charm (Bubble Tea, Lip Gloss, Glamour). Este guia cobre o setup do ambiente, convenções e fluxo de contribuição.

## Pré-requisitos

| Ferramenta | Versão mínima |
|---|---|
| [Go](https://go.dev/dl/) | 1.24.2 |
| GNU Make | qualquer |
| git | qualquer |
| [golangci-lint](https://golangci-lint.run/usage/install/) | latest |

```bash
# Instalar golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Setup Local

```bash
git clone https://github.com/ElioNeto/devon.git
cd devon
go mod download
make build
make run
```

## Comandos do Makefile

| Target | Descrição |
|---|---|
| `make build` | Compila com `CGO_ENABLED=0` e injeta a versão via `-ldflags` |
| `make run` | Compila e executa imediatamente |
| `make test` | Roda `go test ./...` |
| `make lint` | Roda `golangci-lint` |
| `make install` | Copia o binário para `~/.local/bin` |
| `make build-all` | Cross-compila para linux/darwin (amd64/arm64) |

## Testes

Todos os testes devem passar antes de abrir um PR:

```bash
make test
# ou com saída detalhada:
go test ./... -v
```

## Lint

```bash
make lint
```

Rode antes de cada commit. O CI bloqueia PRs com violações de lint.

## Convenções de Código

### Estilo

- Formate com `go fmt ./...` antes de commitar
- Siga [Effective Go](https://go.dev/doc/effective_go) e [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Nomes de pacotes: letras minúsculas, sem underscore
- Comentários de API pública são obrigatórios (godoc)

### Estrutura de Pacotes

```
.
├── cmd/                  Ponto de entrada CLI (Cobra)
└── internal/
    ├── agent/            Loop principal do agente, eventos, contexto de projeto
    ├── config/           Carregamento e validação de configuração
    ├── cost/             Rastreamento de tokens e custo de LLM
    ├── db/               Persistência SQLite (hot path + cold path)
    ├── history/          Histórico de sessões e mensagens
    ├── llm/              Tipos, interfaces e cliente LLM
    ├── orchestrator/     Planejamento, scheduling e execução de agentes
    ├── permissions/      Sistema de permissões e confirmação de tools
    ├── tools/            Ferramentas do agente (read_file, write_file, bash…)
    └── tui/              Interface TUI — painéis, renderização, input
```

Cada pacote dentro de `internal/` deve ser autocontido, com seus tipos, funções e testes no mesmo diretório.

### Mensagens de Commit

Usamos [Conventional Commits](https://www.conventionalcommits.org/):

```
<tipo>(<escopo opcional>): <descrição concisa>
```

| Tipo | Quando usar |
|---|---|
| `feat` | Nova funcionalidade |
| `fix` | Correção de bug |
| `refactor` | Mudança sem corrigir bug nem adicionar feature |
| `chore` | Manutenção, dependências, configuração |
| `docs` | Alterações na documentação |
| `test` | Adição ou mudança de testes |
| `style` | Formatação, whitespace, linter |

**Exemplos:**

```
feat(tui): implement multi-line input with history navigation
fix(llm): handle data: [DONE] in SSE stream parser
docs: rewrite CONTRIBUTING.md
chore: update go.mod dependencies
```

### Idiomas

| Contexto | Idioma |
|---|---|
| Mensagens de commit | Inglês |
| Strings visíveis ao usuário (TUI/CLI) | Português brasileiro (pt-BR) |
| Identificadores internos (tipos, funções, variáveis) | Inglês |
| System prompt do agente | Inglês |

## Fluxo de Pull Request

1. **Crie um branch** a partir de `main`:
   ```bash
   git checkout -b feat/nome-da-feature
   ```

2. **Desenvolva, teste e lint:**
   ```bash
   make test && make lint
   ```

3. **Commit** com conventional commits:
   ```bash
   git add .
   git commit -m "feat: descrição da mudança"
   ```

4. **Abra o PR** para `main`:
   ```bash
   git push -u origin feat/nome-da-feature
   ```

## Anatomia de um Bom PR

- **Título** descritivo e curto (segue o formato de commit)
- **Descrição** explicando *o quê* e *por quê* (o *como* está no diff)
- **Issue relacionada** referenciada (ex: `Closes #21`)
- **Escopo focado** — evite misturar mudanças não relacionadas
- **CI verde** — todos os testes e lint passando

## Roadmap

O roadmap completo está em [ROADMAP.md](ROADMAP.md). Issues com a label `good first issue` são bons pontos de entrada para novos contribuidores.
