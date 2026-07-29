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
          <p className="section-kicker">可靠沉淀每一份理解</p>
          <h1>学习笔记，<br /><em>历久成章。</em></h1>
          <p className="hero-summary">
            NoteFoundry 是个人学习工作台，让经过确认的 Markdown 成为持久、可链接的知识沉淀，而不是又一个资料收藏夹。
          </p>
          <div className="hero-actions">
            <a className="button button-primary" href="#spaces">浏览公开知识</a>
            <Link className="button button-quiet" href="/app">进入所有者工作区 <ArrowRight size={17} /></Link>
          </div>
        </div>
        <aside className="hero-principles" aria-label="工作台原则">
          <div><BookOpenText size={18} /><span>Markdown 是规范格式</span></div>
          <div><LockKeyhole size={18} /><span>笔记草稿始终私有</span></div>
          <p>每一篇公开笔记都具有稳定身份、可恢复历史，并经过知识所有者明确发布。</p>
        </aside>
      </section>

      <PublicSearch />

      <section className="space-index" id="spaces" aria-labelledby="spaces-title">
        <div className="section-heading">
          <div>
            <p className="section-kicker">公开索引</p>
            <h2 id="spaces-title">知识空间</h2>
          </div>
          <p>按长期学习领域组织内容；空间内使用目录分层，空间之间通过笔记链接和标签建立联系。</p>
        </div>
        {spaces.length === 0 ? (
          <div className="empty-state">
            <h3>暂时没有公开知识</h3>
            <p>知识所有者尚未发布任何知识空间。</p>
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
      <footer className="site-footer"><span>NoteFoundry</span><span>每一次公开，都经过明确确认。</span></footer>
    </main>
  );
}
