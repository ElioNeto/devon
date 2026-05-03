// Package cache fornece cache de respostas do LLM por hash deterministico de contexto.
package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/ElioNeto/devon/internal/llm"
)

// MessageForTask cria uma lista de mensagens com uma unica mensagem de usuario
// contendo a tarefa. Usado para gerar chaves de cache no fluxo one-shot.
func MessageForTask(task string) []llm.Message {
	return []llm.Message{
		{Role: llm.RoleUser, Content: llm.TextContent(task)},
	}
}

// HashKey gera um hash deterministico SHA-256 para uma combinacao de model + messages.
// A serializacao das mensagens usa json.Marshal sobre structs com tags JSON,
// que em Go produz saida deterministica (struct fields sao serializados em ordem).
// Mesma entrada sempre produz a mesma saida.
func HashKey(model string, messages []llm.Message) string {
	h := sha256.New()

	// Precede com o model name + separator para evitar colisoes entre modelos
	h.Write([]byte(model))
	h.Write([]byte{0})

	// Serializa mensagens via json.Marshal (deterministico para structs)
	data, _ := json.Marshal(messages) // llm.Message has json tags, serialization never fails
	h.Write(data)

	return fmt.Sprintf("%x", h.Sum(nil))
}
