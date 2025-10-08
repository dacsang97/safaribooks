package html

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dacsang97/safaribooks/internal/models"
	"github.com/dacsang97/safaribooks/pkg/utils"
	nethtml "golang.org/x/net/html"
)

const (
	baseHTMLTemplate = `<!DOCTYPE html>
<html lang="en" xml:lang="en" xmlns="http://www.w3.org/1999/xhtml" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.w3.org/2002/06/xhtml2/ http://www.w3.org/MarkUp/SCHEMA/xhtml2.xsd" xmlns:epub="http://www.idpf.org/2007/ops">
<head>
%s
<style type="text/css">%s</style></head>
<body>%s</body>
</html>`

	baseStyleCSS = `body{margin:1em;background-color:transparent!important;}#sbo-rt-content *{text-indent:0pt!important;}#sbo-rt-content .bq{margin-right:1em!important;}`

	kindleCSS = `#sbo-rt-content *{word-wrap:break-word!important;word-break:break-word!important;}#sbo-rt-content table,#sbo-rt-content pre{overflow-x:unset!important;overflow:unset!important;overflow-y:unset!important;white-space:pre-wrap!important;}`
)

// Parser handles HTML parsing and transformation
type Parser struct {
	bookURL       string
	kindleMode    bool
	baseHTMLStyle string
	cssIndex      map[string]int
	cssList       []string
}

// NewParser creates a new HTML parser
func NewParser(bookURL string, kindleMode bool) *Parser {
	baseStyle := baseStyleCSS
	if !kindleMode {
		baseStyle += kindleCSS
	}

	return &Parser{
		bookURL:       bookURL,
		kindleMode:    kindleMode,
		baseHTMLStyle: baseStyle,
		cssIndex:      make(map[string]int),
		cssList:       []string{},
	}
}

// ParseChapter parses and transforms a chapter's HTML content
func (p *Parser) ParseChapter(chapter models.Chapter, isFirst bool) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(chapter.Content))
	if err != nil {
		return "", "", fmt.Errorf("unable to parse HTML for %s: %w", chapter.Title, err)
	}

	var pageCSS strings.Builder
	pageCSS.Grow(256)

	// Process stylesheets
	if len(chapter.Stylesheets) > 0 || len(chapter.SiteStyles) > 0 {
		for _, sheet := range chapter.Stylesheets {
			if sheet.URL == "" {
				continue
			}
			abs := utils.ResolveURL(chapter.AssetBaseURL, sheet.URL)
			idx := p.ensureCSS(abs)
			pageCSS.WriteString(fmt.Sprintf(`<link href="Styles/Style%02d.css" rel="stylesheet" type="text/css" />`+"\n", idx))
		}

		for _, sheet := range chapter.SiteStyles {
			if sheet == "" {
				continue
			}
			abs := utils.ResolveURL(chapter.AssetBaseURL, sheet)
			idx := p.ensureCSS(abs)
			pageCSS.WriteString(fmt.Sprintf(`<link href="Styles/Style%02d.css" rel="stylesheet" type="text/css" />`+"\n", idx))
		}
	}

	// Process link tags in the document
	doc.Find("link[rel='stylesheet']").Each(func(_ int, sel *goquery.Selection) {
		if href, ok := sel.Attr("href"); ok {
			abs := utils.ResolveURL(p.bookURL, href)
			idx := p.ensureCSS(abs)
			pageCSS.WriteString(fmt.Sprintf(`<link href="Styles/Style%02d.css" rel="stylesheet" type="text/css" />`+"\n", idx))
			sel.Remove()
		}
	})

	// Process style tags
	doc.Find("style").Each(func(_ int, sel *goquery.Selection) {
		node := sel.Get(0)
		if node == nil {
			return
		}
		templateIdx := -1
		for i, attr := range node.Attr {
			if attr.Key == "data-template" {
				clearChildren(node)
				node.Attr = append(node.Attr[:i], node.Attr[i+1:]...)
				node.AppendChild(&nethtml.Node{Type: nethtml.TextNode, Data: attr.Val})
				templateIdx = i
				break
			}
		}
		if templateIdx >= 0 {
			// attribute already removed in branch above
		}
		if css, err := nodeToString(node); err == nil {
			pageCSS.WriteString(css)
			pageCSS.WriteString("\n")
		}
	})

	// Process image tags
	doc.Find("image").Each(func(_ int, sel *goquery.Selection) {
		node := sel.Get(0)
		if node == nil || node.Parent == nil || node.Parent.Parent == nil {
			return
		}
		var src string
		for _, attr := range node.Attr {
			if strings.Contains(strings.ToLower(attr.Key), "href") {
				src = attr.Val
				break
			}
		}
		if src == "" {
			return
		}

		img := &nethtml.Node{
			Type: nethtml.ElementNode,
			Data: "img",
			Attr: []nethtml.Attribute{{Key: "src", Val: src}},
		}

		parent := node.Parent
		grand := parent.Parent
		grand.InsertBefore(img, parent)
		grand.RemoveChild(parent)
	})

	// Find the main content
	bookContent := doc.Find("div#sbo-rt-content")
	if bookContent.Length() == 0 {
		return "", "", fmt.Errorf("parser: book content missing for %s", chapter.Title)
	}

	contentNode := bookContent.Get(0)
	rewriteLinks(contentNode, p.linkReplace)

	// Convert to XHTML
	xhtml, err := nodeToXHTML(contentNode)
	if err != nil {
		return "", "", fmt.Errorf("parser: unable to serialize chapter %s: %w", chapter.Title, err)
	}

	// Generate the final HTML
	pageHTML := fmt.Sprintf(baseHTMLTemplate, pageCSS.String(), p.baseHTMLStyle, xhtml)

	return pageCSS.String(), pageHTML, nil
}

