"use client";

import { BookOpen, ChevronRight, FolderPlus, Lock, Plus, Trash2, Unlock } from "lucide-react";
import type { Directory, KnowledgeSpace, LearningNote } from "@/lib/types";

interface KnowledgeSidebarProps {
  spaces: KnowledgeSpace[];
  directories: Directory[];
  notes: LearningNote[];
  selectedSpaceId: string | null;
  selectedNoteId: string | null;
  onSelectSpace: (id: string) => void;
  onSelectNote: (note: LearningNote) => void;
  onCreateSpace: () => void;
  onCreateDirectory: () => void;
  onCreateNote: () => void;
  onOpenTrash: () => void;
}

function notePath(note: LearningNote, directories: Directory[]): string {
  if (!note.directoryId) return "根目录";
  return directories.find((directory) => directory.id === note.directoryId)?.name ?? "目录";
}

export function KnowledgeSidebar(props: KnowledgeSidebarProps) {
  return (
    <aside className="workspace-sidebar" aria-label="知识导航">
      <div className="workspace-brand"><span>NF</span><strong>NoteFoundry</strong></div>
      <div className="sidebar-heading">
        <span>知识空间</span>
        <button className="icon-button" onClick={props.onCreateSpace} aria-label="创建知识空间"><Plus size={17} /></button>
      </div>
      <div className="space-switcher" role="list">
        {props.spaces.map((space) => (
          <button
            role="listitem"
            key={space.id}
            className={space.id === props.selectedSpaceId ? "is-active" : ""}
            onClick={() => props.onSelectSpace(space.id)}
          >
            {space.visibility === "public" ? <Unlock size={14} /> : <Lock size={14} />}
            <span>{space.name}</span>
            <ChevronRight size={14} />
          </button>
        ))}
      </div>

      <div className="sidebar-heading sidebar-heading-notes">
        <span>学习笔记</span>
        <div>
          <button className="icon-button" onClick={props.onCreateDirectory} aria-label="创建目录" disabled={!props.selectedSpaceId}><FolderPlus size={16} /></button>
          <button className="icon-button" onClick={props.onCreateNote} aria-label="创建学习笔记" disabled={!props.selectedSpaceId}><Plus size={17} /></button>
        </div>
      </div>
      <div className="note-navigation" role="list">
        {props.notes.length === 0 && <p className="sidebar-empty">在这个空间中创建第一篇学习笔记。</p>}
        {props.notes.map((note) => (
          <button
            role="listitem"
            key={note.id}
            className={note.id === props.selectedNoteId ? "is-active" : ""}
            onClick={() => props.onSelectNote(note)}
          >
            <BookOpen size={15} />
            <span><strong>{note.title}</strong><small>{notePath(note, props.directories)}</small></span>
            {note.published && <i title="已发布" />}
          </button>
        ))}
      </div>
      <button className="trash-link" onClick={props.onOpenTrash}><Trash2 size={15} /> 回收站</button>
    </aside>
  );
}
