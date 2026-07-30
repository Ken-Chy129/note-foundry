package historyimport

import "testing"

func TestPrepareDocumentsDeduplicatesEquivalentExports(t *testing.T) {
	candidates := []DocumentCandidate{
		{
			Source:   SourceDescriptor{Kind: SourceSiyuan, Collection: "Java学习", Path: "siyuan.zip!Java NIO.md"},
			Title:    "Java NIO",
			Markdown: "# Java NIO\n\nNIO 通过 Channel 和 Buffer 工作。\n",
		},
		{
			Source:    SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀", Path: "技术沉淀.lakebook!java-nio.json"},
			Title:     "Java NIO",
			Markdown:  "# Java NIO\n\nNIO 通过 **Channel** 和 `Buffer` 工作。\n",
			Directory: []string{"Java"},
			Assets:    []AssetCandidate{{Reference: "https://cdn.example.com/nio.png", RemoteURL: "https://cdn.example.com/nio.png"}},
		},
	}

	result := PrepareDocuments(candidates)
	if len(result.Documents) != 1 {
		t.Fatalf("PrepareDocuments() documents = %d, want 1", len(result.Documents))
	}
	if result.Documents[0].Source.Kind != SourceLakebook {
		t.Fatalf("PrepareDocuments() kept source = %+v, want lakebook", result.Documents[0].Source)
	}
	if len(result.Duplicates) != 1 || len(result.Duplicates[0].Discarded) != 1 {
		t.Fatalf("PrepareDocuments() duplicates = %+v", result.Duplicates)
	}
	if got := result.Documents[0].Directory; len(got) != 2 || got[0] != "Java 与 JVM" || got[1] != "Java" {
		t.Fatalf("PrepareDocuments() directory = %+v", got)
	}
}

func TestPrepareDocumentsKeepsDifferentDocumentsWithSameTitle(t *testing.T) {
	candidates := []DocumentCandidate{
		{Source: SourceDescriptor{Kind: SourceSiyuan, Collection: "MySQL技术内幕"}, Title: "锁", Markdown: "# 锁\n\nInnoDB 行锁、间隙锁和 next-key lock。\n"},
		{Source: SourceDescriptor{Kind: SourceSiyuan, Collection: "MySQL是怎样运行的"}, Title: "锁", Markdown: "# 锁\n\n服务端通过表锁协调不同客户端。\n"},
	}

	result := PrepareDocuments(candidates)
	if len(result.Documents) != 2 {
		t.Fatalf("PrepareDocuments() documents = %d, want 2", len(result.Documents))
	}
	if len(result.Duplicates) != 0 {
		t.Fatalf("PrepareDocuments() duplicates = %+v, want none", result.Duplicates)
	}
}

func TestPrepareDocumentsDoesNotMergeShortTextContainedByDifferentDocument(t *testing.T) {
	candidates := []DocumentCandidate{
		{Source: SourceDescriptor{Kind: SourceSiyuan, Collection: "Linux"}, Title: "进程间通信", Markdown: "# 进程间通信\n"},
		{Source: SourceDescriptor{Kind: SourceSiyuan, Collection: "面试准备"}, Title: "Java并发", Markdown: "# Java并发\n\n进程间通信可以使用共享内存、管道或套接字。\n"},
	}

	result := PrepareDocuments(candidates)
	if len(result.Documents) != 2 {
		t.Fatalf("PrepareDocuments() documents = %d, want 2", len(result.Documents))
	}
}

