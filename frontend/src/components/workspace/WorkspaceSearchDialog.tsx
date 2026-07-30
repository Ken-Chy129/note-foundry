"use client";

import { Search } from "lucide-react";
import { ReactNode, useEffect, useMemo, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { KnowledgeSpace, LearningNote, PageResponse, SearchResult } from "@/lib/types";
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

function formatNoteTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";

  const now = new Date();
  const clock = `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
  const dateDay = Date.UTC(date.getFullYear(), date.getMonth(), date.getDate());
  const today = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  const daysAgo = Math.round((today - dateDay) / 86_400_000);

  if (daysAgo === 0) return `今天 · ${clock}`;
  if (daysAgo === 1) return `昨天 · ${clock}`;
  if (date.getFullYear() === now.getFullYear()) return `${date.getMonth() + 1}月${date.getDate()}日 · ${clock}`;
  return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日 · ${clock}`;
}

export function WorkspaceSearchDialog({ spaces, onClose, onSelectResult }: WorkspaceSearchDialogProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [recentResults, setRecentResults] = useState<SearchResult[]>([]);
  const [recentStatus, setRecentStatus] = useState<"loading" | "success" | "error">("loading");
  const [totalItems, setTotalItems] = useState(0);
  const [status, setStatus] = useState<SearchStatus>("idle");
  const spaceNames = useMemo(() => new Map(spaces.map((space) => [space.id, space.name])), [spaces]);

  useEffect(() => {
    const controller = new AbortController();

    void apiFetch<PageResponse<LearningNote>>("/api/v1/notes?pageSize=12", { signal: controller.signal })
      .then((response) => {
        setRecentResults(response.data.map((note) => ({
          id: note.id,
          spaceId: note.spaceId,
          title: note.title,
          slug: note.slug,
          snippet: "",
          rank: 0,
          updatedAt: note.updatedAt
        })));
        setRecentStatus("success");
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setRecentStatus("error");
      });

    return () => controller.abort();
  }, []);

  useEffect(() => {
    const value = query.trim();
    if (!value) return;

    const controller = new AbortController();
    const timeout = window.setTimeout(() => {
      setStatus("loading");
      void apiFetch<PageResponse<SearchResult>>(`/api/v1/search?q=${encodeURIComponent(value)}&pageSize=100`, { signal: controller.signal })
        .then((response) => {
          setResults(response.data);
          setTotalItems(response.pagination.totalItems);
          setStatus("success");
        })
        .catch((error: unknown) => {
          if (error instanceof DOMException && error.name === "AbortError") return;
          setResults([]);
          setTotalItems(0);
          setStatus("error");
        });
    }, 250);

    return () => {
      window.clearTimeout(timeout);
      controller.abort();
    };
  }, [query]);

  function updateQuery(value: string) {
    setQuery(value);
    if (value.trim()) return;
    setResults([]);
    setTotalItems(0);
    setStatus("idle");
  }

  const resultSummary = status === "success"
    ? totalItems === 0
      ? "没有结果"
      : results.length < totalItems ? `显示 ${results.length} / ${totalItems} 条结果` : `${totalItems} 条结果`
    : status === "loading" ? "正在搜索…" : status === "error" ? "搜索失败" : `${recentResults.length} 篇最近文章`;

  return (
    <WorkspaceDialog
      title="搜索笔记"
      className="workspace-search-dialog"
      onClose={onClose}
    >
      <div className="workspace-search-shell">
        <div className="workspace-search-form" role="search">
          <div className="workspace-search-field">
            <Search size={19} aria-hidden="true" />
            <input
              autoFocus
              type="search"
              value={query}
              onChange={(event) => updateQuery(event.target.value)}
              placeholder="输入标题、正文或技术关键词"
              aria-label="搜索学习笔记"
            />
          </div>
        </div>

        <div className="workspace-search-results" aria-busy={status === "loading"}>
          <span className="workspace-search-status" aria-live="polite">{resultSummary}</span>
          {status === "idle" && recentStatus === "loading" && <div className="workspace-search-loading" role="status">
            {[0, 1, 2, 3].map((item) => <span key={item} />)}
          </div>}

          {status === "idle" && recentStatus === "error" && <div className="workspace-search-empty is-compact">
            <p>最近文章暂时无法加载。</p>
          </div>}

          {status === "idle" && recentStatus === "success" && recentResults.length === 0 && <div className="workspace-search-empty is-compact">
            <p>还没有学习笔记。</p>
          </div>}

          {status === "idle" && recentResults.map((result) => (
            <button key={result.id} className="workspace-search-result" onClick={() => void onSelectResult(result)}>
              <span className="workspace-search-result-copy">
                <span className="workspace-search-result-meta">
                  <span>{spaceNames.get(result.spaceId) ?? "知识空间"}</span>
                  <time className="workspace-search-time" dateTime={result.updatedAt}>{formatNoteTime(result.updatedAt)}</time>
                </span>
                <strong>{result.title}</strong>
              </span>
            </button>
          ))}

          {status === "loading" && <div className="workspace-search-loading" role="status">
            <p>正在搜索…</p>
            {[0, 1, 2, 3].map((item) => <span key={item} />)}
          </div>}

          {status === "error" && <div className="workspace-search-empty is-compact is-error">
            <p>搜索暂时不可用，请稍后重试。</p>
          </div>}

          {status === "success" && results.length === 0 && <div className="workspace-search-empty is-compact">
            <p>没有找到相关笔记</p>
          </div>}

          {status === "success" && results.map((result) => (
            <button key={result.id} className="workspace-search-result" onClick={() => void onSelectResult(result)}>
              <span className="workspace-search-result-copy">
                <span className="workspace-search-result-meta">
                  <span>{spaceNames.get(result.spaceId) ?? "知识空间"}</span>
                  <time className="workspace-search-time" dateTime={result.updatedAt}>{formatNoteTime(result.updatedAt)}</time>
                </span>
                <strong>{highlightedText(result.title, query)}</strong>
                <span className="workspace-search-snippet">{highlightedText(result.snippet, query)}</span>
              </span>
            </button>
          ))}
        </div>
      </div>
    </WorkspaceDialog>
  );
}
