"use client";

import Link from "next/link";
import { Search, X } from "lucide-react";
import { FormEvent, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { PageResponse, SearchResult } from "@/lib/types";

export function PublicSearch() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [state, setState] = useState<"idle" | "loading" | "ready" | "error">("idle");

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
    <section className="public-search" aria-labelledby="search-title">
      <h2 id="search-title">搜索公开笔记</h2>
      <form onSubmit={submit} role="search">
        <Search aria-hidden="true" size={19} />
        <input
          id="public-note-search"
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="输入关键词"
          aria-label="搜索已发布的学习笔记"
        />
        {query && <button type="button" className="icon-button" onClick={clear} aria-label="清空搜索"><X size={18} /></button>}
        <button className="search-submit" type="submit" disabled={state === "loading"}>
          {state === "loading" ? "搜索中…" : "搜索"}
        </button>
      </form>
      <div className="search-results" aria-live="polite">
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
  );
}
