package sources

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHTTPURLExtractorReadsUsefulHTMLText(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       io.NopCloser(strings.NewReader(`<!doctype html><html><head><title>Planner Guide</title><style>hidden</style></head><body><nav>menu</nav><main><h1>Query plans</h1><p>Use EXPLAIN ANALYZE.</p><script>ignore()</script></main></body></html>`)),
			Request:    request,
		}, nil
	})}
	extractor := &HTTPURLExtractor{client: client, maxBytes: 1024 * 1024}

	document, err := extractor.Extract(context.Background(), "https://example.com/guide#top")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if document.Title != "Planner Guide" || document.Content != "Query plans\nUse EXPLAIN ANALYZE." {
		t.Fatalf("extracted document = %+v", document)
	}
}

func TestHTTPURLExtractorRejectsPrivateAndReservedAddresses(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "192.168.1.1", "100.64.0.1", "::1", "fc00::1", "2001:db8::1"} {
		if !isBlockedSourceIP(net.ParseIP(raw)) {
			t.Errorf("isBlockedSourceIP(%q) = false", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if isBlockedSourceIP(net.ParseIP(raw)) {
			t.Errorf("isBlockedSourceIP(%q) = true", raw)
		}
	}
}

func TestHTTPURLExtractorBlocksPrivateDestinationBeforeConnecting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := NewHTTPURLExtractor().Extract(ctx, "http://127.0.0.1/internal"); err == nil || !strings.Contains(err.Error(), "not publicly routable") {
		t.Fatalf("Extract(private URL) error = %v", err)
	}
}

func TestHTTPURLExtractorFetchesPublicPage(t *testing.T) {
	if os.Getenv("RUN_NETWORK_TESTS") != "1" {
		t.Skip("RUN_NETWORK_TESTS is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	document, err := NewHTTPURLExtractor().Extract(ctx, "https://example.com/")
	if err != nil {
		t.Fatalf("Extract(public URL) error = %v", err)
	}
	if document.Title == "" || !strings.Contains(document.Content, "Example Domain") {
		t.Fatalf("public document = %+v", document)
	}
}

func TestHTTPURLExtractorRejectsUnsupportedAndOversizedResponses(t *testing.T) {
	for name, response := range map[string]*http.Response{
		"unsupported": {
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			Body:       io.NopCloser(strings.NewReader("pdf")),
		},
		"oversized": {
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
			Body:       io.NopCloser(strings.NewReader("12345")),
		},
	} {
		t.Run(name, func(t *testing.T) {
			response.Request = httptest.NewRequest(http.MethodGet, "https://example.com", nil)
			extractor := &HTTPURLExtractor{
				client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return response, nil
				})},
				maxBytes: 4,
			}
			if _, err := extractor.Extract(context.Background(), "https://example.com"); err == nil {
				t.Fatal("Extract() error = nil")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
