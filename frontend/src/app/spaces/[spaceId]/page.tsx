import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { notFound } from "next/navigation";
import { PublicSpaceDirectoryNav } from "@/components/public/PublicSpaceDirectoryNav";
import { PublicSpaceDirectorySection, PublicSpaceNoteList } from "@/components/public/PublicSpaceDirectorySection";
import { buildPublicSpaceIndex } from "@/components/public/publicSpaceIndex";
import { SiteHeader } from "@/components/public/SiteHeader";
import { publicApiFetch } from "@/lib/public-api";

export const dynamic = "force-dynamic";
import type { DataResponse, Directory, KnowledgeSpace, PageResponse, PublishedNote } from "@/lib/types";

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
  const index = buildPublicSpaceIndex(directories, notesResponse.data);
  const directoryCount = directories.length;
  const noteCount = notesResponse.pagination.totalItems;

  return (
    <main className="public-space-page">
      <SiteHeader />
      <div className="public-space-shell">
        <Link className="back-link" href="/"><ArrowLeft size={16} /> 全部知识空间</Link>
        <header className="public-space-masthead">
          <p className="space-location"><span>知识空间</span><i>/</i><strong>公开</strong></p>
          <h1>{space.name}</h1>
          <p className="space-facts">
            <span>{noteCount} 篇笔记</span>
            <span>{directoryCount} 个目录</span>
          </p>
        </header>

        {noteCount === 0 ? (
          <div className="public-space-empty">
            <p>知识索引</p>
            <h2>这里还没有公开笔记</h2>
            <span>发布后的学习笔记会按目录整理在这里</span>
          </div>
        ) : (
          <>
            <details className="mobile-space-directory">
              <summary>空间目录 <span>{directoryCount}</span></summary>
              <PublicSpaceDirectoryNav directories={index.directories} rootNoteCount={index.rootNotes.length} />
            </details>

            <div className="public-space-layout">
              <aside className="space-directory-rail">
                <PublicSpaceDirectoryNav directories={index.directories} rootNoteCount={index.rootNotes.length} />
              </aside>

              <section className="space-directory-content" aria-label={`${space.name}中的学习笔记`}>
                <header className="space-index-heading">
                  <div>
                    <p>知识索引</p>
                    <h2>按目录浏览</h2>
                  </div>
                  <p>沿着主题结构查找笔记，而不是翻阅一条时间流</p>
                </header>

                {index.rootNotes.length > 0 && (
                  <section className="space-directory-section" id="未归类" data-depth="0">
                    <header className="space-directory-section-heading">
                      <h3>未归类</h3>
                      <span>{index.rootNotes.length} 篇</span>
                    </header>
                    <PublicSpaceNoteList notes={index.rootNotes} />
                  </section>
                )}

                {index.directories.map((node) => (
                  <PublicSpaceDirectorySection key={node.directory.id} node={node} />
                ))}
              </section>
            </div>
          </>
        )}
      </div>
    </main>
  );
}
