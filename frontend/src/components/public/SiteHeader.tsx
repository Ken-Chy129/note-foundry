import Link from "next/link";
import { NoteFoundryLogo } from "@/components/brand/NoteFoundryLogo";
import { PublicSearch } from "@/components/public/PublicSearch";

export function SiteHeader() {
  return (
    <header className="site-header">
      <div className="site-header-inner">
        <Link className="wordmark" href="/" aria-label="NoteFoundry 首页">
          <NoteFoundryLogo />
          <span>NoteFoundry</span>
        </Link>
        <nav className="public-navigation" aria-label="公开知识库导航">
          <span>公开知识库</span>
          <Link href="/#spaces">浏览空间</Link>
          <PublicSearch />
        </nav>
      </div>
    </header>
  );
}
