"use client";

import Link from "next/link";
import { Search, X } from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { PageResponse, SearchResult } from "@/lib/types";

export function PublicSearch() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [state, setState] = useState<"idle" | "loading" | "ready" | "error">("idle");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    inputRef.current?.focus();

    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }

    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [open]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    const value = query.trim();
    if (!value) return;
    setState("loading");
    try {
      const response = await apiFetch<PageResponse<SearchResult>>(`/api/v1/public/search?q=${encodeURIComponent(value)}&pageSize=8`);
      setResults(response.data);
      setState("ready");
    } catch {
      setState("error");
    }
  }

  function clear() {
    setQuery("");
    setResults([]);
    setState("idle");
  }

  return (
    <>
      <button className="public-search-trigger" type="button" onClick={() => setOpen(true)} aria-haspopup="dialog" aria-label="搜索公开笔记">
        <Search aria-hidden="true" size={16} />
        <span>搜索</span>
      </button>

      {open && (
        <div className="public-search-overlay">
          <section className="public-search-dialog" role="dialog" aria-modal="true" aria-labelledby="search-title">
            <header>
              <div>
                <p>全文检索</p>
                <h2 id="search-title">搜索公开笔记</h2>
              </div>
              <button className="public-search-close" type="button" onClick={() => setOpen(false)} aria-label="关闭搜索">
                <X aria-hidden="true" size={20} />
              </button>
            </header>
            <form onSubmit={submit} role="search">
              <Search aria-hidden="true" size={19} />
              <input
                ref={inputRef}
                id="public-note-search"
                type="search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="搜索标题、正文或技术关键词"
                aria-label="搜索已发布的学习笔记"
              />
              {query && <button type="button" className="icon-button" onClick={clear} aria-label="清空搜索"><X size={18} /></button>}
              <button className="search-submit" type="submit" disabled={state === "loading"}>
                {state === "loading" ? "搜索中…" : "搜索"}
              </button>
            </form>
            <div className="search-results" aria-live="polite">
              {state === "idle" && <p className="search-guidance">输入中文、英文或技术关键词</p>}
              {state === "error" && <p className="inline-error">暂时无法搜索，请稍后重试。</p>}
              {state === "ready" && results.length === 0 && <p className="empty-copy">没有匹配该关键词的已发布笔记。</p>}
              {results.map((result) => (
                <Link className="search-result" key={result.id} href={`/notes/${result.id}/${result.slug}`}>
                  <span>{result.title}</span>
                  <p>{result.snippet}</p>
                </Link>
              ))}
            </div>
          </section>
        </div>
      )}
    </>
  );
}
