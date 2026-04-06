package permissions

import _ "embed"

// DefaultBlocklist contem comandos bloqueados por padrao.
// Comandos perigosos que nunca devem ser executados sem permissao explicita.
var DefaultBlocklist = []string{
	"rm -rf /",
	"rm -rf /*",
	"mkfs",
	"dd if=/dev/zero",
	"shred",
	"wipe",
	"curl .* | .*sh",
	"wget .* | .*sh",
}

// MergeBlocklist combina a blocklist padrao com extensoes do usuario.
func MergeBlocklist(userEntries []string) []string {
	if len(userEntries) == 0 {
		return DefaultBlocklist
	}
	seen := make(map[string]bool, len(DefaultBlocklist)+len(userEntries))
	result := make([]string, 0, len(DefaultBlocklist)+len(userEntries))
	for _, e := range DefaultBlocklist {
		if !seen[e] {
			seen[e] = true
			result = append(result, e)
		}
	}
	for _, e := range userEntries {
		if !seen[e] {
			seen[e] = true
			result = append(result, e)
		}
	}
	return result
}
