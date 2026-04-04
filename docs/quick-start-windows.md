# Início Rápido — Windows

## 1. Instalar dependências

Você precisa de **Node.js** e **Bun**.

- Baixe o Node.js em [nodejs.org](https://nodejs.org) (versão LTS)
- Instale o Bun no PowerShell:

```powershell
powershell -c "irm bun.sh/install.ps1 | iex"
```

Reinicie o terminal e verifique:

```powershell
bun --version   # 1.3.11 ou mais recente
node --version  # 18 ou mais recente
```

## 2. Clonar e compilar

```powershell
git clone https://github.com/ElioNeto/devon.git
cd devon
bun install
bun run build
npm link
```

## 3. Configurar provider

Veja [Configuração Avançada](advanced-setup.md) para todos os providers. O mais rápido para começar é o OpenRouter (gratuito, sem cartão):

1. Crie sua chave em [openrouter.ai/keys](https://openrouter.ai/keys)
2. Configure no PowerShell:

```powershell
$env:CLAUDE_CODE_USE_OPENAI = "1"
$env:OPENAI_API_KEY         = "sk-or-sua-chave-aqui"
$env:OPENAI_BASE_URL        = "https://openrouter.ai/api/v1"
$env:OPENAI_MODEL           = "mistralai/devstral-2512:free"
```

Ou crie um `.env` e carregue com um script `start.ps1`:

```powershell
# start.ps1
Get-Content .env | ForEach-Object {
  if ($_ -match '^\s*([^#][^=]*)=(.*)$') {
    [System.Environment]::SetEnvironmentVariable($Matches[1].Trim(), $Matches[2].Trim(), 'Process')
  }
}
devon
```

## 4. Iniciar

```powershell
devon
```

## Próximos passos

- [Configuração Avançada](advanced-setup.md) — outros providers, perfis, diagnósticos
- Crie um `DEVON.md` na raiz do projeto para dar contexto permanente ao agente
