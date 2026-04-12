# /build — Compilar o projeto

**Uso:** `/build`

**O que faz:**

1. Executa `go build ./...` para compilar todos os pacotes
2. Se houver erros de compilação, exibe o output completo e identifica o arquivo/linha problemático
3. Se bem-sucedido, executa `go vet ./...` como validação adicional
4. Reporta um resumo: erros encontrados, arquivos afetados, sugestão de correção

**Comandos executados:**

```bash
go build ./...
go vet ./...
```

**Quando usar:**

- Após criar ou modificar qualquer arquivo `.go`
- Antes de criar um pull request
- Ao adicionar métodos novos à interface `db.Store` (verificar se `fakeDB` em `cmd/devon/main.go` está atualizado)

**Nota:** Se o build quebrar após adicionar um método à interface `db.Store`,
adicioné o stub correspondente no `fakeDB` em `cmd/devon/main.go`.
