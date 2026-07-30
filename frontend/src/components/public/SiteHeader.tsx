import Link from "next/link";
import { NoteFoundryLogo } from "@/components/brand/NoteFoundryLogo";

export function SiteHeader() {
  return (
    <header className="site-header">
      <Link className="wordmark" href="/" aria-label="NoteFoundry 首页">
        <NoteFoundryLogo />
        <span>NoteFoundry</span>
      </Link>
    </header>
  );
}
