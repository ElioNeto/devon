package llm

import (
	"encoding/json"
)

// ContentPartType indicates the type of content in a message part.
type ContentPartType string

const (
	TypeText     ContentPartType = "text"
	TypeImageURL ContentPartType = "image_url"
)

// ImageURL holds an inline base64-encoded image for multimodal messages.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ContentPart is a single part of a multimodal message.
type ContentPart struct {
	Type     ContentPartType `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *ImageURL       `json:"image_url,omitempty"`
}

// HasVisionContent returns true if the message contains image parts.
func HasVisionContent(parts []ContentPart) bool {
	for _, p := range parts {
		if p.Type == TypeImageURL {
			return true
		}
	}
	return false
}

// NewTextPart creates a text content part.
func NewTextPart(text string) ContentPart {
	return ContentPart{Type: TypeText, Text: text}
}

// NewImagePartBase64 creates an image content part from a base64-encoded data URI.
// dataURI should be in the form "data:image/png;base64,...".
func NewImagePartBase64(dataURI string) ContentPart {
	return ContentPart{
		Type: TypeImageURL,
		ImageURL: &ImageURL{
			URL:    dataURI,
			Detail: "auto",
		},
	}
}

// MarshalJSON implements json.Marshaler for Message.
// When ContentParts is set, it serialises content as the array of parts.
// Otherwise it uses the legacy flat-content serialisation (OpenAI-compatible).
func (m Message) MarshalJSON() ([]byte, error) {
	// Alias to avoid infinite recursion
	type msgAlias struct {
		Role       Role       `json:"role"`
		Content    any        `json:"content,omitempty"`
		ToolCallID string     `json:"tool_call_id,omitempty"`
		ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	}

	alias := msgAlias{
		Role:       m.Role,
		ToolCallID: m.ToolCallID,
		ToolCalls:  m.ToolCalls,
	}

	if len(m.ContentParts) > 0 {
		// Multimodal: content is the array of parts
		alias.Content = m.ContentParts
	} else if m.Content != nil {
		// Legacy: content is the string pointer
		alias.Content = m.Content
	}

	return json.Marshal(alias)
}
