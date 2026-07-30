package historyimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	stageManifestVersion = 1
	maxRemoteAssetBytes  = 32 << 20
)

type AssetFetcher interface {
	Fetch(context.Context, string) ([]byte, string, error)
}

type HTTPAssetFetcher struct {
	Client       *http.Client
	AllowedHosts map[string]bool
}

type StageSpace struct {
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
}

type StageAttachment struct {
	Key       string `json:"key"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	SHA256Hex string `json:"sha256Hex"`
	Source    string `json:"source"`
}

type StageDocument struct {
	Key          string            `json:"key"`
	Title        string            `json:"title"`
	Directory    []string          `json:"directory"`
	MarkdownPath string            `json:"markdownPath"`
	Source       SourceDescriptor  `json:"source"`
	Attachments  []StageAttachment `json:"attachments"`
}

type StageManifest struct {
	Version     int              `json:"version"`
	GeneratedAt time.Time        `json:"generatedAt"`
	Space       StageSpace       `json:"space"`
	Documents   []StageDocument  `json:"documents"`
	Duplicates  []DuplicateGroup `json:"duplicates"`
	Issues      []ScanIssue      `json:"issues"`
}

func WriteStaging(ctx context.Context, outputDirectory string, prepared PrepareResult, scanIssues []ScanIssue, fetcher AssetFetcher) (StageManifest, error) {
	if err := prepareOutputDirectory(outputDirectory); err != nil {
		return StageManifest{}, err
	}

	manifest := StageManifest{
		Version:     stageManifestVersion,
		GeneratedAt: time.Now().UTC(),
		Space:       StageSpace{Name: "历史文档", Visibility: "private"},
		Duplicates:  prepared.Duplicates,
		Issues:      filteredIssues(prepared, scanIssues),
	}

	for _, document := range prepared.Documents {
		if err := ctx.Err(); err != nil {
			return StageManifest{}, err
		}
		staged, documentIssues, err := writeStagedDocument(ctx, outputDirectory, document, fetcher)
		if err != nil {
			return StageManifest{}, err
		}
		manifest.Documents = append(manifest.Documents, staged)
		manifest.Issues = append(manifest.Issues, documentIssues...)
	}

	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return StageManifest{}, fmt.Errorf("encode staging manifest: %w", err)
	}
	manifestJSON = append(manifestJSON, '\n')
	if err := writeStageFile(outputDirectory, "manifest.json", manifestJSON); err != nil {
		return StageManifest{}, err
	}
	if err := writeStageFile(outputDirectory, "report.md", []byte(renderStageReport(manifest))); err != nil {
		return StageManifest{}, err
	}
	return manifest, nil
}

func filteredIssues(prepared PrepareResult, scanIssues []ScanIssue) []ScanIssue {
	discardedPaths := make(map[string]bool)
	for _, duplicate := range prepared.Duplicates {
		for _, discarded := range duplicate.Discarded {
			discardedPaths[discarded.Path] = true
		}
	}
	combined := append(append([]ScanIssue(nil), scanIssues...), prepared.Issues...)
	result := make([]ScanIssue, 0, len(combined))
	seen := make(map[string]bool)
	for _, issue := range combined {
		if discardedPaths[issue.SourcePath] {
			continue
		}
		key := string(issue.Code) + "\x00" + issue.SourcePath + "\x00" + issue.Reference + "\x00" + issue.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, issue)
	}
	return result
}

func (fetcher HTTPAssetFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, string, error) {
	parsed, err := validateRemoteAssetURL(rawURL, fetcher.AllowedHosts)
	if err != nil {
		return nil, "", err
	}
	client := fetcher.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	clientCopy := *client
	previousRedirectCheck := client.CheckRedirect
	clientCopy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if _, err := validateRemoteAssetURL(request.URL.String(), fetcher.AllowedHosts); err != nil {
			return err
		}
		if previousRedirectCheck != nil {
			return previousRedirectCheck(request, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("create asset request: %w", err)
	}
	response, err := clientCopy.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("download asset: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download asset: unexpected HTTP status %d", response.StatusCode)
	}
	content, err := readLimited(response.Body, maxRemoteAssetBytes)
	if err != nil {
		return nil, "", fmt.Errorf("download asset: %w", err)
	}
	mediaType := strings.TrimSpace(strings.SplitN(response.Header.Get("Content-Type"), ";", 2)[0])
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = http.DetectContentType(content)
	}
	if !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return nil, "", fmt.Errorf("download asset: content type %q is not an image", mediaType)
	}
	return content, mediaType, nil
}

func writeStagedDocument(ctx context.Context, outputDirectory string, document DocumentCandidate, fetcher AssetFetcher) (StageDocument, []ScanIssue, error) {
	documentKey := shortHash(document.Source.Path)
	components := make([]string, 0, len(document.Directory)+2)
	components = append(components, "notes")
	for _, component := range document.Directory {
		components = append(components, safePathComponent(component, "未分类"))
	}
	filename := safePathComponent(document.Title, "无标题") + "--" + documentKey + ".md"
	components = append(components, filename)
	markdownPath := path.Join(components...)
	markdown := document.Markdown
	issues := make([]ScanIssue, 0)
	attachments := make([]StageAttachment, 0, len(document.Assets))

	for _, asset := range document.Assets {
		if err := ctx.Err(); err != nil {
			return StageDocument{}, nil, err
		}
		content := asset.Data
		mediaType := asset.MediaType
		if asset.RemoteURL != "" {
			if fetcher == nil {
				issues = append(issues, ScanIssue{Code: IssueAssetDownloadFailed, SourcePath: document.Source.Path, Reference: asset.RemoteURL, Message: "no remote asset fetcher is configured"})
				continue
			}
			fetched, fetchedMediaType, err := fetcher.Fetch(ctx, asset.RemoteURL)
			if err != nil {
				issues = append(issues, ScanIssue{Code: IssueAssetDownloadFailed, SourcePath: document.Source.Path, Reference: asset.RemoteURL, Message: err.Error()})
				continue
			}
			content = fetched
			if fetchedMediaType != "" {
				mediaType = fetchedMediaType
			}
		}
		if len(content) == 0 {
			issues = append(issues, ScanIssue{Code: IssueAssetDownloadFailed, SourcePath: document.Source.Path, Reference: asset.Reference, Message: "asset content is empty"})
			continue
		}

		assetKey := shortHash(documentKey + "\x00" + asset.Reference)
		assetName := safePathComponent(asset.Name, "attachment")
		attachmentPath := path.Join("attachments", documentKey, assetKey+"-"+assetName)
		if err := writeStageFile(outputDirectory, attachmentPath, content); err != nil {
			return StageDocument{}, nil, err
		}
		checksum := sha256.Sum256(content)
		attachments = append(attachments, StageAttachment{
			Key:       assetKey,
			Path:      attachmentPath,
			Name:      assetName,
			MediaType: mediaType,
			SHA256Hex: hex.EncodeToString(checksum[:]),
			Source:    asset.Reference,
		})
		markdown = rewriteImageReference(markdown, asset.Reference, "asset:"+assetKey)
	}

	if err := writeStageFile(outputDirectory, markdownPath, []byte(NormalizeMarkdown(markdown))); err != nil {
		return StageDocument{}, nil, err
	}
	return StageDocument{
		Key:          documentKey,
		Title:        document.Title,
		Directory:    append([]string(nil), document.Directory...),
		MarkdownPath: markdownPath,
		Source:       document.Source,
		Attachments:  attachments,
	}, issues, nil
}

func validateRemoteAssetURL(rawURL string, allowedHosts map[string]bool) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse asset URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return nil, errors.New("asset URL must be an HTTPS URL without credentials")
	}
	if len(allowedHosts) > 0 && !allowedHosts[strings.ToLower(parsed.Hostname())] {
		return nil, fmt.Errorf("asset host %q is not allowed", parsed.Hostname())
	}
	return parsed, nil
}

func prepareOutputDirectory(outputDirectory string) error {
	info, err := os.Stat(outputDirectory)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(outputDirectory, 0o700); err != nil {
			return fmt.Errorf("create staging output directory: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect staging output directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("staging output path exists and is not a directory")
	}
	entries, err := os.ReadDir(outputDirectory)
	if err != nil {
		return fmt.Errorf("read staging output directory: %w", err)
	}
	if len(entries) != 0 {
		return errors.New("staging output directory must be empty")
	}
	return nil
}

func writeStageFile(outputDirectory, relativePath string, content []byte) error {
	destination, err := safeStagePath(outputDirectory, relativePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	if err := os.WriteFile(destination, content, 0o600); err != nil {
		return fmt.Errorf("write staging file %s: %w", relativePath, err)
	}
	return nil
}

func safeStagePath(outputDirectory, relativePath string) (string, error) {
	if filepath.IsAbs(relativePath) {
		return "", errors.New("staging file path must be relative")
	}
	destination := filepath.Join(outputDirectory, filepath.FromSlash(relativePath))
	relative, err := filepath.Rel(outputDirectory, destination)
	if err != nil {
		return "", fmt.Errorf("resolve staging file path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("staging file path escapes the output directory")
	}
	return destination, nil
}

func safePathComponent(value, fallback string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, character := range value {
		switch {
		case character == '/' || character == '\\' || character == ':' || unicode.IsControl(character):
			builder.WriteRune('-')
		default:
			builder.WriteRune(character)
		}
	}
	result := strings.Trim(builder.String(), " .")
	if result == "" || result == "." || result == ".." {
		return fallback
	}
	return result
}

func rewriteImageReference(markdown, oldReference, newReference string) string {
	matches := imageLinkPattern.FindAllStringSubmatchIndex(markdown, -1)
	if len(matches) == 0 {
		return markdown
	}
	var builder strings.Builder
	last := 0
	for _, match := range matches {
		if len(match) < 4 || match[2] < 0 || match[3] < 0 || markdown[match[2]:match[3]] != oldReference {
			continue
		}
		builder.WriteString(markdown[last:match[2]])
		builder.WriteString(newReference)
		last = match[3]
	}
	if last == 0 {
		return markdown
	}
	builder.WriteString(markdown[last:])
	return builder.String()
}

func shortHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:8])
}

func renderStageReport(manifest StageManifest) string {
	directoryCounts := make(map[string]int)
	for _, document := range manifest.Documents {
		root := "待整理"
		if len(document.Directory) > 0 {
			root = document.Directory[0]
		}
		directoryCounts[root]++
	}
	directories := make([]string, 0, len(directoryCounts))
	for directory := range directoryCounts {
		directories = append(directories, directory)
	}
	sort.Strings(directories)

	var report strings.Builder
	report.WriteString("# 历史文档迁移报告\n\n")
	report.WriteString(fmt.Sprintf("- 待导入学习笔记：%d\n", len(manifest.Documents)))
	report.WriteString(fmt.Sprintf("- 合并的重复组：%d\n", len(manifest.Duplicates)))
	report.WriteString(fmt.Sprintf("- 待处理问题：%d\n", len(manifest.Issues)))
	report.WriteString("- 目标知识空间：历史文档（private）\n\n")
	report.WriteString("## 主题目录\n\n")
	for _, directory := range directories {
		report.WriteString(fmt.Sprintf("- %s：%d 篇\n", directory, directoryCounts[directory]))
	}
	if len(manifest.Issues) > 0 {
		type issueSummary struct {
			Code       IssueCode
			SourcePath string
			Count      int
		}
		countsByCode := make(map[IssueCode]int)
		sourcesByCode := make(map[IssueCode]map[string]bool)
		countsBySource := make(map[string]int)
		for _, issue := range manifest.Issues {
			countsByCode[issue.Code]++
			if sourcesByCode[issue.Code] == nil {
				sourcesByCode[issue.Code] = make(map[string]bool)
			}
			sourcesByCode[issue.Code][issue.SourcePath] = true
			countsBySource[string(issue.Code)+"\x00"+issue.SourcePath]++
		}
		codes := make([]IssueCode, 0, len(countsByCode))
		for code := range countsByCode {
			codes = append(codes, code)
		}
		sort.Slice(codes, func(left, right int) bool { return codes[left] < codes[right] })

		report.WriteString("\n## 问题摘要\n\n")
		for _, code := range codes {
			report.WriteString(fmt.Sprintf("- `%s`：%d 项，影响 %d 篇文档\n", code, countsByCode[code], len(sourcesByCode[code])))
		}
		report.WriteString("\n## 受影响文档\n\n")
		summaries := make([]issueSummary, 0, len(countsBySource))
		for key, count := range countsBySource {
			parts := strings.SplitN(key, "\x00", 2)
			summaries = append(summaries, issueSummary{Code: IssueCode(parts[0]), SourcePath: parts[1], Count: count})
		}
		sort.Slice(summaries, func(left, right int) bool {
			if summaries[left].Code == summaries[right].Code {
				return summaries[left].SourcePath < summaries[right].SourcePath
			}
			return summaries[left].Code < summaries[right].Code
		})
		for _, summary := range summaries {
			report.WriteString(fmt.Sprintf("- `%s` %s：%d 项\n", summary.Code, summary.SourcePath, summary.Count))
		}
		report.WriteString("\n完整逐项明细见 `manifest.json`。\n")
	}
	return report.String()
}
