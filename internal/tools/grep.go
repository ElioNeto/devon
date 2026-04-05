package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GrepTool busca conteudo por regex em arquivos.
type GrepTool struct {
	Dir       string
	MaxLines  int
	MaxFiles  int
	MaxMatchSize int
}

type grepParams struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`    // diretorio ou arquivo (vazio = workdir)
	NoCase  bool   `json:"no_case"` // busca case-insensitive
}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Busca um padrao regex em arquivos. Retorna linhas com matches, arquivos e numeros de linha." }
func (t *GrepTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "O padrao regex a buscar"
			},
			"path": {
				"type": "string",
				"description": "Arquivo ou diretorio onde buscar (padrao: raiz do projeto)"
			},
			"no_case": {
				"type": "boolean",
				"description": "Se true, busca case-insensitive (padrao: false)"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *GrepTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p grepParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("grep: parametros invalidos: %w", err)
	}
	if p.Pattern == "" {
		return "", fmt.Errorf("grep: padrao nao pode estar vazio")
	}

	dir := t.Dir
	if dir == "" {
		dir = "."
	}

	searchPath := dir
	if p.Path != "" {
		if !filepath.IsAbs(p.Path) {
			searchPath = filepath.Join(dir, p.Path)
		} else {
			searchPath = p.Path
		}
	}

	opts := regexpOptions(p)
	maxMatchSize := t.MaxMatchSize
	if maxMatchSize == 0 {
		maxMatchSize = 32 * 1024
	}

	var results []string
	var fileCount int
	maxFiles := t.MaxFiles
	if maxFiles == 0 {
		maxFiles = 50
	}

	err := filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignora erros, continua andando
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			fileCount++
			if fileCount > maxFiles {
				return fmt.Errorf("grep: atingido limite maximo de arquivos (%d)", maxFiles)
			}
			lines, err := readFileMatches(ctx, path, opts, t.MaxLines)
			if err != nil {
				return nil
			}
			for _, line := range lines {
				results = append(results, fmt.Sprintf("%s:%d:%s", path, line.Num, line.Text))
				if len(results) >= 200 {
					return fmt.Errorf("grep: muitas correspondencias, truncado apos 200")
				}
			}
		}
		return nil
	})

	if len(results) == 0 {
		if err != nil && err.Error() == "grep: muitas correspondencias, truncado apos 200" {
			return strings.Join(results, "\n") + "\n... (truncado: mais de 200 correspondencias)", nil
		}
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Nenhuma correspondencia encontrada para o padrao %q.", p.Pattern), nil
	}

	out := strings.Join(results, "\n")
	out = sanitizeLineLimit(out)
	return out, nil
}

type lineMatch struct {
	Num  int
	Text string
}

func readFileMatches(ctx context.Context, path string, opts regexpSyntax, maxLines int) ([]lineMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("grep ao abrir: %w", err)
	}
	defer f.Close()

	re, err := regexp.Compile(opts.pattern)
	if err != nil {
		return nil, fmt.Errorf("grep ao compilar padrao: %w", err)
	}

	var matches []lineMatch
	scanner := bufio.NewScanner(f)
	// Aumenta buffer para linhas longas
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	lineNum := 0

	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			if maxLines > 0 && len(matches) >= maxLines {
				break
			}
			matches = append(matches, lineMatch{Num: lineNum, Text: line})
		}
	}

	return matches, nil
}

// shouldSkipDir verifica se um diretorio deve ser ignorado durante a busca.
func shouldSkipDir(name string) bool {
	skip := map[string]bool{
		".git":    true,
		"node_modules": true,
		"vendor":  true,
		".cache":  true,
		".tox":    true,
		"__pycache__": true,
		".eggs":   true,
	}
	return skip[name]
}

type regexpSyntax struct {
	pattern string
}

func regexpOptions(p grepParams) regexpSyntax {
	opts := regexpSyntax{pattern: p.Pattern}
	if p.NoCase {
		opts.pattern = "(?i)" + p.Pattern
	}
	return opts
}

func sanitizeLineLimit(s string) string {
	const maxLen = 32 * 1024
	if len(s) > maxLen {
		return s[:maxLen] + "\n... [saida truncada: excedeu limite de 32 KB]"
	}
	return s
}
