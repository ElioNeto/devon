package tools

import (
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/ElioNeto/devon/internal/memory"
	"github.com/ElioNeto/devon/internal/tools/web"
)

// RegisterBuiltin registra todas as ferramentas nativas no registry.
func RegisterBuiltin(r *Registry, dir string, timeout time.Duration, sandbox config.SandboxConfig) {
	r.Register(&BashTool{Dir: dir, Timeout: timeout, Sandbox: sandbox})
	r.Register(&ReadTool{Dir: dir})
	r.Register(&WriteTool{Dir: dir})
	r.Register(&EditTool{Dir: dir})
	r.Register(&PatchTool{Dir: dir})
	r.Register(&GlobTool{Dir: dir})
	r.Register(&GrepTool{Dir: dir})
	r.Register(&ListDirTool{Dir: dir})
}

// RegisterMemoryTools registers memory tools (remember, recall).
func RegisterMemoryTools(r *Registry, mem *memory.Manager, projectID string) {
	r.Register(&memory.RememberTool{Manager: mem, ProjectID: projectID})
	r.Register(&memory.RecallTool{Manager: mem, ProjectID: projectID})
}

// RegisterWebTools registra as ferramentas web_search e web_fetch se habilitadas.
func RegisterWebTools(r *Registry, cfg *config.WebConfig) {
	if cfg != nil && cfg.Enabled {
		r.Register(&web.SearchTool{Config: cfg})
		r.Register(&web.FetchTool{Config: cfg})
	}
}

// RegisterBuiltinWithConfig oferece controle granular sobre cada tool.
func RegisterBuiltinWithConfig(r *Registry, dir string, timeout time.Duration, sandbox config.SandboxConfig, maxLines, maxFiles, maxMatchSize int) {
	r.Register(&BashTool{Dir: dir, Timeout: timeout, Sandbox: sandbox})
	r.Register(&ReadTool{Dir: dir})
	r.Register(&WriteTool{Dir: dir})
	r.Register(&EditTool{Dir: dir})
	r.Register(&GlobTool{Dir: dir})
	r.Register(&GrepTool{
		Dir:          dir,
		MaxLines:     maxLines,
		MaxFiles:     maxFiles,
		MaxMatchSize: maxMatchSize,
	})
	r.Register(&ListDirTool{Dir: dir})
}
