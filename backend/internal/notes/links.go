package notes

import (
	"regexp"
	"strings"
)

var noteLinkPattern = regexp.MustCompile(`(?i)note:([0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12})`)

func ExtractNoteLinkTargets(markdown string) []string {
	matches := noteLinkPattern.FindAllStringSubmatch(markdown, -1)
	targets := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		target := strings.ToLower(match[1])
		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets
}
