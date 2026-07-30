package historyimport

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type SourceKind string

const (
	SourceMarkdown SourceKind = "markdown"
	SourceZip      SourceKind = "zip"
	SourceSiyuan   SourceKind = "siyuan"
	SourceLakebook SourceKind = "lakebook"
)

type IssueCode string

const (
	IssueMissingAsset        IssueCode = "missing_asset"
	IssueEmptyDocument       IssueCode = "empty_document"
	IssueAssetDownloadFailed IssueCode = "asset_download_failed"
	IssueUnsupportedSource   IssueCode = "unsupported_source"
	IssueUnreadableEntry     IssueCode = "unreadable_entry"
)

type SourceDescriptor struct {
	Kind       SourceKind `json:"kind"`
	Collection string     `json:"collection"`
	Path       string     `json:"path"`
}

type AssetCandidate struct {
	Reference string `json:"reference"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Data      []byte `json:"-"`
	RemoteURL string `json:"remoteUrl,omitempty"`
}

type DocumentCandidate struct {
	Source    SourceDescriptor `json:"source"`
	Title     string           `json:"title"`
	Markdown  string           `json:"-"`
	Directory []string         `json:"directory,omitempty"`
	Assets    []AssetCandidate `json:"assets,omitempty"`
}

type ScanIssue struct {
	Code       IssueCode `json:"code"`
	SourcePath string    `json:"sourcePath"`
	Reference  string    `json:"reference,omitempty"`
	Message    string    `json:"message"`
}

const (
	maxDocumentBytes = 32 << 20
	maxAssetBytes    = 128 << 20
)

var (
	firstHeadingPattern = regexp.MustCompile(`(?m)^\s{0,3}#\s+(.+?)\s*$`)
	imageLinkPattern    = regexp.MustCompile(`!\[[^\]]*\]\(<?([^\s)>]+)>?(?:\s+["'][^"']*["'])?\)`)
)

func ScanSourceDirectory(ctx context.Context, sourceDirectory string) ([]DocumentCandidate, []ScanIssue, error) {
	entries, err := os.ReadDir(sourceDirectory)
	if err != nil {
		return nil, nil, fmt.Errorf("read history source directory: %w", err)
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})

	documents := make([]DocumentCandidate, 0)
	issues := make([]ScanIssue, 0)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(sourceDirectory, entry.Name())
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		switch extension {
		case ".md", ".markdown":
			document, entryIssues, readErr := scanMarkdownFile(fullPath)
			if readErr != nil {
				return nil, nil, readErr
			}
			documents = append(documents, document)
			issues = append(issues, entryIssues...)
		case ".zip":
			archiveDocuments, archiveIssues, readErr := scanZipFile(fullPath)
			if readErr != nil {
				return nil, nil, readErr
			}
			documents = append(documents, archiveDocuments...)
			issues = append(issues, archiveIssues...)
		case ".lakebook":
			lakeDocuments, lakeIssues, readErr := scanLakebookFile(fullPath)
			if readErr != nil {
				return nil, nil, readErr
			}
			documents = append(documents, lakeDocuments...)
			issues = append(issues, lakeIssues...)
		}
	}
	return documents, issues, nil
}

func scanMarkdownFile(filename string) (DocumentCandidate, []ScanIssue, error) {
	content, err := readFileLimited(filename, maxDocumentBytes)
	if err != nil {
		return DocumentCandidate{}, nil, err
	}
	markdown := NormalizeMarkdown(string(content))
	document := DocumentCandidate{
		Source: SourceDescriptor{
			Kind:       SourceMarkdown,
			Collection: "独立文档",
			Path:       filename,
		},
		Title:    markdownTitle(markdown, strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))),
		Markdown: markdown,
	}
	assets, issues := scanFilesystemMarkdownAssets(filename, markdown)
	document.Assets = assets
	return document, issues, nil
}

func scanZipFile(filename string) ([]DocumentCandidate, []ScanIssue, error) {
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("open zip archive %s: %w", filename, err)
	}
	defer archive.Close()

	collection := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	if strings.EqualFold(collection, "siyuan") {
		return scanSiyuanArchive(filename, archive.File)
	}
	return scanZipEntries(filename, collection, SourceZip, archive.File)
}

