# /memory — Inspecionar memória semântica

**Uso:** `/memory [clear|list|stats]`

**Exemplos:**
- `/memory list` — lista todos os fatos armazenados no projeto atual
- `/memory stats` — mostra contagem de fatos por categoria
- `/memory clear` — remove todos os fatos (equivale a `devon memory clear`)

**O que faz:**

**`/memory list`**
```bash
devon memory list   # se o subcomando existir
# ou diretamente no SQLite:
sqlite3 .devon/state.db "SELECT category, content, confidence FROM facts WHERE project_id='<id>' ORDER BY created_at DESC;"
```

**`/memory stats`**
```bash
sqlite3 .devon/state.db "SELECT category, COUNT(*) as total FROM facts GROUP BY category ORDER BY total DESC;"
```

**`/memory clear`**
```bash
devon memory clear
```

**Onde fica o banco:**
`<WorkDir>/.devon/state.db` (padrão configurado por `DEVON_DB_PATH`)

**Estrutura da tabela `facts`:**
```sql
facts(
  id         INTEGER PRIMARY KEY,
  project_id TEXT NOT NULL,
  category   TEXT NOT NULL,    -- ex: "convention", "architecture", "error"
  content    TEXT NOT NULL,    -- o fato em si
  context    TEXT,             -- contexto adicional
  confidence REAL DEFAULT 1.0, -- 0.0 a 1.0
  created_at DATETIME,
  updated_at DATETIME
)
```
