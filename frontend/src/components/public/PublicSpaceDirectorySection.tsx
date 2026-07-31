import Link from "next/link";
import type { PublishedNote } from "@/lib/types";
import type { PublicSpaceDirectoryNode } from "./publicSpaceIndex";

function noteExcerpt(markdown: string): string {
  return markdown
    .replace(/!\[[^\]]*\]\([^)]*\)/g, " ")
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .replace(/[#>*_`~|=-]/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .slice(0, 118);
}

function publishedDate(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit"
  }).format(new Date(value));
}

export function PublicSpaceNoteList({ notes }: { notes: PublishedNote[] }) {
  if (notes.length === 0) return null;

  return (
    <ol className="space-note-list">
      {notes.map((note) => {
        const excerpt = noteExcerpt(note.markdown);
        return (
          <li className="space-note-entry" key={note.id}>
            <Link href={`/notes/${note.id}/${note.slug}`}>
              <span className="space-note-copy">
                <strong>{note.title}</strong>
                {excerpt && <span>{excerpt}</span>}
              </span>
              <time dateTime={note.publishedAt}>{publishedDate(note.publishedAt)}</time>
            </Link>
          </li>
        );
      })}
    </ol>
  );
}

export function PublicSpaceDirectorySection({
  node,
  depth = 0
}: {
  node: PublicSpaceDirectoryNode;
  depth?: number;
}) {
  const Heading = depth === 0 ? "h3" : depth === 1 ? "h4" : "h5";

  return (
    <section className="space-directory-section" id={node.sectionId} data-depth={depth}>
      <header className="space-directory-section-heading">
        <div>
          {depth > 0 && <p>{node.path.slice(0, -1).join(" / ")}</p>}
          <Heading>{node.directory.name}</Heading>
        </div>
        <span>{node.totalNotes} 篇</span>
      </header>
      <PublicSpaceNoteList notes={node.notes} />
      {node.children.length > 0 && (
        <div className="space-directory-children">
          {node.children.map((child) => (
            <PublicSpaceDirectorySection key={child.directory.id} node={child} depth={depth + 1} />
          ))}
        </div>
      )}
    </section>
  );
}
