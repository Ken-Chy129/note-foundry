import Link from "next/link";
import { ArrowLeft } from "lucide-react";
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

function noteExcerpt(markdown: string): string {
  return markdown
    .replace(/!\[[^\]]*\]\([^)]*\)/g, " ")
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .replace(/[#>*_`~|=-]/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .slice(0, 150);
}

function publishedDate(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "long",
    day: "numeric"
  }).format(new Date(value));
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
        <div className="space-page-heading">
          <div>
            <p className="section-kicker">公开知识空间</p>
            <h1>{space.name}</h1>
          </div>
          <p className="space-note-count">
            <strong>{notesResponse.pagination.totalItems}</strong>
            <span>篇公开笔记</span>
          </p>
        </div>
      </section>
      <section className="space-note-catalog" aria-label={`${space.name}中的学习笔记`}>
        <header>
          <h2>文章</h2>
          <p>按最近发布时间排序</p>
        </header>
        {notesResponse.data.length === 0 ? (
          <div className="empty-state"><div><h3>这里还没有公开笔记</h3><p>发布后的学习笔记会出现在这里。</p></div></div>
        ) : (
          notesResponse.data.map((note) => {
            const excerpt = noteExcerpt(note.markdown);
            return (
              <article className="space-note-row" key={note.id}>
                <Link href={`/notes/${note.id}/${note.slug}`}>
                  <p className="space-note-meta">
                    <span>{directoryPath(note.directoryId, directories)}</span>
                    <time dateTime={note.publishedAt}>{publishedDate(note.publishedAt)}</time>
                  </p>
                  <h2>{note.title}</h2>
                  {excerpt && <p className="note-excerpt">{excerpt}</p>}
                </Link>
              </article>
            );
          })
        )}
      </section>
    </main>
  );
}
