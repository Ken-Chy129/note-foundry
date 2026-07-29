package historyimport

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanSourceDirectoryReadsMarkdownAndZipExports(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "独立文档.md"), []byte("# 独立文档\n\n正文\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	writeZipFile(t, filepath.Join(source, "Linux命令.zip"), map[string][]byte{
		"Linux命令.md":        []byte("# Linux命令\n\n![截图](图片和附件/image%201.png)\n"),
		"图片和附件/image 1.png": []byte("png-data"),
	})

	inner := zipBytes(t, map[string][]byte{
		"Java NIO.md": []byte("# Java NIO\n\n内容\n"),
		"Java集合.md":   []byte("# Java集合\n\n内容\n"),
	})
	writeZipFile(t, filepath.Join(source, "siyuan.zip"), map[string][]byte{
		"Java学习.md.zip": inner,
	})

	documents, issues, err := ScanSourceDirectory(context.Background(), source)
	if err != nil {
		t.Fatalf("ScanSourceDirectory() error = %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("ScanSourceDirectory() issues = %+v", issues)
	}
	if len(documents) != 4 {
		t.Fatalf("ScanSourceDirectory() documents = %d, want 4", len(documents))
	}

	linux := findDocument(t, documents, "Linux命令")
	if linux.Source.Kind != SourceZip || linux.Source.Collection != "Linux命令" {
		t.Fatalf("Linux source = %+v", linux.Source)
	}
	if len(linux.Assets) != 1 || linux.Assets[0].Reference != "图片和附件/image%201.png" || string(linux.Assets[0].Data) != "png-data" {
		t.Fatalf("Linux assets = %+v", linux.Assets)
	}

	java := findDocument(t, documents, "Java NIO")
	if java.Source.Kind != SourceSiyuan || java.Source.Collection != "Java学习" {
		t.Fatalf("Java source = %+v", java.Source)
	}
}

func TestScanSourceDirectoryReportsMissingZipAsset(t *testing.T) {
	source := t.TempDir()
	writeZipFile(t, filepath.Join(source, "broken.zip"), map[string][]byte{
		"broken.md": []byte("# Broken\n\n![missing](images/missing.png)\n"),
	})

	documents, issues, err := ScanSourceDirectory(context.Background(), source)
	if err != nil {
		t.Fatalf("ScanSourceDirectory() error = %v", err)
	}
	if len(documents) != 1 {
		t.Fatalf("ScanSourceDirectory() documents = %d, want 1", len(documents))
	}
	if len(issues) != 1 || issues[0].Code != IssueMissingAsset {
		t.Fatalf("ScanSourceDirectory() issues = %+v", issues)
	}
}

func findDocument(t *testing.T, documents []DocumentCandidate, title string) DocumentCandidate {
	t.Helper()
	for _, document := range documents {
		if document.Title == title {
			return document
		}
	}
	t.Fatalf("document %q not found", title)
	return DocumentCandidate{}
}

func writeZipFile(t *testing.T, destination string, entries map[string][]byte) {
	t.Helper()
	if err := os.WriteFile(destination, zipBytes(t, entries), 0o600); err != nil {
		t.Fatal(err)
	}
}

func zipBytes(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
