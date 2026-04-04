# Início Rápido do OpenClaude no macOS e Linux

Este guia usa um shell padrão como Terminal, iTerm, bash ou zsh.

## 1. Instalar o Node.js

Instale o Node.js 20 ou mais recente em:

- `https://nodejs.org/`

Em seguida, verifique:

```bash
node --version
npm --version
```

## 2. Instalar o OpenClaude

```bash
npm install -g @gitlawb/openclaude
```

## 3. Escolha Um Provider

### Opção A: OpenAI

Substitua `sk-sua-chave-aqui` pela sua chave real.

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=sk-sua-chave-aqui
export OPENAI_MODEL=gpt-4o

openclaude
```

### Opção B: DeepSeek

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=sk-sua-chave-aqui
export OPENAI_BASE_URL=https://api.deepseek.com/v1
export OPENAI_MODEL=deepseek-chat

openclaude
```

### Opção C: Ollama

Instale o Ollama primeiro em:

- `https://ollama.com/download`

Em seguida execute:

```bash
ollama pull llama3.1:8b

export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_BASE_URL=http://localhost:11434/v1
export OPENAI_MODEL=llama3.1:8b

openclaude
```

Nenhuma chave de API é necessária para modelos locais do Ollama.

## 4. Se `openclaude` Não For Encontrado

Feche o terminal, abra um novo e tente novamente:

```bash
openclaude
```

## 5. Se o Seu Provider Falhar

Verifique o básico:

### Para OpenAI ou DeepSeek

- certifique-se de que a chave é real
- certifique-se de que você a copiou completamente

### Para Ollama

- certifique-se de que o Ollama está instalado
- certifique-se de que o Ollama está em execução
- certifique-se de que o modelo foi baixado com sucesso

## 6. Atualizando o OpenClaude

```bash
npm install -g @gitlawb/openclaude@latest
```

## 7. Desinstalando o OpenClaude

```bash
npm uninstall -g @gitlawb/openclaude
```

## Precisa de Configuração Avançada?

Use:

- [Configuração Avançada](advanced-setup.md)
