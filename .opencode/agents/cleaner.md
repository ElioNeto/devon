---
description: Limpa .task-state.json se todas as tasks estão completed, faz commit e chama /issue-done e /next-issue
mode: subagent
temperature: 0.0
maxSteps: 100
permission:
  read: allow
  list: allow
  glob: allow
  grep: allow
  edit: allow
  bash: allow
  task: allow
---

## Condição obrigatória

**Só execute os passos abaixo se todas as tasks (TODOs) no arquivo `.task-state.json` estiverem com status `completed`.**

## Passos

1. Ler `.task-state.json` e verificar o campo `tasks`. Se algum item tiver `status != "completed"`, abortar com `STATUS: REJECTED` e a lista de tasks pendentes.
2. **Limpar o conteúdo do `.task-state.json`** – substituir por `{}` ou `{"tasks": []}`.
3. **Commit das mudanças**: 
   - `git add .task-state.json`
   - `git commit -m "chore: finalizar issue #<numero> - limpar task state"`
4. **Chamar `/issue-done <numero>`**.
5. **Chamar `/next-issue`** e capturar o número da próxima issue.

## Saída

```
Issue <numero> completa. Proxima issue: <numero>
```

Onde:
- `<numero>` é o número da issue que acabou de ser finalizada.
- `<numero>` após "Proxima issue:" é o número da próxima issue retornado por `/next-issue`.

## Saída em caso de pendência

```
STATUS: REJECTED
PENDING_TASKS:
- <id ou descrição da task pendente>
```

## Observações

- O número da issue atual deve ser obtido do contexto (branch name, variável de ambiente, ou argumento).
- Se `/issue-done` ou `/next-issue` falharem, reportar erro e não prosseguir.

