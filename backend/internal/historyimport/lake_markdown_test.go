package historyimport

import "testing"

func TestLakeHTMLToMarkdownConvertsSupportedBlocks(t *testing.T) {
	input := `<!doctype html><div class="lake-content">
<h1>标题</h1>
<p>第一段 <strong>重点</strong> 和 <a href="https://example.com">链接</a><br>下一行</p>
<ul><li>条目一</li><li>条目二</li></ul>
<pre data-language="Java"><code>class Example {
}
</code></pre>
<table><tbody><tr><td>A</td><td>B</td></tr><tr><td>1</td><td>2</td></tr></tbody></table>
<img src="https://cdn.example.com/diagram.png" alt="示意图">
</div>`

	result, err := LakeHTMLToMarkdown(input)
	if err != nil {
		t.Fatalf("LakeHTMLToMarkdown() error = %v", err)
	}
	want := "# 标题\n\n第一段 **重点** 和 [链接](https://example.com)  \n下一行\n\n- 条目一\n- 条目二\n\n```java\nclass Example {\n}\n```\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\n![示意图](https://cdn.example.com/diagram.png)\n"
	if result.Markdown != want {
		t.Fatalf("LakeHTMLToMarkdown().Markdown = %q, want %q", result.Markdown, want)
	}
	if len(result.Images) != 1 || result.Images[0].URL != "https://cdn.example.com/diagram.png" || result.Images[0].Alt != "示意图" {
		t.Fatalf("LakeHTMLToMarkdown().Images = %+v", result.Images)
	}
}

func TestLakeHTMLToMarkdownHandlesOrderedAndNestedLists(t *testing.T) {
	input := `<div><ol><li>第一项<ul><li>子项</li></ul></li><li>第二项</li></ol></div>`

	result, err := LakeHTMLToMarkdown(input)
	if err != nil {
		t.Fatalf("LakeHTMLToMarkdown() error = %v", err)
	}
	want := "1. 第一项\n   - 子项\n2. 第二项\n"
	if result.Markdown != want {
		t.Fatalf("LakeHTMLToMarkdown().Markdown = %q, want %q", result.Markdown, want)
	}
}

func TestLakeHTMLToMarkdownRejectsMalformedInput(t *testing.T) {
	_, err := LakeHTMLToMarkdown("\x00")
	if err == nil {
		t.Fatal("LakeHTMLToMarkdown() error = nil, want malformed input error")
	}
}
