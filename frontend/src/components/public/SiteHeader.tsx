import Link from "next/link";

export function SiteHeader() {
  return (
    <header className="site-header">
      <Link className="wordmark" href="/" aria-label="NoteFoundry home">
        <span className="wordmark-mark">NF</span>
        <span>NoteFoundry</span>
      </Link>
      <nav aria-label="Primary navigation">
        <Link href="/#spaces">Knowledge spaces</Link>
        <Link href="/app">Owner workspace</Link>
      </nav>
    </header>
  );
}
