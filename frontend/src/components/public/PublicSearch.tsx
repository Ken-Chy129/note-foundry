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
      <div className="section-kicker">Bilingual retrieval</div>
      <h2 id="search-title">Search what has been learned</h2>
      <form onSubmit={submit} role="search">
        <Search aria-hidden="true" size={20} />
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Try “memory”, “上下文”, or “Agent Loop”"
          aria-label="Search published notes"
        />
        {query && <button type="button" className="icon-button" onClick={clear} aria-label="Clear search"><X size={18} /></button>}
        <button className="button button-primary" type="submit" disabled={state === "loading"}>
          {state === "loading" ? "Searching…" : "Search"}
        </button>
      </form>
      <div className="search-results" aria-live="polite">
        {state === "error" && <p className="inline-error">Search is unavailable. Try again in a moment.</p>}
        {state === "ready" && results.length === 0 && <p className="empty-copy">No published notes match this phrase.</p>}
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
