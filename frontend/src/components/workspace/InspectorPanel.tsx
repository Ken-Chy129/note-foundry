"use client";

import { Clipboard, FileUp, History, Link2, RotateCcw, Tag as TagIcon } from "lucide-react";
import type { ChangeEvent } from "react";
import type { Attachment, LearningNote, LinkedNote, Tag } from "@/lib/types";

export interface RevisionSummary {
  id: string;
  noteId: string;
  title: string;
  reason: "publish" | "restore" | "manual";
  createdAt: string;
}

interface InspectorPanelProps {
  note: LearningNote | null;
  tags: Tag[];
  selectedTagIds: Set<string>;
  attachments: Attachment[];
  backlinks: LinkedNote[];
  revisions: RevisionSummary[];
  onToggleTag: (tagId: string) => void;
  onUpload: (file: File) => void;
  onRestoreRevision: (revisionId: string) => void;
  onCreateTag: () => void;
}

export function InspectorPanel(props: InspectorPanelProps) {
  if (!props.note) {
    return <aside className="inspector-panel inspector-empty">Details appear when a note is selected.</aside>;
  }
  const stableLink = `note:${props.note.id}`;

  function upload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (file) props.onUpload(file);
    event.target.value = "";
  }

  return (
    <aside className="inspector-panel" aria-label="Learning Note details">
      <section className="inspector-section">
        <h2><Link2 size={15} /> Stable link</h2>
        <button className="copy-value" onClick={() => navigator.clipboard.writeText(stableLink)} title="Copy stable Note Link">
          <code>{stableLink}</code><Clipboard size={14} />
        </button>
        <p>Use in Markdown as <code>[label]({stableLink})</code>.</p>
      </section>

      <section className="inspector-section">
        <h2><TagIcon size={15} /> Tags <button onClick={props.onCreateTag}>New</button></h2>
        <div className="tag-picker">
          {props.tags.length === 0 && <p>No global Tags yet.</p>}
          {props.tags.map((tag) => (
            <button key={tag.id} className={props.selectedTagIds.has(tag.id) ? "is-active" : ""} onClick={() => props.onToggleTag(tag.id)}>{tag.name}</button>
          ))}
        </div>
      </section>

      <section className="inspector-section">
        <h2><FileUp size={15} /> Attachments</h2>
        <label className="attachment-upload">
          <input type="file" onChange={upload} />
          Upload file
        </label>
        <div className="attachment-list">
          {props.attachments.map((attachment) => (
            <div key={attachment.id}>
              <a href={`/api/v1/attachments/${attachment.id}`} target="_blank" rel="noreferrer">{attachment.originalName}</a>
              <button onClick={() => navigator.clipboard.writeText(`attachment:${attachment.id}`)}>Copy URI</button>
            </div>
          ))}
        </div>
      </section>

      <section className="inspector-section">
        <h2><Link2 size={15} /> Backlinks</h2>
        {props.backlinks.length === 0 ? <p>No notes link here yet.</p> : props.backlinks.map((note) => <p key={note.id}>{note.title}</p>)}
      </section>

      <section className="inspector-section">
        <h2><History size={15} /> Revisions</h2>
        <div className="revision-list">
          {props.revisions.slice(0, 8).map((revision) => (
            <div key={revision.id}>
              <span><strong>{revision.reason}</strong>{new Date(revision.createdAt).toLocaleString()}</span>
              <button onClick={() => props.onRestoreRevision(revision.id)} aria-label={`Restore revision from ${revision.createdAt}`}><RotateCcw size={14} /></button>
            </div>
          ))}
          {props.revisions.length === 0 && <p>No checkpoints yet. Publishing creates one.</p>}
        </div>
      </section>
    </aside>
  );
}
