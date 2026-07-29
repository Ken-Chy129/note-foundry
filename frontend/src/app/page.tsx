import Link from "next/link";
import { ArrowRight } from "lucide-react";
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
      <section className="home-intro">
        <header className="home-intro-meta">
          <p className="section-kicker">个人公开知识库</p>
          <p className="catalog-count"><span>公开空间</span><strong>{String(spaces.length).padStart(2, "0")}</strong></p>
        </header>
        <div className="home-intro-grid">
          <h1>把<em>理解</em>写下来。</h1>
          <div className="home-intro-copy">
            <p>学习笔记按知识空间整理，支持中文与英文检索。这里展示的每一篇内容，都经过确认后明确发布。</p>
            <dl>
              <div><dt>内容</dt><dd>学习笔记</dd></div>
              <div><dt>组织</dt><dd>知识空间</dd></div>
              <div><dt>格式</dt><dd>Markdown</dd></div>
            </dl>
          </div>
        </div>
      </section>

      <PublicSearch />

      <section className="space-index" id="spaces" aria-labelledby="spaces-title">
        <div className="section-heading">
          <div>
            <p className="section-kicker">按领域浏览</p>
            <h2 id="spaces-title">知识空间</h2>
          </div>
          <p>共 {spaces.length} 个公开空间</p>
        </div>
        {spaces.length === 0 ? (
          <div className="empty-state">
            <h3>暂时没有公开知识</h3>
            <p>知识所有者尚未发布任何知识空间。</p>
          </div>
        ) : (
          <ul className="space-list">
            {spaces.map((space) => (
              <li key={space.id}>
                <Link href={`/spaces/${space.id}`}>
                  <span>{space.name}</span>
                  <ArrowRight size={19} />
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
      <footer className="site-footer"><span>NoteFoundry</span><span>每一次公开，都经过明确确认。</span></footer>
    </main>
  );
}
