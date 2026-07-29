package search

import "testing"

func TestExtractMarkdownHeadingsIgnoresCodeFences(t *testing.T) {
	markdown := "# Agent Loop\n\n## Memory 管理\n\n```go\n# not a heading\n```\n"
	if got := ExtractMarkdownHeadings(markdown); got != "Agent Loop\nMemory 管理" {
		t.Errorf("headings = %q", got)
	}
}
