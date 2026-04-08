package tools

import (
	"time"

	"github.com/ElioNeto/devon/internal/config"
)

// RegisterBuiltin registra todas as ferramentas nativas no registry.
func RegisterBuiltin(r *Registry, dir string, timeout time.Duration, sandbox config.SandboxConfig) {
	r.Register(&BashTool{Dir: dir, Timeout: timeout, Sandbox: sandbox})
	r.Register(&ReadTool{Dir: dir})
	r.Register(&WriteTool{Dir: dir})
	r.Register(&EditTool{Dir: dir})
	r.Register(&GlobTool{Dir: dir})
	r.Register(&GrepTool{Dir: dir})
	r.Register(&ListDirTool{Dir: dir})
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
