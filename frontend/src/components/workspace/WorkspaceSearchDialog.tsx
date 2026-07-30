"use client";

import { ArrowUpRight, CornerDownLeft, Search, X } from "lucide-react";
import { FormEvent, ReactNode, useMemo, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { KnowledgeSpace, PageResponse, SearchResult } from "@/lib/types";
import { WorkspaceDialog } from "@/components/workspace/WorkspaceDialog";

type SearchStatus = "idle" | "loading" | "success" | "error";

interface WorkspaceSearchDialogProps {
  spaces: KnowledgeSpace[];
  onClose: () => void;
  onSelectResult: (result: SearchResult) => Promise<void>;
}

function highlightedText(text: string, query: string): ReactNode {
  const terms = query.trim().split(/\s+/).filter(Boolean);
  if (terms.length === 0) return text;

  const pattern = new RegExp(`(${terms.map((term) => term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|")})`, "gi");
  const normalizedTerms = new Set(terms.map((term) => term.toLocaleLowerCase("zh-CN")));

  return text.split(pattern).map((part, index) => normalizedTerms.has(part.toLocaleLowerCase("zh-CN"))
    ? <mark key={`${part}-${index}`}>{part}</mark>
    : part);
}

export function WorkspaceSearchDialog({ spaces, onClose, onSelectResult }: WorkspaceSearchDialogProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [totalItems, setTotalItems] = useState(0);
  const [status, setStatus] = useState<SearchStatus>("idle");
  const spaceNames = useMemo(() => new Map(spaces.map((space) => [space.id, space.name])), [spaces]);

  async function search(event: FormEvent) {
    event.preventDefault();
    const value = query.trim();
    if (!value || status === "loading") return;

    setStatus("loading");
    try {
      const response = await apiFetch<PageResponse<SearchResult>>(`/api/v1/search?q=${encodeURIComponent(value)}&pageSize=100`);
      setResults(response.data);
      setTotalItems(response.pagination.totalItems);
      setStatus("success");
    } catch {
      setResults([]);
      setTotalItems(0);
      setStatus("error");
    }
  }

  function clearSearch() {
    setQuery("");
    setResults([]);
    setTotalItems(0);
    setStatus("idle");
  }

  const resultSummary = status === "success"
    ? totalItems === 0
      ? "没有结果"
      : results.length < totalItems ? `显示 ${results.length} / ${totalItems} 条结果` : `${totalItems} 条结果`
    : status === "loading" ? "正在搜索…" : status === "error" ? "搜索失败" : "搜索全部知识空间";

  return (
    <WorkspaceDialog
      title="搜索笔记"
      description="按标题、正文和标签查找学习笔记"
      className="workspace-search-dialog"
      onClose={onClose}
    >
      <div className="workspace-search-shell">
        <form className="workspace-search-form" onSubmit={search} role="search">
          <div className="workspace-search-field">
            <Search size={19} aria-hidden="true" />
            <input
              autoFocus
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="输入标题、正文或技术关键词"
              aria-label="搜索学习笔记"
            />
            {query && <button type="button" className="workspace-search-clear" onClick={clearSearch} aria-label="清空搜索"><X size={15} /></button>}
            <button className="workspace-search-submit" disabled={!query.trim() || status === "loading"} aria-label="执行搜索">
              <CornerDownLeft size={16} />
            </button>
          </div>
        </form>

        <div className="workspace-search-context">
          <span aria-live="polite">{resultSummary}</span>
          <small>按相关度排序</small>
        </div>

        <div className="workspace-search-results" aria-busy={status === "loading"}>
          {status === "idle" && <div className="workspace-search-empty">
            <span><Search size={20} /></span>
            <strong>从你的学习记录中定位一个问题</strong>
            <p>支持中文、英文和技术关键词，例如“上下文窗口”或“TCP recycle”。</p>
          </div>}

          {status === "loading" && <div className="workspace-search-loading" role="status">
            <p>正在搜索…</p>
            {[0, 1, 2, 3].map((item) => <span key={item} />)}
          </div>}

          {status === "error" && <div className="workspace-search-empty is-error">
            <strong>搜索暂时不可用</strong>
            <p>请检查服务状态后重试，当前输入不会丢失。</p>
          </div>}

          {status === "success" && results.length === 0 && <div className="workspace-search-empty">
            <strong>没有找到相关笔记</strong>
            <p>尝试减少关键词，或改用标题中的词语搜索。</p>
          </div>}

          {status === "success" && results.map((result, index) => (
            <button key={result.id} className="workspace-search-result" onClick={() => void onSelectResult(result)}>
              <span className="workspace-search-index">{String(index + 1).padStart(2, "0")}</span>
              <span className="workspace-search-result-copy">
                <span className="workspace-search-result-meta">{spaceNames.get(result.spaceId) ?? "知识空间"}<i />学习笔记</span>
                <strong>{highlightedText(result.title, query)}</strong>
                <span className="workspace-search-snippet">{highlightedText(result.snippet, query)}</span>
              </span>
              <ArrowUpRight size={17} aria-hidden="true" />
            </button>
          ))}
        </div>

        <footer className="workspace-search-footer"><kbd>Enter</kbd> 搜索 <span aria-hidden="true">·</span> <kbd>Esc</kbd> 关闭</footer>
      </div>
    </WorkspaceDialog>
  );
}
