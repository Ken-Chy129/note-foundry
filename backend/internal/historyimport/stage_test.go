package historyimport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteStagingWritesManifestMarkdownAndAssets(t *testing.T) {
	output := filepath.Join(t.TempDir(), "staging")
	prepared := PrepareResult{
		Documents: []DocumentCandidate{{
			Source:    SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀", Path: "source.lakebook!java.json"},
			Title:     "Java NIO",
			Directory: []string{"Java 与 JVM", "Java"},
			Markdown:  "# Java NIO\n\n![本地](图片/local.png)\n\n![远程](https://cdn.example.com/remote.png)\n",
			Assets: []AssetCandidate{
				{Reference: "图片/local.png", Name: "local.png", MediaType: "image/png", Data: []byte("local-data")},
				{Reference: "https://cdn.example.com/remote.png", Name: "remote.png", MediaType: "image/png", RemoteURL: "https://cdn.example.com/remote.png"},
			},
		}},
		Duplicates: []DuplicateGroup{{
			Kept:      SourceDescriptor{Kind: SourceLakebook, Path: "source.lakebook!java.json"},
			Discarded: []SourceDescriptor{{Kind: SourceSiyuan, Path: "siyuan.zip!java.md"}},
		}},
	}
	fetcher := fakeAssetFetcher{content: []byte("remote-data"), mediaType: "image/png"}

	manifest, err := WriteStaging(context.Background(), output, prepared, []ScanIssue{{Code: IssueEmptyDocument, SourcePath: "empty.json", Message: "empty"}}, fetcher)
	if err != nil {
		t.Fatalf("WriteStaging() error = %v", err)
	}
	if manifest.Space.Name != "历史文档" || manifest.Space.Visibility != "private" {
		t.Fatalf("WriteStaging() space = %+v", manifest.Space)
	}
	if len(manifest.Documents) != 1 || len(manifest.Documents[0].Attachments) != 2 {
		t.Fatalf("WriteStaging() manifest = %+v", manifest)
	}

	markdownPath := filepath.Join(output, filepath.FromSlash(manifest.Documents[0].MarkdownPath))
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(markdown), "图片/local.png") || strings.Contains(string(markdown), "https://cdn.example.com/remote.png") {
		t.Fatalf("staged markdown still contains source asset references: %q", markdown)
	}
	for _, attachment := range manifest.Documents[0].Attachments {
		if !strings.Contains(string(markdown), "asset:"+attachment.Key) {
			t.Fatalf("staged markdown does not reference attachment %s: %q", attachment.Key, markdown)
		}
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(attachment.Path))); err != nil {
			t.Fatalf("staged attachment %s: %v", attachment.Path, err)
		}
	}

	manifestContent, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded StageManifest
	if err := json.Unmarshal(manifestContent, &decoded); err != nil {
		t.Fatalf("decode manifest.json: %v", err)
	}
	if len(decoded.Duplicates) != 1 || len(decoded.Issues) != 1 {
		t.Fatalf("decoded manifest = %+v", decoded)
	}
	if _, err := os.Stat(filepath.Join(output, "report.md")); err != nil {
		t.Fatalf("report.md: %v", err)
	}
}

func TestWriteStagingRefusesNonEmptyOutputDirectory(t *testing.T) {
	output := t.TempDir()
	if err := os.WriteFile(filepath.Join(output, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := WriteStaging(context.Background(), output, PrepareResult{}, nil, fakeAssetFetcher{})
	if err == nil {
		t.Fatal("WriteStaging() error = nil, want non-empty output error")
	}
	content, readErr := os.ReadFile(filepath.Join(output, "keep.txt"))
	if readErr != nil || string(content) != "keep" {
		t.Fatalf("existing output was changed: %q, %v", content, readErr)
	}
}

func TestValidateRemoteAssetURLRequiresAllowedHTTPSHost(t *testing.T) {
	allowed := map[string]bool{"cdn.nlark.com": true}
	for _, rawURL := range []string{
		"http://cdn.nlark.com/image.png",
		"https://user:password@cdn.nlark.com/image.png",
		"https://example.com/image.png",
	} {
		if _, err := validateRemoteAssetURL(rawURL, allowed); err == nil {
			t.Fatalf("validateRemoteAssetURL(%q) error = nil", rawURL)
		}
	}
	if _, err := validateRemoteAssetURL("https://cdn.nlark.com/image.png", allowed); err != nil {
		t.Fatalf("validateRemoteAssetURL() error = %v", err)
	}
}

func TestHTTPAssetFetcherRejectsNonImageResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain")
		_, _ = response.Write([]byte("not an image"))
	}))
	defer server.Close()

	host := strings.Split(strings.TrimPrefix(server.URL, "https://"), ":")[0]
	fetcher := HTTPAssetFetcher{Client: server.Client(), AllowedHosts: map[string]bool{host: true}}
	if _, _, err := fetcher.Fetch(context.Background(), server.URL+"/file.txt"); err == nil {
		t.Fatal("HTTPAssetFetcher.Fetch() error = nil, want non-image rejection")
	}
}

func TestHTTPAssetFetcherRejectsRedirectToUnallowedHost(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "image/png")
		_, _ = response.Write([]byte("image-data"))
	}))
	defer target.Close()
	targetURL := strings.Replace(target.URL, "127.0.0.1", "localhost", 1)

	source := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, targetURL+"/image.png", http.StatusFound)
	}))
	defer source.Close()

	fetcher := HTTPAssetFetcher{Client: source.Client(), AllowedHosts: map[string]bool{"127.0.0.1": true}}
	if _, _, err := fetcher.Fetch(context.Background(), source.URL+"/redirect"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("HTTPAssetFetcher.Fetch() error = %v, want allowed-host rejection", err)
	}
}

type fakeAssetFetcher struct {
	content   []byte
	mediaType string
	err       error
}

func (fetcher fakeAssetFetcher) Fetch(_ context.Context, _ string) ([]byte, string, error) {
	return fetcher.content, fetcher.mediaType, fetcher.err
}
