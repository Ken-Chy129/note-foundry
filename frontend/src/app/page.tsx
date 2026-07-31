import Link from "next/link";
import { ArrowUpRight, BookOpen } from "lucide-react";
import { SiteHeader } from "@/components/public/SiteHeader";
import { publicApiFetch } from "@/lib/public-api";

export const dynamic = "force-dynamic";
import type { KnowledgeSpace, PageResponse } from "@/lib/types";

export default async function HomePage() {
  const response = await publicApiFetch<PageResponse<KnowledgeSpace>>("/api/v1/public/spaces?page=1&pageSize=50");
  const spaces = response?.data ?? [];

  return (
    <main className="public-site">
      <SiteHeader />
      <div className="public-home">
        <section className="home-hero" aria-labelledby="home-title">
          <div className="home-hero-intro">
            <p className="home-hero-kicker">NoteFoundry / Public</p>
            <h1 id="home-title">公开知识库</h1>
            <p>按知识空间浏览长期整理的技术笔记，需要时再搜索一个具体问题。</p>
          </div>
        </section>

        <section className="space-index" id="spaces" aria-labelledby="spaces-title">
          <header className="space-index-header">
            <div>
              <p>知识结构</p>
              <h2 id="spaces-title">知识空间</h2>
            </div>
            <p><strong>{spaces.length}</strong> 个公开空间</p>
          </header>
          {spaces.length === 0 ? (
            <div className="empty-state" role="status">
              <BookOpen aria-hidden="true" size={22} strokeWidth={1.7} />
              <div>
                <h3>还没有公开内容</h3>
                <p>发布后的知识空间会显示在这里。</p>
              </div>
            </div>
          ) : (
            <ul className="space-shelf">
              {spaces.map((space) => (
                <li key={space.id}>
                  <Link href={`/spaces/${space.id}`}>
                    <span className="space-shelf-label">知识空间</span>
                    <strong>{space.name}</strong>
                    <span className="space-shelf-action">浏览目录 <ArrowUpRight aria-hidden="true" size={18} /></span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </section>
        <footer className="site-footer"><span>NoteFoundry</span><span>持续整理的个人技术知识</span></footer>
      </div>
    </main>
  );
}
