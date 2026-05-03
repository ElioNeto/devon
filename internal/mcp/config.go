// Package mcp implements Model Context Protocol client and transport layers.
package mcp

import "github.com/ElioNeto/devon/internal/config"

// ToTransportConfig converts config.MCPServerConfig to TransportConfig.
func ToTransportConfig(c config.MCPServerConfig) TransportConfig {
	return TransportConfig{
		Type:    c.Type,
		Command: c.Command,
		Args:    c.Args,
		Env:     c.Env,
		URL:     c.URL,
		Headers: c.Headers,
	}
}