func scanSiyuanArchive(filename string, entries []*zip.File) ([]DocumentCandidate, []ScanIssue, error) {
	documents := make([]DocumentCandidate, 0)
	issues := make([]ScanIssue, 0)
	for _, entry := range entries {
		if !strings.HasSuffix(strings.ToLower(entry.Name), ".zip") || entry.FileInfo().IsDir() {
			continue
		}
		content, err := readZipEntry(entry, maxAssetBytes)
		if err != nil {
			issues = append(issues, unreadableIssue(filename, entry.Name, err))
			continue
		}
		nested, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			issues = append(issues, unreadableIssue(filename, entry.Name, err))
			continue
		}
		collection := strings.TrimSuffix(path.Base(entry.Name), ".zip")
		collection = strings.TrimSuffix(collection, ".md")
		nestedDocuments, nestedIssues, err := scanZipEntries(filename+"!"+entry.Name, collection, SourceSiyuan, nested.File)
		if err != nil {
			return nil, nil, err
		}
		documents = append(documents, nestedDocuments...)
		issues = append(issues, nestedIssues...)
	}
	return documents, issues, nil
}

func scanZipEntries(sourcePath, collection string, kind SourceKind, entries []*zip.File) ([]DocumentCandidate, []ScanIssue, error) {
	entryByName := make(map[string]*zip.File, len(entries))
	for _, entry := range entries {
		entryByName[cleanArchivePath(entry.Name)] = entry
	}

	documents := make([]DocumentCandidate, 0)
	issues := make([]ScanIssue, 0)
	for _, entry := range entries {
		if entry.FileInfo().IsDir() || !isMarkdownPath(entry.Name) {
			continue
		}
		content, err := readZipEntry(entry, maxDocumentBytes)
		if err != nil {
			issues = append(issues, unreadableIssue(sourcePath, entry.Name, err))
			continue
		}
		markdown := NormalizeMarkdown(string(content))
		document := DocumentCandidate{
			Source: SourceDescriptor{
				Kind:       kind,
				Collection: collection,
				Path:       sourcePath + "!" + entry.Name,
			},
			Title:    markdownTitle(markdown, strings.TrimSuffix(path.Base(entry.Name), path.Ext(entry.Name))),
			Markdown: markdown,
		}

		seenAssets := make(map[string]bool)
		for _, match := range imageLinkPattern.FindAllStringSubmatch(markdown, -1) {
			reference := match[1]
			if seenAssets[reference] {
				continue
			}
			seenAssets[reference] = true
			if isHTTPReference(reference) {
				document.Assets = append(document.Assets, remoteAssetCandidate(reference))
				continue
			}
			if isManagedReference(reference) {
				continue
			}
			resolved, resolveErr := resolveArchiveReference(entry.Name, reference)
			if resolveErr != nil {
				issues = append(issues, ScanIssue{Code: IssueMissingAsset, SourcePath: document.Source.Path, Reference: reference, Message: resolveErr.Error()})
				continue
			}
			assetEntry, ok := entryByName[resolved]
			if !ok {
				issues = append(issues, ScanIssue{Code: IssueMissingAsset, SourcePath: document.Source.Path, Reference: reference, Message: "referenced asset is not present in the archive"})
				continue
			}
			assetContent, readErr := readZipEntry(assetEntry, maxAssetBytes)
			if readErr != nil {
				issues = append(issues, unreadableIssue(document.Source.Path, reference, readErr))
				continue
			}
			document.Assets = append(document.Assets, AssetCandidate{
				Reference: reference,
				Name:      path.Base(resolved),
				MediaType: mediaTypeForName(resolved),
				Data:      assetContent,
			})
		}
		documents = append(documents, document)
	}
	return documents, issues, nil
}

