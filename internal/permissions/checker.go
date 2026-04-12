// Package permissions implements verificacao de permissoes e controle de acoes destrutivas.
package permissions

import "github.com/ElioNeto/devon/internal/config"

// PermissionLevel representa o nivel de permissao de uma ferramenta.
type PermissionLevel int

const (
	PermRead    PermissionLevel = iota // leitura segura
	PermWrite                          /* alteracoes em arquivos */
	PermExecute                        // execucao de comandos
)

func (p PermissionLevel) String() string {
	switch p {
	case PermRead:
		return "read"
	case PermWrite:
		return "write"
	case PermExecute:
		return "execute"
	default:
		return "unknown"
	}
}

// Tool é a interface mínima necessária para verificacao de permissao.
// Evita import circular com internal/tools.
type Tool interface {
	Name() string
	Permission() PermissionLevel
}

// Checker verifica se uma execucao de ferramenta requer confirmacao.
type Checker struct {
	// Mode é o modo de permissao ativo (config.Mode).
	Mode config.Mode
	// Session armazena ferramentas aprovadas "sempre para esta sessao".
	Session map[string]bool
	// Blocklist contem comandos/ferramentas bloqueadas.
	Blocklist []string
	// Allowlist, se definido, restringe execução APENAS aos itens da lista.
	Allowlist []string
}

// Requires retorna true se a execucao da ferramenta requer confirmacao.
// Tambem verifica se a ferramenta esta na blocklist (erro se bloqueada).
func (c *Checker) Requires(tool Tool) (blocked bool, needsConfirm bool) {
	// Verifica blocklist primeiro
	for _, b := range c.Blocklist {
		if b == tool.Name() {
			return true, false
		}
	}

	// Se allowlist está definida, só permite ferramentas na lista
	if len(c.Allowlist) > 0 {
		allowed := false
		for _, a := range c.Allowlist {
			if a == tool.Name() {
				allowed = true
				break
			}
		}
		if !allowed {
			return true, false
		}
	}

	// Sessao "always approve" pula confirmacao
	if c.Session[tool.Name()] {
		return false, false
	}

	return false, c.needsConfirmation(tool.Permission())
}

// Approve marca uma ferramenta como aprovada para o restante da sessao.
func (c *Checker) Approve(toolName string) {
	if c.Session == nil {
		c.Session = make(map[string]bool)
	}
	c.Session[toolName] = true
}

func (c *Checker) needsConfirmation(level PermissionLevel) bool {
	switch c.Mode {
	case config.ModeYolo:
		return false
	case config.ModeSafe:
		return true
	case config.ModeAuto:
		// Auto: write e execute precisam confirmacao, read nao
		switch level {
		case PermWrite, PermExecute:
			return true
		default:
			return false
		}
	default:
		// Padrao seguro
		return true
	}
}
