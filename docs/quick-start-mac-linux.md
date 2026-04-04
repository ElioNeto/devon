# Início Rápido — macOS / Linux

## 1. Instalar dependências

Você precisa de **Bun** e **Node.js**.

```bash
# instalar Bun
curl -fsSL https://bun.sh/install | bash

# verificar
bun --version   # 1.3.11 ou mais recente
node --version  # 18 ou mais recente
```

## 2. Clonar e compilar

```bash
git clone https://github.com/ElioNeto/devon.git
cd devon
bun install
bun run build
npm link
```

Verifique que o comando está disponível:

```bash
which devon   # deve retornar o caminho do binário
```

## 3. Configurar provider

Veja [Configuração Avançada](advanced-setup.md) para todos os providers. O mais rápido para começar é o OpenRouter (gratuito, sem cartão):

1. Crie sua chave em [openrouter.ai/keys](https://openrouter.ai/keys)
2. Crie um `.env` no projeto que quiser usar:

```bash
cat > .env << 'EOF'
CLAUDE_CODE_USE_OPENAI=1
OPENAI_API_KEY=sk-or-sua-chave-aqui
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_MODEL=mistralai/devstral-2512:free
EOF

echo ".env" >> .gitignore
```

## 4. Iniciar

```bash
set -a && source .env && set +a
devon
```

Ou crie um atalho `start.sh` na raiz do projeto:

```bash
cat > start.sh << 'EOF'
#!/bin/bash
set -a
source .env
set +a
devon
EOF
chmod +x start.sh
./start.sh
```

## Próximos passos

- [Configuração Avançada](advanced-setup.md) — outros providers, perfis, diagnósticos
- Crie um `DEVON.md` na raiz do projeto para dar contexto permanente ao agente
