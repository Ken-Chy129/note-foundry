import Link from "next/link";

export function SiteHeader() {
  return (
    <header className="site-header">
      <Link className="wordmark" href="/" aria-label="NoteFoundry 首页">
        <span className="wordmark-mark">NF</span>
        <span>NoteFoundry</span>
      </Link>
      <nav aria-label="主导航">
        <Link href="/#spaces">知识空间</Link>
        <Link href="/app">所有者工作区</Link>
      </nav>
    </header>
  );
}
