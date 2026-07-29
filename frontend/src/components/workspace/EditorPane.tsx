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
        <p className="section-kicker">Owner workspace</p>
        <h1>Select a Learning Note</h1>
        <p>Choose a note from the knowledge tree, or create a new one. Autosave begins as soon as you write.</p>
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
    if (!window.confirm(`Move “${title}” to Trash? Its stable identity and history will be preserved.`)) return;
    await onTrash(note.id, versionRef.current);
  }

  return (
    <section className="editor-pane">
      <header className="editor-toolbar">
        <div className={`save-indicator save-${saveState}`} aria-live="polite">
          {saveState === "saving" ? <Save size={14} /> : <Check size={14} />}
          {saveState === "saved" ? "Saved" : saveState === "dirty" ? "Unsaved changes" : saveState === "saving" ? "Saving…" : saveState === "conflict" ? "Save conflict — reload" : "Save failed"}
        </div>
        <div className="view-switcher" aria-label="Editor view">
          <button className={mode === "edit" ? "is-active" : ""} onClick={() => setMode("edit")}>Edit</button>
          <button className={mode === "split" ? "is-active" : ""} onClick={() => setMode("split")} aria-label="Split editor and preview"><Columns2 size={15} /></button>
          <button className={mode === "preview" ? "is-active" : ""} onClick={() => setMode("preview")} aria-label="Preview"><Eye size={15} /></button>
        </div>
        <button className="button button-quiet toolbar-trash" onClick={trash}><Trash2 size={15} /> Trash</button>
        <button className="button button-primary" onClick={() => void publish()} disabled={saveState === "saving" || saveState === "conflict"}><Send size={15} /> Publish</button>
      </header>
      <input
        className="note-title-input"
        value={title}
        onChange={(event) => setTitle(event.target.value)}
        aria-label="Learning Note title"
      />
      <div className={`editor-grid mode-${mode}`}>
        {mode !== "preview" && (
          <textarea
            className="markdown-editor"
            value={markdown}
            onChange={(event) => setMarkdown(event.target.value)}
            spellCheck
            aria-label="Canonical Markdown editor"
          />
        )}
        {mode !== "edit" && (
          <div className="editor-preview" aria-label="Markdown preview">
            <MarkdownRenderer markdown={markdown} attachmentScope="owner" />
          </div>
        )}
      </div>
    </section>
  );
}
