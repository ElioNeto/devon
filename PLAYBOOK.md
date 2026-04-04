# Playbook do Agente Local OpenClaude

Este playbook é um guia prático para executar o OpenClaude com um modelo local (Ollama), trabalhar com segurança e obter bons resultados no dia a dia.

## 1. O Que Você Tem

- Um loop de agente CLI que pode ler/escrever arquivos, executar comandos no terminal e ajudar com fluxos de trabalho de programação.
- Um sistema de perfis de provider local (`profile:init` e `dev:profile`).
- Verificações de runtime (`doctor:runtime`) e relatórios (`doctor:report`).
- Um perfil de modelo local atualmente configurado para `llama3.1:8b`.

## 2. Inicialização Diária (Caminho Rápido)

Execute isso na raiz do seu projeto:

```powershell
bun run dev:profile
```

Para trocas rápidas:

```powershell
# preset de baixa latência
bun run dev:fast

# preset de melhor qualidade de código
bun run dev:code
```

Se tudo estiver saudável, o OpenClaude inicia diretamente.

## 3. Configuração Única (Se Necessário)

### 3.1 Inicializar um perfil local

```powershell
bun run profile:init -- --provider ollama --model llama3.1:8b
```

Ou deixe o OpenClaude recomendar o melhor modelo local para seu objetivo:

```powershell
bun run profile:init -- --provider ollama --goal coding
```

Visualize as recomendações antes de salvar:

```powershell
bun run profile:recommend -- --goal coding --benchmark
```

### 3.2 Confirmar arquivo de perfil

```powershell
Get-Content .\.openclaude-profile.json
```

### 3.3 Validar ambiente

```powershell
bun run doctor:runtime
```

## 4. Saúde e Diagnósticos

### 4.1 Verificações legíveis por humanos

```powershell
bun run doctor:runtime
```

### 4.2 Diagnósticos em JSON (automação/logging)

```powershell
bun run doctor:runtime:json
```

### 4.3 Persistir relatório de runtime

```powershell
bun run doctor:report
```

Saída do relatório:

- `reports/doctor-runtime.json`

### 4.4 Verificações de hardening

```powershell
# verificações práticas (smoke + runtime doctor)
bun run hardening:check

# verificações estritas (inclui typecheck)
bun run hardening:strict
```

## 5. Modos de Provider

## 5.1 Modo local (Ollama)

```powershell
bun run profile:init -- --provider ollama --model llama3.1:8b
bun run dev:profile
```

Comportamento esperado:

- Nenhuma chave de API necessária.
- `OPENAI_BASE_URL` deve ser `http://localhost:11434/v1`.

## 5.2 Modo OpenAI

```powershell
bun run profile:init -- --provider openai --api-key sk-... --model gpt-4o
bun run dev:profile
```

Comportamento esperado:

- Chave de API real necessária.
- Valores de placeholder falham rapidamente.

## 6. Matriz de Solução de Problemas

## 6.1 `Script not found "dev"`

Causa:

- Você executou o comando na pasta errada.

Solução:

```powershell
cd C:\Users\SeuUsuario\Documents\openclaude\openclaude
bun run dev:profile
```

## 6.2 `ollama: term not recognized`

Causa:

- Ollama não está instalado ou o PATH não foi carregado neste terminal.

Solução:

- Instale o Ollama em https://ollama.com/download/windows ou `winget install Ollama.Ollama`.
- Abra um novo terminal e execute:

```powershell
ollama --version
```

## 6.3 `Provider reachability failed` para localhost

Causa:

- Serviço Ollama não está em execução.

Solução:

```powershell
ollama serve
```

Em seguida, em outro terminal:

```powershell
bun run doctor:runtime
```

## 6.4 `Missing key for non-local provider URL`

Causa:

- `OPENAI_BASE_URL` aponta para endpoint remoto sem chave.

Solução:

- Reinicialize o perfil para ollama:

```powershell
bun run profile:init -- --provider ollama --model llama3.1:8b
```

Ou escolha um perfil Ollama local automaticamente por objetivo:

