package web

import (
	"strings"

	"golang.org/x/net/html"
)

// HTMLToMarkdown converts HTML content to simplified Markdown.
// Uses Go's html tokenizer for robust streaming conversion.
func HTMLToMarkdown(htmlContent string) string {
	z := html.NewTokenizer(strings.NewReader(htmlContent))
	var sb strings.Builder

	// Track formatting context: stack of open tags
	var openTags []string
	links := make(map[int]string) // stack index → href
	inPre := false

	write := func(s string) {
		sb.WriteString(s)
	}

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}

		switch tt {
		case html.TextToken:
			text := string(z.Text())
			if inPre {
				write(text)
				continue
			}

			// Check if we're inside an anchor
			inAnchor := false
			for _, t := range openTags {
				if t == "a" {
					inAnchor = true
					break
				}
			}

			if inAnchor {
				write(strings.TrimSpace(text))
			} else {
				write(text)
			}

		case html.StartTagToken:
			name, hasAttr := z.TagName()
			tag := string(name)
			openTags = append(openTags, tag)

			switch tag {
			case "pre":
				inPre = true
				write("\n```\n")
			case "code":
				if !inPre {
					write("`")
				}
			case "strong", "b":
				write("**")
			case "em", "i":
				write("*")
			case "h1", "h2", "h3", "h4", "h5", "h6":
				write("\n" + strings.Repeat("#", int(tag[1]-'0')) + " ")
			case "p":
				write("\n\n")
			case "br":
				write("\n")
			case "hr":
				write("\n---\n")
			case "blockquote":
				write("\n> ")
			case "li":
				// Determine list type by scanning open tags
				isOL := false
				for i := len(openTags) - 2; i >= 0; i-- {
					if openTags[i] == "ol" {
						isOL = true
						break
					} else if openTags[i] == "ul" {
						break
					}
				}
				if isOL {
					write("1. ")
				} else {
					write("- ")
				}
			case "ul", "ol":
				write("\n")
			case "div", "section", "article", "header", "footer", "nav", "main":
				// block-level spacing
			case "a":
				if hasAttr {
					links[len(openTags)-1] = getAttrOnce(z)
				}
				write("[")
			case "img":
				if hasAttr {
					alt, src := getImgAttrs(z)
					if alt != "" && src != "" {
						write("![" + alt + "](" + src + ")")
					}
				}
			default:
				// ignore unknown tags
			}

		case html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			tag := string(name)

			switch tag {
			case "br":
				write("\n")
			case "hr":
				write("\n---\n")
			case "img":
				if hasAttr {
					alt, src := getImgAttrs(z)
					if alt != "" && src != "" {
						write("![" + alt + "](" + src + ")")
					}
				}
			}

		case html.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)

			// Pop from open tags
			popIdx := -1
			for i := len(openTags) - 1; i >= 0; i-- {
				if openTags[i] == tag {
					popIdx = i
					break
				}
			}

			switch tag {
			case "pre":
				inPre = false
				write("\n```\n")
			case "code":
				if !inPre {
					write("`")
				}
			case "strong", "b":
				write("**")
			case "em", "i":
				write("*")
			case "a":
				href := ""
				if popIdx >= 0 {
					href = links[popIdx]
				}
				if href != "" {
					write("](" + href + ")")
				} else {
					write("]()")
				}
			case "p":
				write("\n")
			case "h1", "h2", "h3", "h4", "h5", "h6", "li", "blockquote":
				write("\n")
			case "ul", "ol":
				write("\n")
			}

			if popIdx >= 0 {
				delete(links, popIdx)
				openTags = append(openTags[:popIdx], openTags[popIdx+1:]...)
			}
		}
	}

	result := sb.String()
	// Normalize whitespace: collapse 3+ consecutive newlines to 2
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	result = strings.TrimSpace(result)
	return result
}

// getAttrOnce reads all attributes and returns the href value.
// This is used for <a> tags where we only need the href.
func getAttrOnce(z *html.Tokenizer) string {
	href := ""
	for {
		attrKey, attrVal, more := z.TagAttr()
		if string(attrKey) == "href" {
			href = string(attrVal)
		}
		if !more {
			break
		}
	}
	return href
}

// getImgAttrs reads all attributes and returns alt and src.
func getImgAttrs(z *html.Tokenizer) (alt, src string) {
	for {
		attrKey, attrVal, more := z.TagAttr()
		key := string(attrKey)
		val := string(attrVal)
		switch key {
		case "alt":
			alt = val
		case "src":
			src = val
		}
		if !more {
			break
		}
	}
	return
}
