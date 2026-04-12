---
description: Padrões de estilo e formatação para código Go no projeto Devon
paths: "**/*.go"
---

# Padrões de código Go — Devon

## Formatação e lint

- Todo código Go deve passar em `gofmt` e `goimports` antes do commit
- Use `golangci-lint run` para checagem completa (configurado em `.golangci.yml` se existir)
- `go vet ./...` deve retornar zero warnings

## Tratamento de erros

```go
// CERTO — erro com contexto e %w para unwrap
if err != nil {
    return fmt.Errorf("operação X: %w", err)
}

// ERRADO — erro sem contexto
if err != nil {
    return err
}

// ERRADO — panic em código de produção
if err != nil {
    panic(err)
}
```

## Escrita de arquivos — obrigatoriamente atômica

```go
// CERTO — temp file + rename
tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
if err != nil { return fmt.Errorf("criar temp: %w", err) }
defer os.Remove(tmp.Name())
if _, err := tmp.Write(data); err != nil { return fmt.Errorf("escrever temp: %w", err) }
if err := tmp.Close(); err != nil { return fmt.Errorf("fechar temp: %w", err) }
if err := os.Rename(tmp.Name(), dst); err != nil { return fmt.Errorf("renomear: %w", err) }

// ERRADO — escrita direta sem atomicidade
os.WriteFile(dst, data, 0644)
```

## Dependências

- **Zero CGO**: use `modernc.org/sqlite` para SQLite (nunca `mattn/go-sqlite3`)
- Não adicione dependências externas sem discutir primeiro
- Testes: apenas stdlib `testing` — sem testify, gomock ou frameworks externos

## Interfaces

- Defina interfaces no pacote **consumidor**, não no implementador
- Interfaces pequenas e focadas (1-5 métodos)
- Nomes de interfaces terminam em `-er` quando faz sentido: `Streamer`, `Checker`

## Estrutura de pacotes

- Sem `init()` — use construtores explícitos (`New(...)`) 
- Sem variáveis globais mutáveis
- Pacotes em `internal/` não devem importar `cmd/`
- Exportar apenas o mínimo necessário

## Concorrência

- Prefira canais a mutexes para comunicação
- `sync.Mutex` apenas para proteção de estado compartilhado
- Sempre passe `context.Context` como primeiro parâmetro em funções que fazem I/O
- Respeite cancelamento: verifique `ctx.Err()` nos loops

## Nomes

- Variáveis locais: `camelCase` curto e descritivo (`cfg`, `err`, `ch`)
- Tipos exportados: `PascalCase`
- Constantes: `PascalCase` para exportadas, `camelCase` para internas
- Evite abreviações obscuras — `srv` ok, `s` para server não
