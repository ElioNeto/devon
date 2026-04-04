# Política de Segurança

## Versões Suportadas

O Open Claude é atualmente mantido no branch `main` mais recente e apenas no release npm mais recente.

| Versão | Suportada |
| ------- | --------- |
| Último release | :white_check_mark: |
| Releases antigos | :x: |
| Forks não oficiais / builds modificados | :x: |

As correções de segurança geralmente são lançadas na próxima versão de patch e também podem ser aplicadas diretamente no `main` antes que um release de pacote seja publicado.

## Reportando uma Vulnerabilidade

Se você acredita ter encontrado uma vulnerabilidade de segurança no Open Claude, por favor reporte-a de forma privada.

Canal de reporte preferido:

- GitHub Security Advisories / relato privado de vulnerabilidades para este repositório

Por favor inclua:

- uma descrição clara do problema
- versão afetada, commit ou ambiente
- passos de reprodução ou uma prova de conceito
- avaliação de impacto
- qualquer remediação sugerida, se disponível

Por favor, **não** abra uma issue pública para uma vulnerabilidade sem correção.

## Processo de Resposta

Nossos objetivos gerais são:

- reconhecimento inicial de triagem em até 7 dias
- acompanhamento após validação quando conseguirmos reproduzir o problema
- divulgação coordenada após uma correção estar disponível

A gravidade, a exploitabilidade e a capacidade de manutenção podem afetar os prazos.

## Divulgação e CVEs

Relatórios válidos podem ser corrigidos privadamente primeiro e divulgados após um patch estar disponível.

Se um relatório for aceito e o problema for significativo o suficiente para justificar rastreamento formal, podemos publicar um GitHub Security Advisory e solicitar ou atribuir um CVE pelo canal apropriado. A emissão de CVE não é garantida para todo relatório.

## Escopo

Esta política se aplica a:

- o código-fonte do Open Claude neste repositório
- artefatos de release oficiais publicados a partir deste repositório
- o pacote npm `@gitlawb/openclaude`

Esta política não cobre:

- providers de modelos de terceiros, endpoints ou serviços hospedados
- má configuração local na máquina do relator
- vulnerabilidades em forks não oficiais, espelhos ou reempacotamentos downstream
