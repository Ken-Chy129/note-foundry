package historyimport

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

type ImageReference struct {
	URL string
	Alt string
}

type LakeMarkdownResult struct {
	Markdown string
	Images   []ImageReference
}

func LakeHTMLToMarkdown(body string) (LakeMarkdownResult, error) {
	if strings.ContainsRune(body, '\x00') {
		return LakeMarkdownResult{}, errors.New("Lake HTML contains a null byte")
	}
	document, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return LakeMarkdownResult{}, fmt.Errorf("parse Lake HTML: %w", err)
	}

	converter := lakeConverter{}
	converter.renderBlockChildren(document)
	markdown := NormalizeMarkdown(strings.Join(converter.blocks, "\n\n"))
	return LakeMarkdownResult{Markdown: markdown, Images: converter.images}, nil
}

type lakeConverter struct {
	blocks []string
	images []ImageReference
}

func (converter *lakeConverter) renderBlockChildren(node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		converter.renderBlock(child)
	}
}

func (converter *lakeConverter) renderBlock(node *html.Node) {
	if node.Type == html.TextNode {
		if text := strings.TrimSpace(collapseWhitespace(node.Data)); text != "" {
			converter.blocks = append(converter.blocks, text)
		}
		return
	}
	if node.Type != html.ElementNode {
		converter.renderBlockChildren(node)
		return
	}

	switch node.Data {
	case "html", "head", "body", "div", "section", "article":
		converter.renderBlockChildren(node)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level, _ := strconv.Atoi(node.Data[1:])
		if text := strings.TrimSpace(converter.renderInlineChildren(node)); text != "" {
			converter.blocks = append(converter.blocks, strings.Repeat("#", level)+" "+text)
		}
	case "p":
		if text := strings.TrimSpace(converter.renderInlineChildren(node)); text != "" {
			converter.blocks = append(converter.blocks, text)
		}
	case "ul", "ol":
		if list := converter.renderList(node, 0); list != "" {
			converter.blocks = append(converter.blocks, list)
		}
	case "pre":
		language := normalizedLanguage(attribute(node, "data-language"))
		code := strings.TrimSuffix(textContent(node), "\n")
		converter.blocks = append(converter.blocks, "```"+language+"\n"+code+"\n```")
	case "table":
		if table := converter.renderTable(node); table != "" {
			converter.blocks = append(converter.blocks, table)
		}
	case "blockquote":
		text := strings.TrimSpace(converter.renderInlineChildren(node))
		if text != "" {
			lines := strings.Split(text, "\n")
			for index := range lines {
				lines[index] = "> " + lines[index]
			}
			converter.blocks = append(converter.blocks, strings.Join(lines, "\n"))
		}
	case "hr":
		converter.blocks = append(converter.blocks, "---")
	case "img":
		if image := converter.renderImage(node); image != "" {
			converter.blocks = append(converter.blocks, image)
		}
	default:
		converter.renderBlockChildren(node)
	}
}

func (converter *lakeConverter) renderInlineChildren(node *html.Node) string {
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(converter.renderInline(child))
	}
	return builder.String()
}

func (converter *lakeConverter) renderInline(node *html.Node) string {
	if node.Type == html.TextNode {
		return collapseWhitespace(node.Data)
	}
	if node.Type != html.ElementNode {
		return converter.renderInlineChildren(node)
	}

	content := converter.renderInlineChildren(node)
	switch node.Data {
	case "strong", "b":
		return "**" + strings.TrimSpace(content) + "**"
	case "em", "i":
		return "*" + strings.TrimSpace(content) + "*"
	case "del", "s":
		return "~~" + strings.TrimSpace(content) + "~~"
	case "code":
		return "`" + strings.TrimSpace(content) + "`"
	case "a":
		href := strings.TrimSpace(attribute(node, "href"))
		if href == "" {
			return content
		}
		return "[" + strings.TrimSpace(content) + "](" + href + ")"
	case "br":
		return "  \n"
	case "img":
		return converter.renderImage(node)
	default:
		return content
	}
}

func (converter *lakeConverter) renderImage(node *html.Node) string {
	source := strings.TrimSpace(attribute(node, "src"))
	if source == "" {
		return ""
	}
	alt := strings.TrimSpace(attribute(node, "alt"))
	converter.images = append(converter.images, ImageReference{URL: source, Alt: alt})
	return "![" + alt + "](" + source + ")"
}

func (converter *lakeConverter) renderList(node *html.Node, depth int) string {
	ordered := node.Data == "ol"
	lines := make([]string, 0)
	itemNumber := 1
	for item := node.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != html.ElementNode || item.Data != "li" {
			continue
		}
		var inline strings.Builder
		nested := make([]*html.Node, 0)
		for child := item.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && (child.Data == "ul" || child.Data == "ol") {
				nested = append(nested, child)
				continue
			}
			inline.WriteString(converter.renderInline(child))
		}
		prefix := "- "
		if ordered {
			prefix = strconv.Itoa(itemNumber) + ". "
		}
		indent := strings.Repeat("   ", depth)
		lines = append(lines, indent+prefix+strings.TrimSpace(inline.String()))
		for _, childList := range nested {
			if child := converter.renderList(childList, depth+1); child != "" {
				lines = append(lines, child)
			}
		}
		itemNumber++
	}
	return strings.Join(lines, "\n")
}

func (converter *lakeConverter) renderTable(node *html.Node) string {
	rows := make([][]string, 0)
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if current.Type == html.ElementNode && current.Data == "tr" {
			cells := make([]string, 0)
			for cell := current.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Type == html.ElementNode && (cell.Data == "td" || cell.Data == "th") {
					value := strings.ReplaceAll(strings.TrimSpace(converter.renderInlineChildren(cell)), "|", "\\|")
					cells = append(cells, value)
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	if len(rows) == 0 {
		return ""
	}

	width := len(rows[0])
	lines := []string{tableRow(rows[0], width), tableRow(repeatValue("---", width), width)}
	for _, row := range rows[1:] {
		lines = append(lines, tableRow(row, width))
	}
	return strings.Join(lines, "\n")
}

func tableRow(cells []string, width int) string {
	values := make([]string, width)
	copy(values, cells)
	return "| " + strings.Join(values, " | ") + " |"
}

func repeatValue(value string, count int) []string {
	values := make([]string, count)
	for index := range values {
		values[index] = value
	}
	return values
}

func normalizedLanguage(language string) string {
	key := strings.ToLower(strings.TrimSpace(language))
	if normalized, ok := fenceLanguages[key]; ok {
		return normalized
	}
	return strings.TrimSpace(language)
}

func attribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func textContent(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(textContent(child))
	}
	return builder.String()
}

func collapseWhitespace(value string) string {
	var builder strings.Builder
	spacePending := false
	for _, character := range value {
		if unicode.IsSpace(character) {
			spacePending = true
			continue
		}
		if spacePending {
			builder.WriteByte(' ')
		}
		spacePending = false
		builder.WriteRune(character)
	}
	if spacePending {
		builder.WriteByte(' ')
	}
	return builder.String()
}
