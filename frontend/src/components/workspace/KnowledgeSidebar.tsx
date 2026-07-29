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
  if (!note.directoryId) return "Root";
  return directories.find((directory) => directory.id === note.directoryId)?.name ?? "Directory";
}

export function KnowledgeSidebar(props: KnowledgeSidebarProps) {
  return (
    <aside className="workspace-sidebar" aria-label="Knowledge navigation">
      <div className="workspace-brand"><span>NF</span><strong>NoteFoundry</strong></div>
      <div className="sidebar-heading">
        <span>Knowledge spaces</span>
        <button className="icon-button" onClick={props.onCreateSpace} aria-label="Create Knowledge Space"><Plus size={17} /></button>
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
        <span>Learning notes</span>
        <div>
          <button className="icon-button" onClick={props.onCreateDirectory} aria-label="Create directory" disabled={!props.selectedSpaceId}><FolderPlus size={16} /></button>
          <button className="icon-button" onClick={props.onCreateNote} aria-label="Create Learning Note" disabled={!props.selectedSpaceId}><Plus size={17} /></button>
        </div>
      </div>
      <div className="note-navigation" role="list">
        {props.notes.length === 0 && <p className="sidebar-empty">Create the first note in this space.</p>}
        {props.notes.map((note) => (
          <button
            role="listitem"
            key={note.id}
            className={note.id === props.selectedNoteId ? "is-active" : ""}
            onClick={() => props.onSelectNote(note)}
          >
            <BookOpen size={15} />
            <span><strong>{note.title}</strong><small>{notePath(note, props.directories)}</small></span>
            {note.published && <i title="Published" />}
          </button>
        ))}
      </div>
      <button className="trash-link" onClick={props.onOpenTrash}><Trash2 size={15} /> Trash</button>
    </aside>
  );
}
