package historyimport

import (
	"regexp"
	"strings"
)

var emptyHeadingPattern = regexp.MustCompile(`^\s{0,3}#{1,6}\s*$`)

var fenceLanguages = map[string]string{
	"bash":       "bash",
	"java":       "java",
	"javascript": "javascript",
	"json":       "json",
	"plain text": "text",
	"plaintext":  "text",
	"shell":      "shell",
	"sql":        "sql",
	"text":       "text",
	"typescript": "typescript",
	"xml":        "xml",
	"yaml":       "yaml",
	"yml":        "yaml",
}

// NormalizeMarkdown applies layout-only cleanup while preserving prose and
// fenced code contents. It deliberately avoids reflowing paragraphs or lists.
func NormalizeMarkdown(markdown string) string {
	markdown = strings.TrimPrefix(markdown, "\ufeff")
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = strings.ReplaceAll(markdown, "\r", "\n")

	lines := strings.Split(markdown, "\n")
	normalized := make([]string, 0, len(lines))
	inFence := false
	fenceMarker := byte(0)
	blankPending := false

	for _, line := range lines {
		if marker, info, ok := fencedLine(line); ok {
			if !inFence {
				appendPendingBlank(&normalized, &blankPending)
				line = normalizeFenceOpening(line, marker, info)
				inFence = true
				fenceMarker = marker
			} else if marker == fenceMarker && strings.TrimSpace(info) == "" {
				inFence = false
				fenceMarker = 0
			}
			normalized = append(normalized, line)
			continue
		}

		if inFence {
			normalized = append(normalized, line)
			continue
		}

		line = trimLayoutWhitespace(line)
		if emptyHeadingPattern.MatchString(line) {
			blankPending = len(normalized) > 0
			continue
		}
		if strings.TrimSpace(line) == "" {
			blankPending = len(normalized) > 0
			continue
		}

		appendPendingBlank(&normalized, &blankPending)
		normalized = append(normalized, line)
	}

	for len(normalized) > 0 && normalized[len(normalized)-1] == "" {
		normalized = normalized[:len(normalized)-1]
	}
	if len(normalized) == 0 {
		return ""
	}
	return strings.Join(normalized, "\n") + "\n"
}

func appendPendingBlank(lines *[]string, pending *bool) {
	if *pending && len(*lines) > 0 && (*lines)[len(*lines)-1] != "" {
		*lines = append(*lines, "")
	}
	*pending = false
}

func trimLayoutWhitespace(line string) string {
	withoutTabs := strings.TrimRight(line, "\t")
	trailingSpaces := len(withoutTabs) - len(strings.TrimRight(withoutTabs, " "))
	if trailingSpaces >= 2 {
		return strings.TrimRight(withoutTabs, " ") + "  "
	}
	return strings.TrimRight(withoutTabs, " ")
}

func fencedLine(line string) (byte, string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 {
		return 0, "", false
	}
	marker := trimmed[0]
	if marker != '`' && marker != '~' {
		return 0, "", false
	}
	count := 0
	for count < len(trimmed) && trimmed[count] == marker {
		count++
	}
	if count < 3 {
		return 0, "", false
	}
	return marker, trimmed[count:], true
}

func normalizeFenceOpening(line string, marker byte, info string) string {
	key := strings.ToLower(strings.TrimSpace(info))
	language, ok := fenceLanguages[key]
	if !ok {
		return line
	}
	indentLength := len(line) - len(strings.TrimLeft(line, " "))
	trimmed := strings.TrimLeft(line, " ")
	markerLength := 0
	for markerLength < len(trimmed) && trimmed[markerLength] == marker {
		markerLength++
	}
	return line[:indentLength] + trimmed[:markerLength] + language
}
