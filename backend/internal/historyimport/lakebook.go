package historyimport

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

type lakebookMetaEnvelope struct {
	Meta string `json:"meta"`
}

type lakebookMeta struct {
	Book struct {
		TOCYAML string `json:"tocYml"`
		Public  int    `json:"public"`
	} `json:"book"`
	Version string `json:"version"`
}

type lakebookTOCNode struct {
	Type       string `yaml:"type"`
	Title      string `yaml:"title"`
	UUID       string `yaml:"uuid"`
	URL        string `yaml:"url"`
	ParentUUID string `yaml:"parent_uuid"`
	Level      int    `yaml:"level"`
}

type lakebookDocumentEnvelope struct {
	Document lakebookDocument `json:"doc"`
}

type lakebookDocument struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
	Body  string `json:"body"`
}

type lakebookMember struct {
	Name    string
	Content []byte
}

func scanLakebookFile(filename string) ([]DocumentCandidate, []ScanIssue, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("open lakebook %s: %w", filename, err)
	}
	defer file.Close()

	reader := tar.NewReader(file)
	var metaContent []byte
	documentMembers := make([]lakebookMember, 0)
	for {
		header, nextErr := reader.Next()
		if nextErr != nil {
			if nextErr == io.EOF {
				break
			}
			return nil, nil, fmt.Errorf("read lakebook %s: %w", filename, nextErr)
		}
		if !header.FileInfo().Mode().IsRegular() {
			continue
		}
		memberName := cleanArchivePath(header.Name)
		if memberName == "" || memberName == "." || strings.HasPrefix(memberName, "../") || path.IsAbs(memberName) {
			return nil, nil, fmt.Errorf("lakebook %s contains an unsafe member path %q", filename, header.Name)
		}
		content, readErr := readLimited(reader, maxDocumentBytes)
		if readErr != nil {
			return nil, nil, fmt.Errorf("read lakebook member %s: %w", memberName, readErr)
		}
		if path.Base(memberName) == "$meta.json" {
			metaContent = content
			continue
		}
		if strings.EqualFold(path.Ext(memberName), ".json") {
			documentMembers = append(documentMembers, lakebookMember{Name: memberName, Content: content})
		}
	}
	if len(metaContent) == 0 {
		return nil, nil, fmt.Errorf("lakebook %s does not contain $meta.json", filename)
	}

	metadata, toc, err := parseLakebookMeta(metaContent)
	if err != nil {
		return nil, nil, fmt.Errorf("parse lakebook metadata %s: %w", filename, err)
	}
	_ = metadata

	memberBySlug := make(map[string]lakebookMember, len(documentMembers))
	for _, member := range documentMembers {
		slug := strings.TrimSuffix(path.Base(member.Name), path.Ext(member.Name))
		memberBySlug[slug] = member
	}

	collection := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	documents := make([]DocumentCandidate, 0, len(documentMembers))
	issues := make([]ScanIssue, 0)
	consumed := make(map[string]bool, len(documentMembers))
	for _, node := range toc {
		if !strings.EqualFold(node.Type, "DOC") || strings.TrimSpace(node.URL) == "" {
			continue
		}
		member, ok := memberBySlug[node.URL]
		if !ok {
			issues = append(issues, ScanIssue{
				Code:       IssueUnreadableEntry,
				SourcePath: filename,
				Reference:  node.URL,
				Message:    "TOC document has no matching JSON member",
			})
			continue
		}
		document, documentIssues, parseErr := parseLakebookDocument(filename, collection, member, node.Title, lakebookDirectory(node, toc))
		if parseErr != nil {
			issues = append(issues, unreadableIssue(filename, member.Name, parseErr))
			continue
		}
		documents = append(documents, document)
		issues = append(issues, documentIssues...)
		consumed[node.URL] = true
	}

	sort.Slice(documentMembers, func(left, right int) bool { return documentMembers[left].Name < documentMembers[right].Name })
	for _, member := range documentMembers {
		slug := strings.TrimSuffix(path.Base(member.Name), path.Ext(member.Name))
		if consumed[slug] {
			continue
		}
		document, documentIssues, parseErr := parseLakebookDocument(filename, collection, member, "", nil)
		if parseErr != nil {
			issues = append(issues, unreadableIssue(filename, member.Name, parseErr))
			continue
		}
		documents = append(documents, document)
		issues = append(issues, documentIssues...)
	}
	return documents, issues, nil
}

func parseLakebookMeta(content []byte) (lakebookMeta, []lakebookTOCNode, error) {
	var envelope lakebookMetaEnvelope
	if err := json.Unmarshal(content, &envelope); err != nil {
		return lakebookMeta{}, nil, err
	}
	var metadata lakebookMeta
	if err := json.Unmarshal([]byte(envelope.Meta), &metadata); err != nil {
		return lakebookMeta{}, nil, err
	}
	var toc []lakebookTOCNode
	if err := yaml.Unmarshal([]byte(metadata.Book.TOCYAML), &toc); err != nil {
		return lakebookMeta{}, nil, err
	}
	return metadata, toc, nil
}

func parseLakebookDocument(filename, collection string, member lakebookMember, fallbackTitle string, directory []string) (DocumentCandidate, []ScanIssue, error) {
	var envelope lakebookDocumentEnvelope
	if err := json.Unmarshal(member.Content, &envelope); err != nil {
		return DocumentCandidate{}, nil, err
	}
	title := strings.TrimSpace(envelope.Document.Title)
	if title == "" {
		title = strings.TrimSpace(fallbackTitle)
	}
	if title == "" {
		title = strings.TrimSuffix(path.Base(member.Name), path.Ext(member.Name))
	}
	converted, err := LakeHTMLToMarkdown(envelope.Document.Body)
	if err != nil {
		return DocumentCandidate{}, nil, err
	}
	document := DocumentCandidate{
		Source:    SourceDescriptor{Kind: SourceLakebook, Collection: collection, Path: filename + "!" + member.Name},
		Title:     title,
		Markdown:  converted.Markdown,
		Directory: append([]string(nil), directory...),
	}
	seenImages := make(map[string]bool)
	for _, image := range converted.Images {
		if seenImages[image.URL] {
			continue
		}
		seenImages[image.URL] = true
		document.Assets = append(document.Assets, AssetCandidate{
			Reference: image.URL,
			Name:      remoteAssetName(image.URL),
			MediaType: mediaTypeForName(image.URL),
			RemoteURL: image.URL,
		})
	}
	return document, nil, nil
}

func lakebookDirectory(document lakebookTOCNode, toc []lakebookTOCNode) []string {
	nodeByUUID := make(map[string]lakebookTOCNode, len(toc))
	for _, node := range toc {
		if node.UUID != "" {
			nodeByUUID[node.UUID] = node
		}
	}
	pathParts := make([]string, 0)
	visited := make(map[string]bool)
	parentUUID := document.ParentUUID
	for parentUUID != "" && !visited[parentUUID] {
		visited[parentUUID] = true
		parent, ok := nodeByUUID[parentUUID]
		if !ok {
			break
		}
		if strings.EqualFold(parent.Type, "TITLE") && strings.TrimSpace(parent.Title) != "" {
			pathParts = append(pathParts, strings.TrimSpace(parent.Title))
		}
		parentUUID = parent.ParentUUID
	}
	for left, right := 0, len(pathParts)-1; left < right; left, right = left+1, right-1 {
		pathParts[left], pathParts[right] = pathParts[right], pathParts[left]
	}
	return pathParts
}

func remoteAssetName(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		if name := path.Base(parsed.Path); name != "" && name != "." && name != "/" {
			return name
		}
	}
	return "image"
}
