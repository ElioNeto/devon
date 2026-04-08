package tools

import (
	"fmt"
	"path/filepath"
)

// ensurePath resolve um caminho relativo ao Dir e valida que não escapa
// do diretório de trabalho (após resolver symlinks e "..").
// Retorna o path absoluto canônico ou um erro descritivo.
func ensurePath(p string, dir string) (string, error) {
	if filepath.IsAbs(p) {
		abs := filepath.Clean(p)
		if !withinDir(abs, dir) {
			return "", fmt.Errorf("caminho %q fora do diretório de trabalho %q", p, dir)
		}
		return abs, nil
	}
	if dir == "" {
		dir = "."
	}
	abs := filepath.Clean(filepath.Join(dir, p))
	if !withinDir(abs, dir) {
		return "", fmt.Errorf("caminho %q escapa do diretório de trabalho %q", p, dir)
	}
	return abs, nil
}

// withinDir verifica se abs está dentro de dir (ou é igual a dir).
func withinDir(abs, dir string) bool {
	if dir == "." {
		// "." permite qualquer caminho relativo já limpo
		return true
	}
	return abs == dir || hasPrefixDir(dir, abs)
}

// hasPrefixDir verifica se abs está contido em dir (compara strings limpanas).
func hasPrefixDir(dir, abs string) bool {
	d := filepath.Clean(dir)
	if !filepath.IsAbs(d) {
		// Se dir é ".", aceita qualquer caminho relativo
		return true
	}
	if !filepath.IsAbs(abs) {
		return false
	}
	return abs == d || len(abs) > len(d) && abs[len(d)] == '/' && abs[:len(d)] == d
}
