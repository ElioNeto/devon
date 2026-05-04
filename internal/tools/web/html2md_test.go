package web

import (
	"testing"
)

func TestHTMLToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "headings",
			input:    "<h1>Title</h1><h2>Section</h2><h3>Subsection</h3>",
			expected: "# Title\n\n## Section\n\n### Subsection",
		},
		{
			name:     "bold and italic",
			input:    "<strong>bold</strong> and <em>italic</em>",
			expected: "**bold** and *italic*",
		},
		{
			name:     "links",
			input:    `<a href="https://example.com">Example</a>`,
			expected: "[Example](https://example.com)",
		},
		{
			name:     "code inline",
			input:    "Use <code>fmt.Println()</code>",
			expected: "Use `fmt.Println()`",
		},
		{
			name:     "code block",
			input:    "<pre><code>package main\nfunc main() {}</code></pre>",
			expected: "```\npackage main\nfunc main() {}\n```",
		},
		{
			name:     "unordered list",
			input:    "<ul><li>Item 1</li><li>Item 2</li></ul>",
			expected: "- Item 1\n- Item 2",
		},
		{
			name:     "image",
			input:    `<img src="pic.png" alt="A picture" />`,
			expected: "![A picture](pic.png)",
		},
		{
			name:     "paragraphs",
			input:    "<p>First paragraph.</p><p>Second paragraph.</p>",
			expected: "First paragraph.\n\nSecond paragraph.",
		},
		{
			name:     "blockquote",
			input:    "<blockquote>Cited text</blockquote>",
			expected: "> Cited text",
		},
		{
			name:     "horizontal rule",
			input:    "<hr>",
			expected: "---",
		},
		{
			name:     "nested elements",
			input:    "<p>This is <strong>very</strong> important.</p>",
			expected: "This is **very** important.",
		},
		{
			name:     "malformed HTML fallback",
			input:    "<p>Unclosed tag",
			expected: "Unclosed tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HTMLToMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("HTMLToMarkdown() = %q, want %q", result, tt.expected)
			}
		})
	}
}
