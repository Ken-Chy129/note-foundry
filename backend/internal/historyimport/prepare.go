package historyimport

import (
	"sort"
	"strings"
	"unicode"
)

type DuplicateGroup struct {
	Kept      SourceDescriptor   `json:"kept"`
	Discarded []SourceDescriptor `json:"discarded"`
}

type PrepareResult struct {
	Documents  []DocumentCandidate `json:"documents"`
	Duplicates []DuplicateGroup    `json:"duplicates"`
	Issues     []ScanIssue         `json:"issues"`
}

func PrepareDocuments(candidates []DocumentCandidate) PrepareResult {
	prepared := make([]DocumentCandidate, len(candidates))
	copy(prepared, candidates)

	normalized := make([]string, len(prepared))
	titleKeys := make([]string, len(prepared))
	shingles := make([]map[string]struct{}, len(prepared))
	for index, document := range prepared {
		normalized[index] = normalizedDocumentBody(document.Markdown)
		titleKeys[index] = normalizedTitle(document.Title)
		shingles[index] = textShingles(normalized[index], 5)
	}

	groups := newDisjointSet(len(prepared))
	for left := range prepared {
		if normalized[left] == "" {
			continue
		}
		for right := left + 1; right < len(prepared); right++ {
			if normalized[right] == "" {
				continue
			}
			similarity := containmentSimilarity(shingles[left], shingles[right])
			sameTitle := titleKeys[left] != "" && titleKeys[left] == titleKeys[right]
			if normalized[left] == normalized[right] || (sameTitle && similarity >= 0.85) {
				groups.union(left, right)
			}
		}
	}

	indicesByRoot := make(map[int][]int)
	for index := range prepared {
		root := groups.find(index)
		indicesByRoot[root] = append(indicesByRoot[root], index)
	}

	result := PrepareResult{}
	for _, indices := range indicesByRoot {
		keptIndex := indices[0]
		for _, index := range indices[1:] {
			if preferDocument(prepared[index], prepared[keptIndex], normalized[index], normalized[keptIndex]) {
				keptIndex = index
			}
		}
		kept := prepared[keptIndex]
		kept.Directory = ClassifyDocument(kept)
		result.Documents = append(result.Documents, kept)

		if len(indices) > 1 {
			duplicate := DuplicateGroup{Kept: kept.Source}
			for _, index := range indices {
				if index != keptIndex {
					duplicate.Discarded = append(duplicate.Discarded, prepared[index].Source)
				}
			}
			result.Duplicates = append(result.Duplicates, duplicate)
		}
		if normalized[keptIndex] == "" {
			result.Issues = append(result.Issues, ScanIssue{
				Code:       IssueEmptyDocument,
				SourcePath: kept.Source.Path,
				Message:    "document has no body content",
			})
		}
	}

	sort.Slice(result.Documents, func(left, right int) bool {
		leftPath := strings.Join(result.Documents[left].Directory, "/") + "/" + result.Documents[left].Title
		rightPath := strings.Join(result.Documents[right].Directory, "/") + "/" + result.Documents[right].Title
		if leftPath == rightPath {
			return result.Documents[left].Source.Path < result.Documents[right].Source.Path
		}
		return leftPath < rightPath
	})
	sort.Slice(result.Duplicates, func(left, right int) bool {
		return result.Duplicates[left].Kept.Path < result.Duplicates[right].Kept.Path
	})
	return result
}

func ClassifyDocument(document DocumentCandidate) []string {
	collection := strings.TrimSpace(document.Source.Collection)
	title := strings.TrimSpace(document.Title)
	joined := strings.ToLower(collection + " " + title + " " + strings.Join(document.Directory, " "))

	if collection == "毕业设计" {
		return appendPath([]string{"毕业设计"}, document.Directory...)
	}
	if collection == "个人规划" || collection == "面试准备" || collection == "课内复习" || title == "OKR" {
		subdirectory := collection
		if title == "OKR" || subdirectory == "独立文档" {
			subdirectory = "个人规划"
		}
		return appendPath([]string{"学习、求职与个人", subdirectory}, document.Directory...)
	}
	if collection == "技术周报" {
		return appendPath([]string{"工程实践与项目", "技术周报"}, document.Directory...)
	}
	if collection == "开源项目" || collection == "项目相关" {
		return appendPath([]string{"工程实践与项目", collection}, document.Directory...)
	}
	if containsAny(joined, "调优实战", "committed", "used memory") {
		return []string{"Java 与 JVM", "JVM 调优"}
	}

	if containsAny(joined, "mysql", "hologress", "hadoop", "数仓", "数据仓库", "事实表", "维表", "列式存储", "数据库容灾", "sqlite", "自增主键") {
		return appendPath([]string{"数据库与数据工程", preferredSubdirectory(collection, document.Directory, "数据库")}, remainingPath(document.Directory)...)
	}
	if containsAny(joined, "java", "jvm", "netty", "spring", "垃圾回收", "gc", "字节码", "内存溢出", "安全点", "finalize", "元空间", "flight record", "卡表", "记忆集") {
		return appendPath([]string{"Java 与 JVM", preferredSubdirectory(collection, document.Directory, "JVM")}, remainingPath(document.Directory)...)
	}
	if containsAny(joined, "linux", "操作系统", "网络", "tcp", "https", "osi", "分布式", "raft", "paxos", "一致性哈希", "事务", "redis", "rocketmq", "sentinel", "raid", "glibc", "中间件") {
		subdirectory := systemSubdirectory(joined)
		return appendPath([]string{"系统、网络与分布式", subdirectory}, remainingPath(document.Directory)...)
	}
	if collection == "开发小贴士" || containsAny(joined, "git", "maven", "umi", "前端", "开发", "工具", "项目", "算法") {
		return appendPath([]string{"工程实践与项目", preferredSubdirectory(collection, document.Directory, "工程实践")}, remainingPath(document.Directory)...)
	}
	return appendPath([]string{"待整理", collection}, document.Directory...)
}

