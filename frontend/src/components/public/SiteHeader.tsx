import Link from "next/link";

export function SiteHeader() {
  return (
    <header className="site-header">
      <Link className="wordmark" href="/" aria-label="NoteFoundry 首页">
        <span className="wordmark-mark">NF</span>
        <span>NoteFoundry</span>
      </Link>
    </header>
  );
}
