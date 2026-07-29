package historyimport

import "testing"

func TestNormalizeMarkdownCleansLayoutWithoutChangingText(t *testing.T) {
	input := "\ufeff# 标题\r\n\r\n\r\n第一段  \r\n\r\n\r\n\r\n## \r\n\r\n第二段\r\n"

	want := "# 标题\n\n第一段  \n\n第二段\n"
	if got := NormalizeMarkdown(input); got != want {
		t.Fatalf("NormalizeMarkdown() = %q, want %q", got, want)
	}
}

func TestNormalizeMarkdownPreservesFencedCodeWhitespace(t *testing.T) {
	input := "```Plain Text\nline one\n\n\nline four  \n```\n\n\nAfter\n"

	want := "```text\nline one\n\n\nline four  \n```\n\nAfter\n"
	if got := NormalizeMarkdown(input); got != want {
		t.Fatalf("NormalizeMarkdown() = %q, want %q", got, want)
	}
}

func TestNormalizeMarkdownNormalizesKnownFenceLanguagesOnly(t *testing.T) {
	input := "```Java\nclass Example {}\n```\n\n```Bash\necho ok\n```\n\n```custom value\nkeep\n```\n"

	want := "```java\nclass Example {}\n```\n\n```bash\necho ok\n```\n\n```custom value\nkeep\n```\n"
	if got := NormalizeMarkdown(input); got != want {
		t.Fatalf("NormalizeMarkdown() = %q, want %q", got, want)
	}
}

func TestNormalizeMarkdownKeepsMarkdownHardBreaks(t *testing.T) {
	input := "first line  \nsecond line\t\n"

	want := "first line  \nsecond line\n"
	if got := NormalizeMarkdown(input); got != want {
		t.Fatalf("NormalizeMarkdown() = %q, want %q", got, want)
	}
}
