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

const revisionReasonLabels: Record<RevisionSummary["reason"], string> = {
  publish: "发布",
  restore: "恢复",
  manual: "手动保存"
};

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
    return <aside className="inspector-panel inspector-empty">选择学习笔记后，这里会显示详细信息。</aside>;
  }
  const stableLink = `note:${props.note.id}`;

  function upload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (file) props.onUpload(file);
    event.target.value = "";
  }

  return (
    <aside className="inspector-panel" aria-label="学习笔记详情">
      <section className="inspector-section">
        <h2><Link2 size={15} /> 稳定链接</h2>
        <button className="copy-value" onClick={() => navigator.clipboard.writeText(stableLink)} title="复制稳定笔记链接">
          <code>{stableLink}</code><Clipboard size={14} />
        </button>
        <p>在 Markdown 中使用：<code>[链接文字]({stableLink})</code>。</p>
      </section>

      <section className="inspector-section">
        <h2><TagIcon size={15} /> 标签 <button onClick={props.onCreateTag}>新建</button></h2>
        <div className="tag-picker">
          {props.tags.length === 0 && <p>暂时没有全局标签。</p>}
          {props.tags.map((tag) => (
            <button key={tag.id} className={props.selectedTagIds.has(tag.id) ? "is-active" : ""} onClick={() => props.onToggleTag(tag.id)}>{tag.name}</button>
          ))}
        </div>
      </section>

      <section className="inspector-section">
        <h2><FileUp size={15} /> 附件</h2>
        <label className="attachment-upload">
          <input type="file" onChange={upload} />
          上传文件
        </label>
        <div className="attachment-list">
          {props.attachments.map((attachment) => (
            <div key={attachment.id}>
              <a href={`/api/v1/attachments/${attachment.id}`} target="_blank" rel="noreferrer">{attachment.originalName}</a>
              <button onClick={() => navigator.clipboard.writeText(`attachment:${attachment.id}`)}>复制 URI</button>
            </div>
          ))}
        </div>
      </section>

      <section className="inspector-section">
        <h2><Link2 size={15} /> 反向链接</h2>
        {props.backlinks.length === 0 ? <p>暂时没有其他笔记链接到这里。</p> : props.backlinks.map((note) => <p key={note.id}>{note.title}</p>)}
      </section>

      <section className="inspector-section">
        <h2><History size={15} /> 修订记录</h2>
        <div className="revision-list">
          {props.revisions.slice(0, 8).map((revision) => (
            <div key={revision.id}>
              <span><strong>{revisionReasonLabels[revision.reason]}</strong>{new Date(revision.createdAt).toLocaleString("zh-CN")}</span>
              <button onClick={() => props.onRestoreRevision(revision.id)} aria-label={`恢复 ${new Date(revision.createdAt).toLocaleString("zh-CN")} 的修订`}><RotateCcw size={14} /></button>
            </div>
          ))}
          {props.revisions.length === 0 && <p>暂时没有修订记录，发布时会自动创建。</p>}
        </div>
      </section>
    </aside>
  );
}
