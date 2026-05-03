# /test — Executar testes

**Uso:** `/test [pacote]`

**Exemplos:**
- `/test` — roda todos os testes
- `/test memory` — roda apenas `go test ./internal/memory/...`
- `/test -race` — roda com detecção de race conditions

**O que faz:**

1. Executa os testes do pacote especificado (ou `./...` se nenhum for informado)
2. Exibe o resumo: total de testes, aprovados, reprovados, tempo de execução
3. Para testes reprovados, exibe o nome do teste, arquivo, linha e mensagem de erro
4. Sugere o próximo passo para corrigir cada falha

**Comandos executados:**

```bash
# Todos
go test -count=1 ./...

# Pacote específico
go test -count=1 -v ./internal/<pacote>/...

# Com race detector
go test -race -count=1 ./...
```

**Regras dos testes no Devon:**

- Banco de dados: sempre `db.New(":memory:")` — nunca arquivo em disco
- Sem dependências externas (testify, gomock etc.) — apenas stdlib `testing`
- Pacote de teste: `package <pkg>_test` (caixa preta)
