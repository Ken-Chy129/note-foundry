package search

import (
	"bufio"
	"strings"
)

func ExtractMarkdownHeadings(markdown string) string {
	scanner := bufio.NewScanner(strings.NewReader(markdown))
	headings := make([]string, 0)
	inFence := false
	fenceMarker := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			marker := line[:3]
			if !inFence {
				inFence = true
				fenceMarker = marker
			} else if marker == fenceMarker {
				inFence = false
				fenceMarker = ""
			}
			continue
		}
		if inFence || !strings.HasPrefix(line, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
		if heading != "" {
			headings = append(headings, heading)
		}
	}
	return strings.Join(headings, "\n")
}
