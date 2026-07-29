"use client";

import { Check, Columns2, Eye, Save, Send, Trash2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { MarkdownRenderer } from "@/components/markdown/MarkdownRenderer";
import type { LearningNote } from "@/lib/types";

type SaveState = "saved" | "dirty" | "saving" | "conflict" | "error";

interface EditorPaneProps {
  note: LearningNote | null;
  onAutosave: (id: string, version: number, title: string, markdown: string) => Promise<LearningNote>;
  onPublish: (id: string, version: number) => Promise<LearningNote>;
  onTrash: (id: string, version: number) => Promise<void>;
  onNoteChange: (note: LearningNote) => void;
}

export function EditorPane({ note, onAutosave, onPublish, onTrash, onNoteChange }: EditorPaneProps) {
  const [title, setTitle] = useState(note?.title ?? "");
  const [markdown, setMarkdown] = useState(note?.markdown ?? "");
  const [mode, setMode] = useState<"edit" | "split" | "preview">("split");
  const [saveState, setSaveState] = useState<SaveState>("saved");
  const versionRef = useRef(note?.version ?? 1);
  const savedRef = useRef({ title: note?.title ?? "", markdown: note?.markdown ?? "" });

  const save = useCallback(async () => {
    if (!note || !title.trim()) return note;
    if (title === savedRef.current.title && markdown === savedRef.current.markdown) return note;
    setSaveState("saving");
    try {
      const updated = await onAutosave(note.id, versionRef.current, title, markdown);
      versionRef.current = updated.version;
      savedRef.current = { title: updated.title, markdown: updated.markdown };
      setSaveState("saved");
      onNoteChange(updated);
      return updated;
    } catch (error) {
      const conflict = error instanceof Error && error.message.includes("changed by another save");
      setSaveState(conflict ? "conflict" : "error");
      throw error;
    }
  }, [markdown, note, onAutosave, onNoteChange, title]);

  useEffect(() => {
    if (!note || (title === savedRef.current.title && markdown === savedRef.current.markdown)) return;
    setSaveState("dirty");
    const timer = window.setTimeout(() => void save().catch(() => undefined), 900);
    return () => window.clearTimeout(timer);
  }, [markdown, note, save, title]);

  if (!note) {
    return (
      <section className="editor-empty">
        <span className="empty-spine" />
        <p className="section-kicker">所有者工作区</p>
        <h1>选择一篇学习笔记</h1>
        <p>从左侧知识树选择一篇笔记，或创建新笔记。开始输入后，系统会自动保存。</p>
      </section>
    );
  }

  async function publish() {
    const saved = await save();
    if (!saved) return;
    const published = await onPublish(saved.id, versionRef.current);
    onNoteChange(published);
  }

  async function trash() {
    if (!note) return;
    if (!window.confirm(`将《${title}》移入回收站吗？笔记的稳定身份与历史记录会保留。`)) return;
    await onTrash(note.id, versionRef.current);
  }

  return (
    <section className="editor-pane">
      <header className="editor-toolbar">
        <div className={`save-indicator save-${saveState}`} aria-live="polite">
          {saveState === "saving" ? <Save size={14} /> : <Check size={14} />}
          {saveState === "saved" ? "已保存" : saveState === "dirty" ? "有未保存更改" : saveState === "saving" ? "保存中…" : saveState === "conflict" ? "保存冲突，请刷新页面" : "保存失败"}
        </div>
        <div className="view-switcher" aria-label="编辑器视图">
          <button className={mode === "edit" ? "is-active" : ""} onClick={() => setMode("edit")}>编辑</button>
          <button className={mode === "split" ? "is-active" : ""} onClick={() => setMode("split")} aria-label="并排显示编辑器和预览"><Columns2 size={15} /></button>
          <button className={mode === "preview" ? "is-active" : ""} onClick={() => setMode("preview")} aria-label="预览"><Eye size={15} /></button>
        </div>
        <button className="button button-quiet toolbar-trash" onClick={trash}><Trash2 size={15} /> 移入回收站</button>
        <button className="button button-primary" onClick={() => void publish()} disabled={saveState === "saving" || saveState === "conflict"}><Send size={15} /> 发布</button>
      </header>
      <input
        className="note-title-input"
        value={title}
        onChange={(event) => setTitle(event.target.value)}
        aria-label="学习笔记标题"
      />
      <div className={`editor-grid mode-${mode}`}>
        {mode !== "preview" && (
          <textarea
            className="markdown-editor"
            value={markdown}
            onChange={(event) => setMarkdown(event.target.value)}
            spellCheck
            aria-label="规范 Markdown 编辑器"
          />
        )}
        {mode !== "edit" && (
          <div className="editor-preview" aria-label="Markdown 预览">
            <MarkdownRenderer markdown={markdown} attachmentScope="owner" />
          </div>
        )}
      </div>
    </section>
  );
}
