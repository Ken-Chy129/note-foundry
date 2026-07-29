package historyimport

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScanSourceDirectoryReadsLakebookHierarchy(t *testing.T) {
	source := t.TempDir()
	metaValue, err := json.Marshal(map[string]any{
		"book": map[string]any{
			"tocYml": "- type: TITLE\n  title: Java\n  uuid: title-java\n  parent_uuid: ''\n  level: 0\n- type: TITLE\n  title: JVM\n  uuid: title-jvm\n  parent_uuid: title-java\n  level: 1\n- type: DOC\n  title: 安全点\n  uuid: doc-safe\n  url: safe-point\n  parent_uuid: title-jvm\n  level: 2\n",
			"public": 0,
		},
		"version": "1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	metaOuter, err := json.Marshal(map[string]any{"meta": string(metaValue)})
	if err != nil {
		t.Fatal(err)
	}
	docOuter, err := json.Marshal(map[string]any{
		"doc": map[string]any{
			"title": "安全点",
			"slug":  "safe-point",
			"body":  `<div><h1>安全点</h1><p>正文</p><img src="https://cdn.example.com/safe.png" alt="图"></div>`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeTarFile(t, filepath.Join(source, "技术沉淀.lakebook"), map[string][]byte{
		"root/$meta.json":      metaOuter,
		"root/safe-point.json": docOuter,
	})

	documents, issues, err := ScanSourceDirectory(context.Background(), source)
	if err != nil {
		t.Fatalf("ScanSourceDirectory() error = %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("ScanSourceDirectory() issues = %+v", issues)
	}
	if len(documents) != 1 {
		t.Fatalf("ScanSourceDirectory() documents = %d, want 1", len(documents))
	}
	document := documents[0]
	if document.Source.Kind != SourceLakebook || document.Source.Collection != "技术沉淀" {
		t.Fatalf("Lakebook source = %+v", document.Source)
	}
	if len(document.Directory) != 2 || document.Directory[0] != "Java" || document.Directory[1] != "JVM" {
		t.Fatalf("Lakebook directory = %+v", document.Directory)
	}
	if document.Markdown != "# 安全点\n\n正文\n\n![图](https://cdn.example.com/safe.png)\n" {
		t.Fatalf("Lakebook markdown = %q", document.Markdown)
	}
	if len(document.Assets) != 1 || document.Assets[0].RemoteURL != "https://cdn.example.com/safe.png" {
		t.Fatalf("Lakebook assets = %+v", document.Assets)
	}
}

func writeTarFile(t *testing.T, destination string, entries map[string][]byte) {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, content := range entries {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}
