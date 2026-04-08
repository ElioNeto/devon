# Devon para Iniciantes

Este guia é para quem quer começar a usar o Devon sem precisar compilar código ou entender a arquitetura interna. Se você consegue copiar e colar comandos em um terminal, consegue configurar o Devon.

---

## O que o Devon faz

Devon é um agente de código que roda no seu terminal e usa um modelo de linguagem (LLM) para:

- Ler e editar arquivos do seu projeto
- Executar comandos no terminal
- Entender e refatorar código
- Criar testes, documentação e muito mais

Você escreve o que quer que seja feito. O Devon executa.

---

## Pré-requisitos

Você precisa de:

1. Um terminal (Terminal no macOS/Linux, PowerShell ou Windows Terminal no Windows)
2. Uma chave de API de um provider de LLM — ou Ollama instalado localmente

---

## Escolhendo um Provider

| Provider | Precisa de chave? | Indicado para |
|---|---|---|
| [OpenRouter](https://openrouter.ai) | Sim (gratuita disponível) | Primeira vez — modelos gratuitos disponíveis |
| [OpenAI](https://platform.openai.com) | Sim | Qualidade e confiabilidade |
| [Ollama](https://ollama.com) | Não | Rodar localmente, sem custo e sem dados na nuvem |
| [DeepSeek](https://platform.deepseek.com) | Sim | Custo-benefício |

Para a maioria dos iniciantes, **OpenRouter com um modelo gratuito** é o caminho mais fácil.

---

## Instalação

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/ElioNeto/devon/main/install.sh | bash
```

**Windows:** veja o [Guia de Início Rápido para Windows](quick-start-windows.md).

---

## Configuração

Crie um arquivo `.env` na pasta do projeto que você quer usar com o Devon:

```bash
DEVON_API_KEY=sk-or-sua-chave-aqui
DEVON_BASE_URL=https://openrouter.ai/api/v1
DEVON_MODEL=mistralai/devstral-2512:free
```

Depois, inicie:

```bash
devon
```

---

## Problemas Comuns

**Comando `devon` não encontrado**
- Feche e abra o terminal novamente
- Verifique se o diretório `~/.local/bin` está no seu `PATH`

**Erro de chave de API inválida**
- Confirme que a chave foi copiada corretamente
- Gere uma nova chave no painel do provider

**Ollama não está funcionando**
- Instale o Ollama em [ollama.com](https://ollama.com)
- Execute `ollama serve` em um terminal separado
- Confirme com `ollama list` que o modelo está baixado

---

## Próximos Passos

Quando quiser explorar configurações mais avançadas — múltiplos providers, perfis com fallback, uso em CI/CD — veja o [Guia de Configuração Avançada](advanced-setup.md).
