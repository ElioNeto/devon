# Início Rápido — macOS / Linux

## 1. Instalar o Devon

```bash
curl -fsSL https://raw.githubusercontent.com/ElioNeto/devon/main/install.sh | bash
```

O script detecta automaticamente o sistema operacional e a arquitetura (amd64 / arm64) e instala o binário em `~/.local/bin`.

Confirme a instalação:

```bash
devon --version
```

## 2. Configurar o Provider

Crie um arquivo `.env` na raiz do projeto que você quer usar:

```bash
DEVON_API_KEY=sk-or-sua-chave-aqui
DEVON_BASE_URL=https://openrouter.ai/api/v1
DEVON_MODEL=mistralai/devstral-2512:free
```

> **Dica:** Crie uma conta gratuita em [openrouter.ai](https://openrouter.ai) para obter uma chave e usar modelos gratuitos como `mistralai/devstral-2512:free`.

## 3. Iniciar

```bash
cd /caminho/do/seu/projeto
devon
```

A TUI abrirá no terminal. Digite seu prompt e pressione `Enter`.

## Usando com Ollama (local, sem chave)

```bash
# Instalar Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Baixar um modelo
ollama pull qwen2.5-coder:7b

# Configurar o Devon para usar Ollama
DEVON_BASE_URL=http://localhost:11434/v1
DEVON_MODEL=qwen2.5-coder:7b
```

## Atalhos de Teclado

| Tecla | Ação |
|---|---|
| `Enter` | Enviar prompt |
| `Ctrl+C` | Interromper turno atual |
| `Ctrl+C` (duas vezes) | Sair do Devon |
| `!` | Abrir Command Palette |
