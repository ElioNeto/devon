# Início Rápido — Windows

## 1. Instalar o Devon

Baixe o binário mais recente para Windows na [página de releases](https://github.com/ElioNeto/devon/releases) (`devon_windows_amd64.exe`) e adicione ao seu `PATH`.

Ou use o Windows Package Manager:

```powershell
# Via Scoop (se disponível)
scoop install devon
```

Confirme a instalação:

```powershell
devon --version
```

## 2. Configurar o Provider

Crie um arquivo `.env` na raiz do projeto que você quer usar:

```powershell
# Crie o arquivo .env com seu editor preferido ou via PowerShell:
@"
DEVON_API_KEY=sk-or-sua-chave-aqui
DEVON_BASE_URL=https://openrouter.ai/api/v1
DEVON_MODEL=mistralai/devstral-2512:free
"@ | Out-File -Encoding utf8 .env
```

> **Dica:** Crie uma conta gratuita em [openrouter.ai](https://openrouter.ai) para obter uma chave e usar modelos gratuitos.

## 3. Iniciar

```powershell
cd C:\caminho\do\seu\projeto
devon
```

## Usando com Ollama (local, sem chave)

```powershell
# Instalar Ollama
winget install Ollama.Ollama

# Abrir novo terminal e baixar um modelo
ollama pull qwen2.5-coder:7b
```

Configure o `.env`:

```
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

## Problemas Comuns

**`devon` não reconhecido como comando**
- Confirme que o binário está em um diretório listado no `PATH`
- Feche e reabra o terminal após adicionar ao `PATH`

**Ollama não conecta**
- Abra um terminal e execute `ollama serve`
- Confirme que o serviço está rodando com `ollama list`