func TestPrepareDocumentsReportsEmptyDocuments(t *testing.T) {
	result := PrepareDocuments([]DocumentCandidate{
		{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀", Path: "empty.json"}, Title: "常用命令"},
		{Source: SourceDescriptor{Kind: SourceSiyuan, Collection: "Linux", Path: "heading-only.md"}, Title: "进程间通信", Markdown: "# 进程间通信\n\n‍\n"},
	})

	if len(result.Documents) != 2 {
		t.Fatalf("PrepareDocuments() documents = %d, want 2", len(result.Documents))
	}
	if len(result.Issues) != 2 || result.Issues[0].Code != IssueEmptyDocument || result.Issues[1].Code != IssueEmptyDocument {
		t.Fatalf("PrepareDocuments() issues = %+v", result.Issues)
	}
}

func TestClassifyDocumentMapsRepresentativeSources(t *testing.T) {
	tests := []struct {
		name      string
		document  DocumentCandidate
		directory []string
	}{
		{name: "graduation", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "毕业设计"}, Directory: []string{"G1 GC"}}, directory: []string{"毕业设计", "G1 GC"}},
		{name: "mysql internals", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceSiyuan, Collection: "MySQL技术内幕"}}, directory: []string{"数据库与数据工程", "MySQL", "技术内幕"}},
		{name: "mysql runtime", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceSiyuan, Collection: "MySQL是怎样运行的"}}, directory: []string{"数据库与数据工程", "MySQL", "运行原理"}},
		{name: "mysql interview", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceSiyuan, Collection: "中间件学习"}, Title: "MySQL八股"}, directory: []string{"数据库与数据工程", "MySQL", "面试"}},
		{name: "data warehouse", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceZip, Collection: "数仓分层"}, Title: "数仓分层"}, directory: []string{"数据库与数据工程", "数据仓库"}},
		{name: "hologress", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceMarkdown, Collection: "独立文档"}, Title: "Hologress"}, directory: []string{"数据库与数据工程", "数据仓库"}},
		{name: "database basics", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀"}, Title: "时间", Directory: []string{"SQLite"}}, directory: []string{"数据库与数据工程", "数据库基础"}},
		{name: "database operations", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceZip, Collection: "数据库容灾"}, Title: "数据库容灾"}, directory: []string{"数据库与数据工程", "数据库运维"}},
		{name: "distributed", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceZip, Collection: "分布式事务"}}, directory: []string{"系统、网络与分布式", "分布式与中间件"}},
		{name: "personal", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceMarkdown, Collection: "独立文档"}, Title: "OKR"}, directory: []string{"学习、求职与个人", "个人规划"}},
		{name: "technical java", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀"}, Title: "Java NIO", Directory: []string{"Java"}}, directory: []string{"Java 与 JVM", "Java"}},
		{name: "jvm tuning", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceMarkdown, Collection: "独立文档"}, Title: "调优实战"}, directory: []string{"Java 与 JVM", "JVM"}},
		{name: "memory metrics", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceMarkdown, Collection: "独立文档"}, Title: "内存committed和used"}, directory: []string{"Java 与 JVM", "JVM"}},
		{name: "garbage collection", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceZip, Collection: "垃圾回收"}, Title: "垃圾回收"}, directory: []string{"Java 与 JVM", "JVM"}},
		{name: "remembered set", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceZip, Collection: "卡表和记忆集"}, Title: "卡表和记忆集"}, directory: []string{"Java 与 JVM", "JVM"}},
		{name: "spring boot tip", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceZip, Collection: "开发小贴士"}, Title: "SpringBoot相关"}, directory: []string{"Java 与 JVM", "Spring"}},
		{name: "spring tip", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀"}, Title: "Spring相关", Directory: []string{"开发贴士"}}, directory: []string{"Java 与 JVM", "Spring"}},
		{name: "spring problem", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀"}, Title: "程序包org.springframework.jdbc.core不存在", Directory: []string{"问题"}}, directory: []string{"Java 与 JVM", "Spring"}},
		{name: "java tip", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀"}, Title: "Java相关", Directory: []string{"开发贴士"}}, directory: []string{"Java 与 JVM", "Java"}},
		{name: "nested jvm implementation", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀"}, Title: "概念", Directory: []string{"JVM", "GraalVM"}}, directory: []string{"Java 与 JVM", "JVM", "GraalVM"}},
		{name: "git tip", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀"}, Title: "git超时", Directory: []string{"开发贴士"}}, directory: []string{"工程实践与项目", "版本控制"}},
		{name: "engineering tip", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀"}, Title: "日志系统", Directory: []string{"开发贴士"}}, directory: []string{"工程实践与项目", "工程实践"}},
		{name: "database problem", document: DocumentCandidate{Source: SourceDescriptor{Kind: SourceLakebook, Collection: "技术沉淀"}, Title: "自增主键问题", Directory: []string{"问题"}}, directory: []string{"数据库与数据工程", "MySQL"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ClassifyDocument(test.document)
			if !equalStrings(got, test.directory) {
				t.Fatalf("ClassifyDocument() = %+v, want %+v", got, test.directory)
			}
		})
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
