# Política de Segurança

## Versões Suportadas

Apenas o branch `main` mais recente e o último release publicado recebem correções de segurança.

| Versão | Suportada |
|---|---|
| Último release | ✅ |
| Releases anteriores | ❌ |
| Forks não oficiais / builds modificados | ❌ |

## Reportando uma Vulnerabilidade

Se você acredita ter encontrado uma vulnerabilidade de segurança no Devon, **não abra uma issue pública**. Reporte de forma privada pelo canal abaixo:

**Canal preferido:** [GitHub Security Advisories](https://github.com/ElioNeto/devon/security/advisories/new) — reporte privado diretamente neste repositório.

### O que incluir no relatório

- Descrição clara do problema e seu impacto
- Versão afetada, commit ou ambiente
- Passos de reprodução ou prova de conceito
- Remediação sugerida, se disponível

## Processo de Resposta

| Etapa | Prazo estimado |
|---|---|
| Triagem inicial e reconhecimento | Até 7 dias |
| Acompanhamento após reprodução | Após confirmação |
| Divulgação coordenada | Após patch disponível |

Gravidade, exploitabilidade e capacidade de manutenção podem afetar os prazos.

## Divulgação e CVEs

Vulnerabilidades válidas são corrigidas de forma privada antes da divulgação pública. Para problemas significativos, podemos publicar um GitHub Security Advisory e solicitar um CVE pelo canal apropriado. A emissão de CVE não é garantida para todo relatório.

## Escopo

**Esta política cobre:**
- Código-fonte do Devon neste repositório
- Artefatos de release oficiais publicados a partir deste repositório

**Esta política não cobre:**
- Providers de modelos de terceiros, endpoints ou serviços hospedados
- Má configuração local na máquina do relator
- Vulnerabilidades em forks não oficiais, espelhos ou reempacotamentos downstream
