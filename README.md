# Devon

<p align="center">
  <strong>Agente de código autônomo para o terminal — escrito em Go.</strong>
</p>

<p align="center">
  <a href="https://github.com/ElioNeto/devon/actions/workflows/ci.yml"><img src="https://github.com/ElioNeto/devon/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/ElioNeto/devon/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <a href="https://github.com/ElioNeto/devon/releases"><img src="https://img.shields.io/github/v/release/ElioNeto/devon" alt="Release"></a>
</p>

---

Devon é um agente de código que roda inteiramente no terminal. Você escreve o que quer que seja feito; o Devon lê arquivos, edita código e executa comandos — com visibilidade total de cada passo via TUI. Conecta a qualquer LLM compatível com OpenAI: OpenRouter, Gemini, Groq, Ollama, Anthropic, DeepSeek ou qualquer endpoint próprio.

Distribuído como binário único estático. Sem Node, sem Python, sem runtime externo.

## Instalação

```bash
curl -fsSL https://raw.githubusercontent.com/ElioNeto/devon/main/install.sh | bash
```

Ou compile do fonte:

```bash
git clone https://github.com/ElioNeto/devon.git && cd devon && make build
```

## Início Rápido

Crie um `.env` na raiz do projeto com as credenciais do provider:

```bash
DEVON_API_KEY=sk-or-sua-chave-aqui
DEVON_BASE_URL=https://openrouter.ai/api/v1
DEVON_MODEL=mistralai/devstral-2512:free
```

Depois, inicie:

```bash
devon
```

O Devon carrega o `.env` automaticamente. Para usar Ollama localmente, sem chave, defina `DEVON_BASE_URL=http://localhost:11434/v1` e `DEVON_MODEL=qwen2.5-coder:32b`.

## Controle de Permissões

Por padrão (`--mode auto`), o Devon lê arquivos livremente e pede confirmação antes de escrever ou executar comandos. Use `--mode safe` para confirmar tudo, ou `--mode yolo` para execução autônoma sem interrupções.

```bash
devon --mode safe    # máximo controle
devon --mode yolo    # máxima velocidade
```

## Modo Não-Interativo

O subcomando `run` executa uma tarefa sem abrir a TUI — útil em scripts, hooks de git e CI/CD:

```bash
devon run "adicione testes para o pacote internal/db"
echo "refatore auth.go" | devon run
```

## Documentação

- [Configuração Avançada](docs/advanced-setup.md) — múltiplos providers, perfis, fallback automático
- [Início Rápido — macOS / Linux](docs/quick-start-mac-linux.md)
- [Início Rápido — Windows](docs/quick-start-windows.md)
- [Contribuindo](CONTRIBUTING.md)
- [Roadmap](ROADMAP.md)
- [Segurança](SECURITY.md)

## Licença

MIT — veja [LICENSE](LICENSE) para detalhes.