```powershell
bun run profile:init -- --provider ollama --goal balanced
```

## 6.5 Erro de chave placeholder (`SUA_CHAVE`)

Causa:

- Um placeholder foi usado em vez de uma chave real.

Solução:

- Para OpenAI: use uma chave real.
- Para Ollama: nenhuma chave necessária; mantenha a URL base do localhost.

## 7. Modelos Locais Recomendados

- Rápido/geral: `llama3.1:8b`
- Melhor qualidade de código (se o hardware suportar): `qwen2.5-coder:14b`
- Fallback para hardware limitado: modelo instruct menor

Troque de modelo rapidamente:

```powershell
bun run profile:init -- --provider ollama --model qwen2.5-coder:14b
bun run dev:profile
```

Atalhos de preset já configurados:

```powershell
bun run profile:fast   # llama3.2:3b
bun run profile:code   # qwen2.5-coder:7b
```

Seleção automática local baseada em objetivo:

```powershell
bun run profile:init -- --provider ollama --goal latency
bun run profile:init -- --provider ollama --goal balanced
bun run profile:init -- --provider ollama --goal coding
```

`profile:auto` é um seletor do melhor provider disponível, não um comando apenas local. Use `--provider ollama` quando quiser ficar em um modelo local.

## 8. Playbook Prático de Prompts (Copiar/Colar)

## 8.1 Entendimento de código

- "Mapeie a arquitetura deste repositório e explique o fluxo de execução do ponto de entrada até a invocação de ferramentas."
- "Encontre os 5 módulos mais arriscados e explique o porquê."

## 8.2 Refatoração

- "Refatore este módulo para maior clareza sem alterar o comportamento, depois execute as verificações e resuma o impacto do diff."
- "Extraia a lógica compartilhada de funções duplicadas e adicione testes mínimos."

## 8.3 Depuração

- "Reproduza a falha, identifique a causa raiz, implemente a correção e valide com comandos."
- "Trace este caminho de erro e liste os pontos de falha prováveis com níveis de confiança."

## 8.4 Confiabilidade

- "Adicione guardrails de runtime e mensagens de falha rápida para variáveis de ambiente de provider inválidas."
- "Crie um comando de diagnóstico que produza um relatório JSON para artefatos de CI."

## 8.5 Modo de revisão

- "Faça uma revisão de código das alterações não preparadas, priorize bugs/regressões e sugira patches concretos."

## 9. Regras de Trabalho Seguro

- Execute `doctor:runtime` antes de depurar problemas de provider.
- Prefira `dev:profile` a edições manuais de variáveis de ambiente.
- Mantenha `.openclaude-profile.json` local (já está no gitignore).
- Use `doctor:report` antes de pedir ajuda para ter um snapshot reproduzível.

## 10. Checklist de Recuperação Rápida

Quando algo quebrar, execute na ordem:

```powershell
bun run doctor:runtime
bun run doctor:report
bun run smoke
```

Se as respostas estiverem muito lentas, verifique o modo do processador:

```powershell
ollama ps
```

Se `PROCESSOR` mostrar `CPU`, sua configuração está válida, mas a latência será maior para modelos grandes.

Se o modo de modelo local estiver falhando:

```powershell
ollama --version
ollama serve
bun run doctor:runtime
bun run dev:profile
```

## 11. Referência de Comandos

```powershell
# perfil
bun run profile:init -- --provider ollama --model llama3.1:8b
bun run profile:init -- --provider openai --api-key sk-... --model gpt-4o

# iniciar
bun run dev:profile
bun run dev:ollama
bun run dev:openai

# diagnósticos
bun run doctor:runtime
bun run doctor:runtime:json
bun run doctor:report

# qualidade
bun run smoke
bun run hardening:check
bun run hardening:strict
```

## 12. Critérios de Sucesso

Sua configuração está saudável quando:

- `bun run doctor:runtime` passa nas verificações de provider e acessibilidade.
- `bun run dev:profile` abre o CLI normalmente.
- O modelo mostrado na interface corresponde ao modelo do perfil selecionado.