// ensureCSS adds a CSS URL to the list if not already present
func (p *Parser) ensureCSS(url string) int {
	if url == "" {
		return 0
	}
	if idx, ok := p.cssIndex[url]; ok {
		return idx
	}
	idx := len(p.cssList)
	p.cssIndex[url] = idx
	p.cssList = append(p.cssList, url)
	return idx
}

// linkReplace replaces links with local equivalents
func (p *Parser) linkReplace(link string) string {
	link = strings.TrimSpace(link)
	if link == "" || strings.HasPrefix(link, "mailto:") {
		return link
	}

	if !utils.IsAbsoluteURL(link) {
		lower := strings.ToLower(link)
		if strings.Contains(lower, "cover") || strings.Contains(lower, "images") || strings.Contains(lower, "graphics") || isImageLink(link) {
			name := utils.BaseName(link)
			if name == "" {
				name = utils.FilenameFromURL(link)
			}
			if name == "" {
				return link
			}
			return "Images/" + name
		}
		return strings.ReplaceAll(link, ".html", ".xhtml")
	}

	return link
}

// rewriteLinks rewrites all links in a node
func rewriteLinks(node *nethtml.Node, repl func(string) string) {
	if node.Type == nethtml.ElementNode {
		for i := range node.Attr {
			attr := &node.Attr[i]
			switch attr.Key {
			case "href", "src", "data", "poster":
				attr.Val = repl(attr.Val)
			case "srcset":
				attr.Val = rewriteSrcset(attr.Val, repl)
			}
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		rewriteLinks(child, repl)
	}
}

// rewriteSrcset rewrites srcset attribute values
func rewriteSrcset(value string, repl func(string) string) string {
	parts := strings.Split(value, ",")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments := strings.Fields(part)
		if len(segments) == 0 {
			continue
		}
		segments[0] = repl(segments[0])
		parts[i] = strings.Join(segments, " ")
	}
	return strings.Join(parts, ", ")
}

// nodeToString converts a node to a string
func nodeToString(n *nethtml.Node) (string, error) {
	var buf bytes.Buffer
	if err := nethtml.Render(&buf, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// nodeToXHTML converts a node to XHTML
func nodeToXHTML(node *nethtml.Node) (string, error) {
	var buf bytes.Buffer
	if err := renderXHTML(&buf, node); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderXHTML renders a node as XHTML
func renderXHTML(buf *bytes.Buffer, node *nethtml.Node) error {
	switch node.Type {
	case nethtml.DocumentNode:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := renderXHTML(buf, child); err != nil {
				return err
			}
		}
	case nethtml.ElementNode:
		buf.WriteByte('<')
		buf.WriteString(node.Data)
		for _, attr := range node.Attr {
			buf.WriteByte(' ')
			buf.WriteString(attr.Key)
			buf.WriteByte('=')
			buf.WriteByte('"')
			buf.WriteString(html.EscapeString(attr.Val))
			buf.WriteByte('"')
		}
		if node.FirstChild == nil {
			buf.WriteString("/>")
			return nil
		}
		buf.WriteByte('>')
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := renderXHTML(buf, child); err != nil {
				return err
			}
		}
		buf.WriteString("</")
		buf.WriteString(node.Data)
		buf.WriteByte('>')
	case nethtml.TextNode:
		buf.WriteString(html.EscapeString(node.Data))
	case nethtml.CommentNode:
		buf.WriteString("<!--")
		buf.WriteString(node.Data)
		buf.WriteString("-->")
	case nethtml.DoctypeNode:
		// ignored inside body
	case nethtml.RawNode:
		buf.WriteString(node.Data)
	}
	return nil
}

// clearChildren removes all children from a node
func clearChildren(node *nethtml.Node) {
	for node.FirstChild != nil {
		node.RemoveChild(node.FirstChild)
	}
}

// isImageLink checks if a link is an image
func isImageLink(raw string) bool {
	raw = strings.ToLower(raw)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"} {
		if strings.HasSuffix(raw, ext) {
			return true
		}
	}
	return false
}
