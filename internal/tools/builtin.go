package tools

import (
	"time"
)

// RegisterBuiltin registra todas as ferramentas nativas (bash, read, write,
// glob, grep) no registry.
func RegisterBuiltin(r *Registry, dir string, timeout time.Duration) {
	r.Register(&BashTool{Dir: dir, Timeout: timeout})
	r.Register(&ReadTool{Dir: dir})
	r.Register(&WriteTool{Dir: dir})
	r.Register(&GlobTool{Dir: dir})
	r.Register(&GrepTool{Dir: dir})
}

// RegisterBuiltinWithConfig oferece controle granular sobre cada tool.
func RegisterBuiltinWithConfig(r *Registry, dir string, timeout time.Duration, maxLines, maxFiles, maxMatchSize int) {
	r.Register(&BashTool{Dir: dir, Timeout: timeout})
	r.Register(&ReadTool{Dir: dir})
	r.Register(&WriteTool{Dir: dir})
	r.Register(&GlobTool{Dir: dir})
	r.Register(&GrepTool{
		Dir:          dir,
		MaxLines:     maxLines,
		MaxFiles:     maxFiles,
		MaxMatchSize: maxMatchSize,
	})
}
