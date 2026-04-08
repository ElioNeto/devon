# OpenClaude no Android (Termux)

Um guia completo para executar o OpenClaude no Android usando Termux + proot Ubuntu.

---

## Pré-requisitos

- Celular Android com ~700MB de armazenamento livre
- [Termux](https://f-droid.org/en/packages/com.termux/) instalado pelo **F-Droid** (não pela Play Store)
- Uma chave de API do [OpenRouter](https://openrouter.ai) (gratuita, sem necessidade de cartão de crédito)

---

## Por Que Esta Configuração?

O OpenClaude requer o [Bun](https://bun.sh) para fazer o build, e o Bun não suporta Android nativamente. A solução é executar um ambiente Ubuntu real dentro do Termux via `proot-distro`, onde o binário Linux do Bun funciona corretamente.

---

## Instalação

### Passo 1 — Atualizar o Termux

```bash
pkg update && pkg upgrade
```

Pressione `N` ou Enter em qualquer prompt de conflito de arquivo de configuração.

### Passo 2 — Instalar dependências

```bash
pkg install nodejs-lts git proot-distro
```

Verifique o Node.js:
```bash
node --version  # deve ser v20+
```

### Passo 3 — Clonar o OpenClaude

```bash
git clone https://github.com/Gitlawb/openclaude.git
cd openclaude
npm install
npm link
```

### Passo 4 — Instalar Ubuntu via proot

```bash
proot-distro install ubuntu
```

Isso baixa ~200–400MB. Aguarde a conclusão.

### Passo 5 — Instalar Bun dentro do Ubuntu

```bash
proot-distro login ubuntu
curl -fsSL https://bun.sh/install | bash
source ~/.bashrc
bun --version  # deve mostrar 1.3.11+
```

### Passo 6 — Fazer o build do OpenClaude

```bash
cd /data/data/com.termux/files/home/openclaude
bun run build
```

Você deve ver:
```
✓ Built openclaude v0.1.6 → dist/cli.mjs
```

### Passo 7 — Salvar variáveis de ambiente permanentemente

Ainda dentro do Ubuntu, adicione sua configuração do OpenRouter ao `.bashrc`:

```bash
echo 'export CLAUDE_CODE_USE_OPENAI=1' >> ~/.bashrc
echo 'export OPENAI_API_KEY=sua_chave_openrouter_aqui' >> ~/.bashrc
echo 'export OPENAI_BASE_URL=https://openrouter.ai/api/v1' >> ~/.bashrc
echo 'export OPENAI_MODEL=qwen/qwen3.6-plus-preview:free' >> ~/.bashrc
source ~/.bashrc
```

Substitua `sua_chave_openrouter_aqui` pela sua chave real de [openrouter.ai/keys](https://openrouter.ai/keys).

### Passo 8 — Executar o OpenClaude

```bash
node dist/cli.mjs
```

Selecione **3** (plataforma de terceiros) na tela de login. Suas variáveis de ambiente serão detectadas automaticamente.

---

## Reiniciando Após Fechar o Termux

Toda vez que você reabrir o Termux após fechá-lo, execute:

```bash
proot-distro login ubuntu
cd /data/data/com.termux/files/home/openclaude
node dist/cli.mjs
```

---

## Modelo Gratuito Recomendado

**`qwen/qwen3.6-plus-preview:free`** — Melhor modelo gratuito no OpenRouter em abril de 2026.

- Janela de contexto de 1M tokens
- Supera Claude 4.5 Opus no Terminal-Bench 2.0 de codificação agêntica (61,6 vs 59,3)
- Raciocínio chain-of-thought integrado
- Uso nativo de ferramentas e chamada de funções
- $0/M tokens (período de preview)

> ⚠️ O status gratuito pode mudar quando o período de preview terminar. Verifique em [openrouter.ai](https://openrouter.ai/qwen/qwen3.6-plus-preview:free) os preços atuais.

---

## Modelos Gratuitos Alternativos (OpenRouter)

| ID do Modelo | Contexto | Notas |
|---|---|---|
| `qwen/qwen3-coder:free` | 262K | Melhor para tarefas puramente de código |
| `openai/gpt-oss-120b:free` | 131K | Modelo aberto da OpenAI, forte chamada de ferramentas |
| `nvidia/nemotron-3-super-120b-a12b:free` | 262K | MoE híbrido, bom uso geral |
| `meta-llama/llama-3.3-70b-instruct:free` | 66K | Confiável, amplamente testado |

Troque de modelo a qualquer hora:
```bash
export OPENAI_MODEL=qwen/qwen3-coder:free
node dist/cli.mjs
```

---

## Por Que Não Groq ou Cerebras?

Ambos foram testados e falham por causa do grande system prompt do OpenClaude (~50K tokens):

- **Plano gratuito Groq**: Limites de TPM muito baixos (6K–12K tokens/min)
- **Plano gratuito Cerebras**: Limites de TPM excedidos, mesmo no `llama3.1-8b`

Modelos gratuitos do OpenRouter não têm restrições de TPM — apenas 20 req/min e 200 req/dia.

---

## Dicas

- **Não deslize o Termux** para fora dos apps recentes durante uma sessão — use o botão home para minimizar.
- O ambiente Ubuntu persiste entre sessões do Termux; seu build e configuração são salvos.
- Execute `bun run build` novamente apenas se você baixar atualizações do repositório do OpenClaude.
