package sources

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const defaultURLExtractionLimit = 4 << 20

var blockedSourcePrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
	netip.MustParsePrefix("2001:db8::/32"),
}

type HTTPURLExtractor struct {
	client   *http.Client
	maxBytes int64
}

func NewHTTPURLExtractor() *HTTPURLExtractor {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           safeSourceDialContext(dialer, net.DefaultResolver),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   25 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			_, err := NormalizeSourceURL(request.URL.String())
			return err
		},
	}
	return &HTTPURLExtractor{client: client, maxBytes: defaultURLExtractionLimit}
}

func (extractor *HTTPURLExtractor) Extract(ctx context.Context, rawURL string) (ExtractedURLDocument, error) {
	normalizedURL, err := NormalizeSourceURL(rawURL)
	if err != nil {
		return ExtractedURLDocument{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL, nil)
	if err != nil {
		return ExtractedURLDocument{}, fmt.Errorf("build URL extraction request: %w", err)
	}
	request.Header.Set("Accept", "text/html, application/xhtml+xml, text/plain;q=0.8")
	request.Header.Set("User-Agent", "NoteFoundry/0.2 (+URL Learning Source extraction)")

	response, err := extractor.client.Do(request)
	if err != nil {
		return ExtractedURLDocument{}, fmt.Errorf("fetch URL: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ExtractedURLDocument{}, fmt.Errorf("fetch URL: unexpected HTTP status %d", response.StatusCode)
	}

	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		return ExtractedURLDocument{}, errors.New("fetch URL: invalid Content-Type")
	}
	if mediaType != "text/html" && mediaType != "application/xhtml+xml" && mediaType != "text/plain" {
		return ExtractedURLDocument{}, fmt.Errorf("fetch URL: unsupported Content-Type %q", mediaType)
	}

	limit := extractor.maxBytes
	if limit <= 0 {
		limit = defaultURLExtractionLimit
	}
	rawBody, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return ExtractedURLDocument{}, fmt.Errorf("read URL content: %w", err)
	}
	if int64(len(rawBody)) > limit {
		return ExtractedURLDocument{}, fmt.Errorf("fetch URL: response exceeds %d bytes", limit)
	}
	decodedBody, err := charset.NewReader(bytes.NewReader(rawBody), response.Header.Get("Content-Type"))
	if err != nil {
		return ExtractedURLDocument{}, fmt.Errorf("decode URL content: %w", err)
	}
	body, err := io.ReadAll(decodedBody)
	if err != nil {
		return ExtractedURLDocument{}, fmt.Errorf("read URL content: %w", err)
	}
	if mediaType == "text/plain" {
		content := strings.TrimSpace(string(body))
		if content == "" {
			return ExtractedURLDocument{}, errors.New("extract text: no readable content found")
		}
		return ExtractedURLDocument{Content: content}, nil
	}
	return extractHTMLDocument(body)
}

func safeSourceDialContext(dialer *net.Dialer, resolver *net.Resolver) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("validate URL destination: %w", err)
		}
		addresses, err := resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve URL destination: %w", err)
		}
		if len(addresses) == 0 {
			return nil, errors.New("resolve URL destination: no addresses")
		}
		for _, address := range addresses {
			if isBlockedSourceIP(address.IP) {
				return nil, errors.New("URL destination is not publicly routable")
			}
		}
		var lastErr error
		for _, address := range addresses {
			connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
			if err == nil {
				return connection, nil
			}
			lastErr = err
		}
		return nil, fmt.Errorf("connect to URL destination: %w", lastErr)
	}
}

func isBlockedSourceIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	address = address.Unmap()
	for _, prefix := range blockedSourcePrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return !address.IsGlobalUnicast()
}

func extractHTMLDocument(body []byte) (ExtractedURLDocument, error) {
	document, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return ExtractedURLDocument{}, fmt.Errorf("parse HTML: %w", err)
	}
	title := strings.Join(strings.Fields(textContent(firstElement(document, "title"))), " ")
	for _, element := range []string{"main", "article", "body"} {
		if content := readableText(firstElement(document, element)); content != "" {
			return ExtractedURLDocument{Title: title, Content: content}, nil
		}
	}
	return ExtractedURLDocument{}, errors.New("extract HTML: no readable content found")
}

func firstElement(node *html.Node, name string) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && node.Data == name {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if match := firstElement(child, name); match != nil {
			return match
		}
	}
	return nil
}

func textContent(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return node.Data
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(textContent(child))
	}
	return builder.String()
}

func readableText(root *html.Node) string {
	var builder strings.Builder
	var lastByte byte
	writeNewline := func() {
		if builder.Len() > 0 && lastByte != '\n' {
			builder.WriteByte('\n')
			lastByte = '\n'
		}
	}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode {
			switch node.Data {
			case "script", "style", "noscript", "svg", "nav", "form":
				return
			}
			if isReadableBlock(node.Data) {
				writeNewline()
			}
		}
		if node.Type == html.TextNode {
			text := strings.Join(strings.Fields(node.Data), " ")
			if text != "" {
				if builder.Len() > 0 && lastByte != '\n' && lastByte != ' ' {
					builder.WriteByte(' ')
				}
				builder.WriteString(text)
				lastByte = text[len(text)-1]
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if node.Type == html.ElementNode && isReadableBlock(node.Data) {
			writeNewline()
		}
	}
	walk(root)

	lines := strings.Split(builder.String(), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

func isReadableBlock(name string) bool {
	switch name {
	case "address", "article", "aside", "blockquote", "div", "dl", "dt", "dd", "figcaption", "figure", "footer", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "ol", "p", "pre", "section", "table", "tr", "ul":
		return true
	default:
		return false
	}
}