func preferredSubdirectory(collection string, original []string, fallback string) string {
	if len(original) > 0 && strings.TrimSpace(original[0]) != "" {
		return strings.TrimSpace(original[0])
	}
	if collection != "" && collection != "独立文档" && collection != "技术沉淀" {
		return collection
	}
	return fallback
}

func remainingPath(original []string) []string {
	if len(original) <= 1 {
		return nil
	}
	return original[1:]
}

func systemSubdirectory(joined string) string {
	switch {
	case containsAny(joined, "网络", "tcp", "https", "osi", "netty"):
		return "计算机网络"
	case containsAny(joined, "分布式", "raft", "paxos", "一致性哈希", "事务", "redis", "rocketmq", "sentinel", "中间件"):
		return "分布式与中间件"
	default:
		return "操作系统与 Linux"
	}
}

func appendPath(base []string, additions ...string) []string {
	result := append([]string(nil), base...)
	for _, addition := range additions {
		addition = strings.TrimSpace(addition)
		if addition == "" || (len(result) > 0 && result[len(result)-1] == addition) {
			continue
		}
		result = append(result, addition)
	}
	return result
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func preferDocument(candidate, current DocumentCandidate, candidateText, currentText string) bool {
	candidateScore := sourceScore(candidate) + len(candidate.Assets)*1000
	currentScore := sourceScore(current) + len(current.Assets)*1000
	if candidateScore != currentScore {
		return candidateScore > currentScore
	}
	if len(candidateText) != len(currentText) {
		return len(candidateText) > len(currentText)
	}
	return candidate.Source.Path < current.Source.Path
}

func sourceScore(document DocumentCandidate) int {
	score := 0
	switch document.Source.Kind {
	case SourceLakebook:
		score = 400
	case SourceZip:
		score = 300
	case SourceMarkdown:
		score = 250
	case SourceSiyuan:
		score = 200
	}
	if document.Source.Collection == "技术沉淀" {
		score += 20
	}
	return score
}

func normalizedTitle(title string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(title) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func normalizedDocumentBody(markdown string) string {
	lines := strings.Split(markdown, "\n")
	headingRemoved := false
	for index, line := range lines {
		if !headingRemoved && firstHeadingPattern.MatchString(line) {
			lines[index] = ""
			headingRemoved = true
			break
		}
	}

	var builder strings.Builder
	for _, character := range strings.ToLower(strings.Join(lines, "\n")) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func textShingles(text string, size int) map[string]struct{} {
	runes := []rune(text)
	result := make(map[string]struct{})
	if len(runes) == 0 {
		return result
	}
	if len(runes) < size {
		result[string(runes)] = struct{}{}
		return result
	}
	for index := 0; index+size <= len(runes); index++ {
		result[string(runes[index:index+size])] = struct{}{}
	}
	return result
}

func containmentSimilarity(left, right map[string]struct{}) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	smaller, larger := left, right
	if len(left) > len(right) {
		smaller, larger = right, left
	}
	intersection := 0
	for shingle := range smaller {
		if _, ok := larger[shingle]; ok {
			intersection++
		}
	}
	return float64(intersection) / float64(len(smaller))
}

type disjointSet struct {
	parent []int
}

func newDisjointSet(size int) *disjointSet {
	set := &disjointSet{parent: make([]int, size)}
	for index := range set.parent {
		set.parent[index] = index
	}
	return set
}

func (set *disjointSet) find(value int) int {
	if set.parent[value] != value {
		set.parent[value] = set.find(set.parent[value])
	}
	return set.parent[value]
}

func (set *disjointSet) union(left, right int) {
	leftRoot := set.find(left)
	rightRoot := set.find(right)
	if leftRoot != rightRoot {
		set.parent[rightRoot] = leftRoot
	}
}
