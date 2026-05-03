// Package tui — attachment (image) support for multimodal input.
package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Attachment represents an image attached to the user's message.
type Attachment struct {
	Path     string
	Filename string
	Data     []byte // raw file bytes (nilled after send)
	MimeType string
	SizeKB   int
}

// allowedImageExts lists supported image extensions for the file picker.
var allowedImageExts = []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}

// allowedMimeTypes maps file extensions to MIME types.
var allowedMimeTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// validateAttachment checks that the attachment size does not exceed the config limit.
// Returns nil if valid, or an error with a descriptive message.
func (m *appModel) validateAttachment(att Attachment) error {
	maxBytes := m.cfg.MaxImageSizeMB * 1024 * 1024
	if len(att.Data) > maxBytes {
		return fmt.Errorf("imagem muito grande: %d KB (máx %d MB)", att.SizeKB, m.cfg.MaxImageSizeMB)
	}
	return nil
}

// attachFile reads a file, validates it, and appends it to m.attachments.
// Returns an error if the file is too large or unreadable.
func (m *appModel) attachFile(path string) error {
	if path == "" {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	mimeType, ok := allowedMimeTypes[ext]
	if !ok {
		return fmt.Errorf("tipo de arquivo não suportado: %s (permitidos: png, jpg, jpeg, gif, webp)", ext)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("erro ao ler arquivo: %w", err)
	}

	att := Attachment{
		Path:     path,
		Filename: filepath.Base(path),
		Data:     data,
		MimeType: mimeType,
		SizeKB:   len(data) / 1024,
	}

	if err := m.validateAttachment(att); err != nil {
		return err
	}

	m.attachments = append(m.attachments, att)
	return nil
}

// dataURI builds a data URI string from the attachment's raw data.
func (att Attachment) dataURI() string {
	encoded := base64.StdEncoding.EncodeToString(att.Data)
	return fmt.Sprintf("data:%s;base64,%s", att.MimeType, encoded)
}

// attachmentBadge returns a short display string for the attachment badge.
func (att Attachment) attachmentBadge() string {
	return fmt.Sprintf("img: %s %dKB", att.Filename, att.SizeKB)
}
