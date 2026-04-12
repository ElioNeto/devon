---
description: Como escrever testes no projeto Devon
paths: "**/*_test.go"
---

# Testes — Devon

## Regras fundamentais

- Pacote de teste: sempre `package <pkg>_test` (teste de caixa preta)
- Sem dependências externas — apenas stdlib `testing`
- `t.Fatalf` para erros que impedem o teste de continuar
- `t.Errorf` para falhas de assertion (o teste continua)
- Todo teste deve ser determinístico e independente de ordem

## Testes com banco de dados

```go
func TestAlgo(t *testing.T) {
    // SEMPRE banco em memória — nunca arquivo em disco
    store, err := db.New(":memory:")
    if err != nil {
        t.Fatalf("criar store: %v", err)
    }
    t.Cleanup(func() { store.Close() })

    ctx := context.Background()
    // ... resto do teste
}
```

## Padrão de assertion sem testify

```go
// Verificar len
if len(results) != 2 {
    t.Fatalf("expected 2 results, got %d", len(results))
}

// Verificar valor
if got.Content != want {
    t.Errorf("Content: got %q, want %q", got.Content, want)
}

// Verificar erro esperado
if err == nil {
    t.Fatal("expected error, got nil")
}

// Verificar substring
if !strings.Contains(result, expectedSub) {
    t.Errorf("expected result to contain %q, got: %s", expectedSub, result)
}
```

## Nomes de testes

```go
// Formato: Test<Tipo><Cenário>
func TestManagerRememberAndRecall(t *testing.T) {}
func TestManagerClearEmptiesStore(t *testing.T) {}
func TestContextForReturnsRelevantFacts(t *testing.T) {}
```

## Subtestes para múltiplos cenários

```go
func TestParseMode(t *testing.T) {
    cases := []struct{
        input string
        want  config.Mode
    }{
        {"auto",  config.ModeAuto},
        {"safe",  config.ModeSafe},
        {"yolo",  config.ModeYolo},
        {"",      config.ModeAuto}, // default
    }
    for _, tc := range cases {
        t.Run(tc.input, func(t *testing.T) {
            if got := config.ParseMode(tc.input); got != tc.want {
                t.Errorf("ParseMode(%q) = %v, want %v", tc.input, got, tc.want)
            }
        })
    }
}
```

## Executar testes

```bash
go test ./...                    # todos
go test ./internal/memory/...    # pacote específico
go test -run TestManager ./...   # por nome
go test -v -count=1 ./...        # verbose, sem cache
go test -race ./...              # detectar data races
```

## O que não fazer

- Sem `time.Sleep` em testes — use canais ou mocks
- Sem arquivos temporários em `/tmp` sem `t.Cleanup` para remover
- Sem chamadas reais à internet — use interfaces e mocks
- Sem testes que dependem de ordem de execução
