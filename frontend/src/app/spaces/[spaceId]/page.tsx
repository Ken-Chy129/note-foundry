import Link from "next/link";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { notFound } from "next/navigation";
import { SiteHeader } from "@/components/public/SiteHeader";
import { publicApiFetch } from "@/lib/public-api";

export const dynamic = "force-dynamic";
import type { DataResponse, Directory, KnowledgeSpace, PageResponse, PublishedNote } from "@/lib/types";

function directoryPath(directoryId: string | null, directories: Directory[]): string {
  if (!directoryId) return "根目录";
  const byId = new Map(directories.map((directory) => [directory.id, directory]));
  const names: string[] = [];
  let current = byId.get(directoryId);
  while (current) {
    names.unshift(current.name);
    current = current.parentId ? byId.get(current.parentId) : undefined;
  }
  return names.join(" / ") || "根目录";
}

export default async function SpacePage({ params }: { params: Promise<{ spaceId: string }> }) {
  const { spaceId } = await params;
  const [spacesResponse, directoriesResponse, notesResponse] = await Promise.all([
    publicApiFetch<PageResponse<KnowledgeSpace>>("/api/v1/public/spaces?page=1&pageSize=100"),
    publicApiFetch<DataResponse<Directory>>(`/api/v1/public/spaces/${spaceId}/directories`),
    publicApiFetch<PageResponse<PublishedNote>>(`/api/v1/public/spaces/${spaceId}/notes?page=1&pageSize=100`)
  ]);
  const space = spacesResponse?.data.find((item) => item.id === spaceId);
  if (!space || !directoriesResponse || !notesResponse) notFound();
  const directories = directoriesResponse.data;

  return (
    <main>
      <SiteHeader />
      <section className="space-page-header">
        <Link className="back-link" href="/"><ArrowLeft size={16} /> 全部知识空间</Link>
        <p className="section-kicker">公开知识空间</p>
        <h1>{space.name}</h1>
        <p>已发布 {notesResponse.pagination.totalItems} 篇学习笔记</p>
      </section>
      <section className="note-index" aria-label={`${space.name}中的学习笔记`}>
        {notesResponse.data.length === 0 ? (
          <div className="empty-state"><h2>这里还没有已发布笔记</h2><p>这个知识空间已经公开，第一篇学习笔记仍在准备中。</p></div>
        ) : (
          notesResponse.data.map((note, index) => (
            <article className="note-index-row" key={note.id}>
              <span className="note-index-marker">{String(index + 1).padStart(2, "0")}</span>
              <div>
                <p className="directory-path">{directoryPath(note.directoryId, directories)}</p>
                <h2><Link href={`/notes/${note.id}/${note.slug}`}>{note.title}</Link></h2>
                <p className="note-excerpt">{note.markdown.replace(/[#*`>\[\]]/g, "").slice(0, 180)}</p>
              </div>
              <Link className="round-link" aria-label={`阅读《${note.title}》`} href={`/notes/${note.id}/${note.slug}`}><ArrowRight size={18} /></Link>
            </article>
          ))
        )}
      </section>
    </main>
  );
}
