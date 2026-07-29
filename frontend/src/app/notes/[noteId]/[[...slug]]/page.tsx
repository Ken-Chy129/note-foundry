import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft, ArrowUpRight, Link2 } from "lucide-react";
import { notFound } from "next/navigation";
import { MarkdownRenderer } from "@/components/markdown/MarkdownRenderer";
import { SiteHeader } from "@/components/public/SiteHeader";
import { publicApiFetch } from "@/lib/public-api";

export const dynamic = "force-dynamic";
import type { DataResponse, LinkedNote, PublishedNote } from "@/lib/types";

type NoteParams = Promise<{ noteId: string; slug?: string[] }>;

export async function generateMetadata({ params }: { params: NoteParams }): Promise<Metadata> {
  const { noteId } = await params;
  const note = await publicApiFetch<PublishedNote>(`/api/v1/public/notes/${noteId}`);
  return note ? { title: note.title, description: note.markdown.replace(/[#*`>\[\]]/g, "").slice(0, 155) } : { title: "未找到学习笔记" };
}

export default async function PublicNotePage({ params }: { params: NoteParams }) {
  const { noteId } = await params;
  const [note, links, backlinks] = await Promise.all([
    publicApiFetch<PublishedNote>(`/api/v1/public/notes/${noteId}`),
    publicApiFetch<DataResponse<LinkedNote>>(`/api/v1/public/notes/${noteId}/links`),
    publicApiFetch<DataResponse<LinkedNote>>(`/api/v1/public/notes/${noteId}/backlinks`)
  ]);
  if (!note) notFound();

  return (
    <main>
      <SiteHeader />
      <div className="reader-shell">
        <aside className="reader-margin">
          <Link className="back-link" href={`/spaces/${note.spaceId}`}><ArrowLeft size={16} /> 返回知识空间</Link>
          <div className="reader-status"><span /> 已发布<br />{new Date(note.publishedAt).toLocaleDateString("zh-CN", { year: "numeric", month: "long", day: "numeric" })}</div>
        </aside>
        <article className="reader-main">
          <header className="reader-title">
            <p className="section-kicker">学习笔记</p>
            <h1>{note.title}</h1>
          </header>
          <MarkdownRenderer markdown={note.markdown} />
        </article>
        <aside className="reader-relations" aria-label="笔记关系">
          <RelationList title="链接到" notes={links?.data ?? []} />
          <RelationList title="反向链接" notes={backlinks?.data ?? []} backlinks />
        </aside>
      </div>
    </main>
  );
}

function RelationList({ title, notes, backlinks = false }: { title: string; notes: LinkedNote[]; backlinks?: boolean }) {
  return (
    <section className="relation-list">
      <h2>{backlinks ? <Link2 size={15} /> : <ArrowUpRight size={15} />}{title}</h2>
      {notes.length === 0 ? <p>暂无。</p> : notes.map((note) => (
        <Link key={note.id} href={`/notes/${note.id}/${note.slug}`}>{note.title}</Link>
      ))}
    </section>
  );
}
