# OpenClaude Quick Start for macOS and Linux

This guide uses a standard shell such as Terminal, iTerm, bash, or zsh.

## 1. Install Node.js

Install Node.js 20 or newer from:

- `https://nodejs.org/`

Then check it:

```bash
node --version
npm --version
```

## 2. Install OpenClaude

```bash
npm install -g @gitlawb/openclaude
```

## 3. Pick One Provider

### Option A: OpenAI

Replace `sk-your-key-here` with your real key.

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=sk-your-key-here
export OPENAI_MODEL=gpt-4o

openclaude
```

### Option B: DeepSeek

```bash
export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_API_KEY=sk-your-key-here
export OPENAI_BASE_URL=https://api.deepseek.com/v1
export OPENAI_MODEL=deepseek-chat

openclaude
```

### Option C: Ollama

Install Ollama first from:

- `https://ollama.com/download`

Then run:

```bash
ollama pull llama3.1:8b

export CLAUDE_CODE_USE_OPENAI=1
export OPENAI_BASE_URL=http://localhost:11434/v1
export OPENAI_MODEL=llama3.1:8b

openclaude
```

No API key is needed for Ollama local models.

## 4. If `openclaude` Is Not Found

Close the terminal, open a new one, and try again:

```bash
openclaude
```

## 5. If Your Provider Fails

Check the basics:

### For OpenAI or DeepSeek

- make sure the key is real
- make sure you copied it fully

### For Ollama

- make sure Ollama is installed
- make sure Ollama is running
- make sure the model was pulled successfully

## 6. Updating OpenClaude

```bash
npm install -g @gitlawb/openclaude@latest
```

## 7. Uninstalling OpenClaude

```bash
npm uninstall -g @gitlawb/openclaude
```

## Need Advanced Setup?

Use:

- [Advanced Setup](advanced-setup.md)
