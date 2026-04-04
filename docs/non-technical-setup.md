# OpenClaude para Usuários Não-Técnicos

Este guia é para pessoas que querem o caminho de configuração mais fácil.

Você não precisa compilar o código-fonte. Você não precisa do Bun. Você não precisa entender toda a base de código.

Se você consegue copiar e colar comandos em um terminal, consegue configurar isso.

## O Que o OpenClaude Faz

O OpenClaude permite que você use um assistente de codificação com IA com diferentes providers de modelos, como:

- OpenAI
- DeepSeek
- Gemini
- Ollama
- Codex

Para a maioria dos usuários de primeira viagem, a OpenAI é a opção mais fácil.

## Antes de Começar

Você precisa de:

1. Node.js 20 ou mais recente instalado
2. Uma janela de terminal
3. Uma chave de API do seu provider, a menos que você use um modelo local como o Ollama

## Caminho Mais Rápido

1. Instale o OpenClaude com npm
2. Defina 3 variáveis de ambiente
3. Execute `openclaude`

## Escolha Seu Sistema Operacional

- Windows: [Início Rápido no Windows](quick-start-windows.md)
- macOS / Linux: [Início Rápido no macOS / Linux](quick-start-mac-linux.md)

## Qual Provider Você Deve Escolher?

### OpenAI

Escolha este se:

- você quer a configuração mais fácil
- você já tem uma chave de API da OpenAI

### Ollama

Escolha este se:

- você quer rodar modelos localmente
- você não quer depender de uma API em nuvem para testes

### Codex

Escolha este se:

- você já usa o Codex CLI
- você já tem autenticação do Codex ou ChatGPT configurada

## Como é o Sucesso

Após executar `openclaude`, o CLI deve iniciar e aguardar seu prompt.

Nesse ponto, você pode pedir para ele:

- explicar código
- editar arquivos
- executar comandos
- revisar alterações

## Problemas Comuns

### Comando `openclaude` não encontrado

Causa:

- o npm instalou o pacote, mas seu terminal ainda não foi atualizado

Solução:

1. Feche o terminal
2. Abra um novo terminal
3. Execute `openclaude` novamente

### Chave de API inválida

Causa:

- a chave está errada, expirada ou copiada incorretamente

Solução:

1. Obtenha uma nova chave do seu provider
2. Cole-a novamente com cuidado
3. Execute `openclaude` novamente

### Ollama não está funcionando

Causa:

- Ollama não está instalado ou não está em execução

Solução:

1. Instale o Ollama em `https://ollama.com/download`
2. Inicie o Ollama
3. Tente novamente

## Quer Mais Controle?

Se você quer builds a partir do código-fonte, perfis avançados de provider, diagnósticos ou fluxos de trabalho com Bun, use:

- [Configuração Avançada](advanced-setup.md)
