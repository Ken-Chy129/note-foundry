import Link from "next/link";
import { ArrowRight, BookOpenText, LockKeyhole } from "lucide-react";
import { PublicSearch } from "@/components/public/PublicSearch";
import { SiteHeader } from "@/components/public/SiteHeader";
import { publicApiFetch } from "@/lib/public-api";

export const dynamic = "force-dynamic";
import type { KnowledgeSpace, PageResponse } from "@/lib/types";

export default async function HomePage() {
  const response = await publicApiFetch<PageResponse<KnowledgeSpace>>("/api/v1/public/spaces?page=1&pageSize=50");
  const spaces = response?.data ?? [];

  return (
    <main>
      <SiteHeader />
      <section className="home-hero">
        <div className="knowledge-spine" aria-hidden="true"><span /></div>
        <div className="hero-copy">
          <p className="section-kicker">A dependable record of understanding</p>
          <h1>Learning notes,<br /><em>forged to last.</em></h1>
          <p className="hero-summary">
            NoteFoundry is a personal knowledge workspace where reviewed Markdown becomes durable, linkable learning—not another stream of saved material.
          </p>
          <div className="hero-actions">
            <a className="button button-primary" href="#spaces">Browse knowledge</a>
            <Link className="button button-quiet" href="/app">Open owner workspace <ArrowRight size={17} /></Link>
          </div>
        </div>
        <aside className="hero-principles" aria-label="Workspace principles">
          <div><BookOpenText size={18} /><span>Canonical Markdown</span></div>
          <div><LockKeyhole size={18} /><span>Drafts stay private</span></div>
          <p>Every published page has a stable identity, a recoverable history, and an explicit owner decision behind it.</p>
        </aside>
      </section>

      <PublicSearch />

      <section className="space-index" id="spaces" aria-labelledby="spaces-title">
        <div className="section-heading">
          <div>
            <p className="section-kicker">Public index</p>
            <h2 id="spaces-title">Knowledge spaces</h2>
          </div>
          <p>Long-lived areas of study. Directories organize within them; links and tags connect across them.</p>
        </div>
        {spaces.length === 0 ? (
          <div className="empty-state">
            <h3>No public knowledge yet</h3>
            <p>The owner has not published a Knowledge Space.</p>
          </div>
        ) : (
          <ol className="space-list">
            {spaces.map((space, index) => (
              <li key={space.id}>
                <span className="space-number">{String(index + 1).padStart(2, "0")}</span>
                <Link href={`/spaces/${space.id}`}>
                  <span>{space.name}</span>
                  <ArrowRight size={19} />
                </Link>
              </li>
            ))}
          </ol>
        )}
      </section>
      <footer className="site-footer"><span>NoteFoundry</span><span>Knowledge is published deliberately.</span></footer>
    </main>
  );
}
