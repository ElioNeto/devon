# OpenClaude

Use o Claude Code com **qualquer LLM** — não apenas o Claude.

O OpenClaude é um fork do [vazamento do código-fonte do Claude Code](https://gitlawb.com/node/repos/z6MkgKkb/instructkr-claude-code) (exposto via source maps do npm em 31 de março de 2026). Adicionamos um shim de provider compatível com OpenAI para que você possa usar GPT-4o, DeepSeek, Gemini, Llama, Mistral ou qualquer modelo que fale a API de chat completions da OpenAI. Agora também suporta o backend ChatGPT Codex para `codexplan` e `codexspark`, e inferência local via [Atomic Chat](https://atomic.chat/) em Apple Silicon.

Todas as ferramentas do Claude Code funcionam — bash, leitura/escrita/edição de arquivos, grep, glob, agentes, tarefas, MCP — só que alimentadas pelo modelo de sua escolha.

---

## Comece Aqui

Se você não tem experiência com terminais ou quer o caminho mais fácil, comece pelos guias para iniciantes:

- [Configuração para Não-Técnicos](docs/non-technical-setup.md)
- [Início Rápido no Windows](docs/quick-start-windows.md)
- [Início Rápido no macOS / Linux](docs/quick-start-mac-linux.md)

Se você quer builds a partir do código-fonte, fluxos com Bun, launchers de perfil ou exemplos completos de providers, use:

- [Configuração Avançada](docs/advanced-setup.md)

---

## Instalação para Iniciantes

Para a maioria dos usuários, instale o pacote npm:

```bash
npm install -g @gitlawb/openclaude
```

O nome do pacote é `@gitlawb/openclaude`, mas o comando que você executa é:

```bash
openclaude
```

Se você instalar via npm e depois ver `ripgrep not found`, instale o ripgrep no sistema e confirme que `rg --version` funciona no mesmo terminal antes de iniciar o OpenClaude.

---

## Configuração Mais Rápida

### Windows PowerShell

```powershell
npm install -g @gitlawb/openclaude

$env:CLAUDE_CODE_USE_OPENAI="1"
$env:OPENAI_API_KEY="sk-sua-chave-aqui"
$env:OPENAI_MODEL="gpt-4o"

openclaude
```

### macOS / Linux

```bash
npm install -g @gitlawb/openclaude

export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=sk-sua-chave-aqui
export OPENAI_MODEL=gpt-4o

openclaude
```

Isso é suficiente para começar com OpenAI.

---

## Escolha Seu Guia

### Iniciante

- Quer a configuração mais fácil com passos para copiar e colar: [Configuração para Não-Técnicos](docs/non-technical-setup.md)
- No Windows: [Início Rápido no Windows](docs/quick-start-windows.md)
- No macOS ou Linux: [Início Rápido no macOS / Linux](docs/quick-start-mac-linux.md)

### Avançado

- Quer builds a partir do código-fonte, Bun, perfis locais, verificações de runtime ou mais opções de providers: [Configuração Avançada](docs/advanced-setup.md)

---

## Escolhas Comuns para Iniciantes

### OpenAI

Melhor opção padrão se você já tem uma chave de API da OpenAI.

### Ollama

Melhor se você quer rodar modelos localmente na sua própria máquina.

### Codex

Melhor se você já usa o Codex CLI ou o backend ChatGPT Codex.

### Atomic Chat

Melhor se você quer inferência local em Apple Silicon com o Atomic Chat. Veja [Configuração Avançada](docs/advanced-setup.md).

---

## Extensão para VS Code

Quer uma experiência nativa no VS Code? Use a extensão do repositório em `vscode-extension/openclaude-vscode` para lançar o terminal com um único comando e o tema `OpenClaude Terminal Black`.

## O Que Funciona

- **Todas as ferramentas**: Bash, FileRead, FileWrite, FileEdit, Glob, Grep, WebFetch, WebSearch, Agent, MCP, LSP, NotebookEdit, Tasks
- **Streaming**: Streaming de tokens em tempo real
- **Chamada de ferramentas**: Cadeias de ferramentas em múltiplos passos (o modelo chama ferramentas, recebe resultados e continua)
- **Imagens**: Imagens em Base64 e URL passadas para modelos de visão
- **Comandos slash**: /commit, /review, /compact, /diff, /doctor, etc.
- **Sub-agentes**: AgentTool cria sub-agentes usando o mesmo provider
- **Memória**: Sistema de memória persistente

## O Que é Diferente

- **Sem modo de raciocínio estendido**: O extended thinking da Anthropic está desabilitado (modelos OpenAI usam raciocínio diferente)
- **Sem cache de prompt**: Headers de cache específicos da Anthropic são ignorados
- **Sem funcionalidades beta**: Headers beta específicos da Anthropic são ignorados
- **Limites de tokens**: Padrão de 32K de saída máxima — alguns modelos podem ter limite menor, o que é tratado graciosamente

---

## Busca e Fetch na Web

Por padrão, o `WebSearch` está desabilitado para todos os providers não-Anthropic. O backend de busca nativo requer a API da Anthropic ou o endpoint de respostas do Codex, então usuários de GPT-4o, DeepSeek, Gemini, Ollama e outros providers compatíveis com OpenAI não têm busca na web.

O `WebFetch` funciona, mas usa HTTP básico com conversão de HTML para markdown. Isso falha em páginas renderizadas por JavaScript (React, Next.js, Vue SPAs) e sites que bloqueiam requisições HTTP simples.

Defina uma chave de API do [Firecrawl](https://firecrawl.dev) para corrigir os dois:

```bash
export FIRECRAWL_API_KEY=sua-chave-aqui
```

Com isso definido:

- `WebSearch` é habilitado para todos os providers e roteado pela API de busca do Firecrawl
- `WebFetch` usa o endpoint de scrape do Firecrawl em vez de HTTP puro, lidando corretamente com páginas renderizadas por JS

O plano gratuito em [firecrawl.dev](https://firecrawl.dev) inclui 500 créditos. A chave é opcional — se não definida, ambas as ferramentas voltam ao comportamento original.

---

## Como Funciona

O shim (`src/services/api/openaiShim.ts`) fica entre o Claude Code e a API do LLM:

```
Sistema de Ferramentas do Claude Code
        |
        v
  Interface do SDK Anthropic (duck-typed)
        |
        v
  openaiShim.ts  <-- traduz formatos
        |
        v
  API de Chat Completions OpenAI
        |
        v
  Qualquer modelo compatível
```

Ele traduz:
- Blocos de mensagem Anthropic → mensagens OpenAI
- tool_use/tool_result Anthropic → chamadas de função OpenAI
- Streaming SSE OpenAI → eventos de stream Anthropic
- Arrays de system prompt Anthropic → mensagens de sistema OpenAI

O restante do Claude Code não sabe que está falando com um modelo diferente.

---

## Notas sobre Qualidade dos Modelos

Nem todos os modelos são iguais no uso de ferramentas agênticas. Aqui está um guia aproximado:

| Modelo | Chamada de Ferramentas | Qualidade de Código | Velocidade |
|-------|-------------|-------------|-------|
| GPT-4o | Excelente | Excelente | Rápido |
| DeepSeek-V3 | Ótimo | Ótimo | Rápido |
| Gemini 2.0 Flash | Ótimo | Bom | Muito Rápido |
| Llama 3.3 70B | Bom | Bom | Médio |
| Mistral Large | Bom | Bom | Rápido |
| GPT-4o-mini | Bom | Bom | Muito Rápido |
| Qwen 2.5 72B | Bom | Bom | Médio |
| Modelos menores (<7B) | Limitado | Limitado | Muito Rápido |

Para melhores resultados, use modelos com forte suporte a chamadas de função/ferramentas.

---

## Arquivos Alterados em Relação ao Original

```
src/services/api/openaiShim.ts   — NOVO: Shim de API compatível com OpenAI (724 linhas)
src/services/api/client.ts       — Roteia para o shim quando CLAUDE_CODE_USE_OPENAI=1
src/utils/model/providers.ts     — Adicionado tipo de provider 'openai'
src/utils/model/configs.ts       — Adicionados mapeamentos de modelo openai
src/utils/model/model.ts         — Respeita OPENAI_MODEL para padrões
src/utils/auth.ts                — Reconhece OpenAI como provider 3P válido
```

6 arquivos alterados. 786 linhas adicionadas. Zero dependências adicionadas.

---

## Origem

Este é um fork de [instructkr/claude-code](https://gitlawb.com/node/repos/z6MkgKkb/instructkr-claude-code), que espelhou o snapshot do código-fonte do Claude Code que se tornou publicamente acessível por meio de uma exposição de source map do npm em 31 de março de 2026.

O código-fonte original do Claude Code é propriedade da Anthropic. Este repositório não é afiliado nem endossado pela Anthropic.

---

## Licença

Este repositório é fornecido para fins educacionais e de pesquisa. O código-fonte original está sujeito aos termos da Anthropic. As adições do shim OpenAI são de domínio público.