func scanFilesystemMarkdownAssets(filename, markdown string) ([]AssetCandidate, []ScanIssue) {
	assets := make([]AssetCandidate, 0)
	issues := make([]ScanIssue, 0)
	seenAssets := make(map[string]bool)
	for _, match := range imageLinkPattern.FindAllStringSubmatch(markdown, -1) {
		reference := match[1]
		if seenAssets[reference] {
			continue
		}
		seenAssets[reference] = true
		if isHTTPReference(reference) {
			assets = append(assets, remoteAssetCandidate(reference))
			continue
		}
		if isManagedReference(reference) {
			continue
		}

		resolved, err := resolveFilesystemReference(filename, reference)
		if err != nil {
			issues = append(issues, ScanIssue{Code: IssueMissingAsset, SourcePath: filename, Reference: reference, Message: err.Error()})
			continue
		}
		content, err := readFileLimited(resolved, maxAssetBytes)
		if err != nil {
			issues = append(issues, ScanIssue{Code: IssueMissingAsset, SourcePath: filename, Reference: reference, Message: err.Error()})
			continue
		}
		assets = append(assets, AssetCandidate{
			Reference: reference,
			Name:      filepath.Base(resolved),
			MediaType: mediaTypeForName(resolved),
			Data:      content,
		})
	}
	return assets, issues
}

func resolveFilesystemReference(markdownFilename, reference string) (string, error) {
	decoded, err := url.PathUnescape(reference)
	if err != nil {
		return "", fmt.Errorf("decode asset reference: %w", err)
	}
	decoded = strings.SplitN(decoded, "#", 2)[0]
	decoded = strings.SplitN(decoded, "?", 2)[0]
	if filepath.IsAbs(decoded) {
		return "", errors.New("asset reference must be relative to the markdown file")
	}
	baseDirectory := filepath.Dir(markdownFilename)
	resolved := filepath.Clean(filepath.Join(baseDirectory, filepath.FromSlash(decoded)))
	relative, err := filepath.Rel(baseDirectory, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("asset reference escapes the markdown directory")
	}
	return resolved, nil
}

func remoteAssetCandidate(reference string) AssetCandidate {
	name := remoteAssetName(reference)
	return AssetCandidate{
		Reference: reference,
		Name:      name,
		MediaType: mediaTypeForName(name),
		RemoteURL: reference,
	}
}

func markdownTitle(markdown, fallback string) string {
	match := firstHeadingPattern.FindStringSubmatch(markdown)
	if len(match) < 2 {
		return strings.TrimSpace(fallback)
	}
	title := strings.TrimSpace(match[1])
	title = strings.Trim(title, "*_`")
	title = strings.ReplaceAll(title, "\\.", ".")
	return strings.TrimSpace(title)
}

func resolveArchiveReference(markdownPath, reference string) (string, error) {
	decoded, err := url.PathUnescape(reference)
	if err != nil {
		return "", fmt.Errorf("decode asset reference: %w", err)
	}
	decoded = strings.SplitN(decoded, "#", 2)[0]
	decoded = strings.SplitN(decoded, "?", 2)[0]
	resolved := cleanArchivePath(path.Join(path.Dir(markdownPath), decoded))
	if resolved == "." || resolved == "" || strings.HasPrefix(resolved, "../") || path.IsAbs(resolved) {
		return "", errors.New("asset reference escapes the archive root")
	}
	return resolved, nil
}

func cleanArchivePath(value string) string {
	return strings.TrimPrefix(path.Clean(strings.ReplaceAll(value, "\\", "/")), "./")
}

func isMarkdownPath(value string) bool {
	extension := strings.ToLower(path.Ext(value))
	return extension == ".md" || extension == ".markdown"
}

func isHTTPReference(reference string) bool {
	lower := strings.ToLower(strings.TrimSpace(reference))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func isManagedReference(reference string) bool {
	lower := strings.ToLower(strings.TrimSpace(reference))
	return strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "attachment:") || strings.HasPrefix(lower, "asset:")
}

func mediaTypeForName(name string) string {
	if mediaType := mime.TypeByExtension(strings.ToLower(path.Ext(name))); mediaType != "" {
		return mediaType
	}
	return "application/octet-stream"
}

func readFileLimited(filename string, limit int64) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filename, err)
	}
	defer file.Close()
	content, err := readLimited(file, limit)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filename, err)
	}
	return content, nil
}

func readZipEntry(entry *zip.File, limit int64) ([]byte, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return readLimited(reader, limit)
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("entry exceeds %d bytes", limit)
	}
	return content, nil
}

func unreadableIssue(sourcePath, reference string, err error) ScanIssue {
	return ScanIssue{
		Code:       IssueUnreadableEntry,
		SourcePath: sourcePath,
		Reference:  reference,
		Message:    err.Error(),
	}
}
