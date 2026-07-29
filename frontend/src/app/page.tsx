import Link from "next/link";
import { ArrowRight, BookOpen } from "lucide-react";
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
      <div className="public-home">
        <section className="home-hero" aria-labelledby="home-title">
          <div className="home-hero-copy">
            <h1 id="home-title">学习笔记</h1>
            <p>整理长期学习中值得留下的理解与实践。</p>
          </div>
          <PublicSearch />
        </section>

        <section className="space-index" id="spaces" aria-labelledby="spaces-title">
          <div className="section-heading">
            <h2 id="spaces-title">知识空间</h2>
            <p>{spaces.length} 个公开空间</p>
          </div>
          {spaces.length === 0 ? (
            <div className="empty-state" role="status">
              <BookOpen aria-hidden="true" size={22} strokeWidth={1.7} />
              <div>
                <h3>还没有公开内容</h3>
                <p>发布后的知识空间会显示在这里。</p>
              </div>
            </div>
          ) : (
            <ul className="space-list">
              {spaces.map((space) => (
                <li key={space.id}>
                  <Link href={`/spaces/${space.id}`}>
                    <span>{space.name}</span>
                    <ArrowRight aria-hidden="true" size={19} />
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </section>
        <footer className="site-footer">NoteFoundry · 个人学习笔记</footer>
      </div>
    </main>
  );
}
