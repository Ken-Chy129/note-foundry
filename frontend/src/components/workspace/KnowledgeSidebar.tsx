"use client";

import { ChevronRight, FilePlus2, FolderPlus, Inbox, Lock, Plus, Search, Trash2, Unlock } from "lucide-react";
import { NoteFoundryLogo } from "@/components/brand/NoteFoundryLogo";
import type { Directory, KnowledgeSpace, LearningNoteSummary } from "@/lib/types";
import { WorkspaceDirectoryTree } from "@/components/workspace/WorkspaceDirectoryTree";

interface KnowledgeSidebarProps {
  spaces: KnowledgeSpace[];
  directories: Directory[];
  notes: LearningNoteSummary[];
  selectedSpaceId: string | null;
  selectedNoteId: string | null;
  onSelectSpace: (id: string) => void;
  onSelectNote: (note: LearningNoteSummary) => void;
  onCreateSpace: () => void;
  onCreateDirectory: () => void;
  onCreateNote: () => void;
  onOpenSourceInbox: () => void;
  onCreateSource: () => void;
  onOpenSearch: () => void;
  onOpenTrash: () => void;
}

export function KnowledgeSidebar(props: KnowledgeSidebarProps) {
  return (
    <aside className="workspace-sidebar" aria-label="知识导航">
      <div className="workspace-brand"><NoteFoundryLogo /><strong>NoteFoundry</strong></div>
      <div className="sidebar-search-area">
        <button className="sidebar-search-button" onClick={props.onOpenSearch}>
          <Search size={15} />
          <span>搜索笔记</span>
          <kbd>⌘K</kbd>
        </button>
      </div>
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

      <div className="sidebar-heading sidebar-heading-sources">
        <span>学习资料</span>
        <button className="icon-button" onClick={props.onCreateSource} aria-label="创建手动资料"><FilePlus2 size={16} /></button>
      </div>
      <button className="source-inbox-link" onClick={props.onOpenSourceInbox}>
        <Inbox size={15} />
        <span>资料收件箱</span>
        <ChevronRight size={14} />
      </button>

      <div className="sidebar-heading sidebar-heading-notes">
        <span>目录与笔记</span>
        <div>
          <button className="icon-button" onClick={props.onCreateDirectory} aria-label="创建目录" disabled={!props.selectedSpaceId}><FolderPlus size={16} /></button>
          <button className="icon-button" onClick={props.onCreateNote} aria-label="创建学习笔记" disabled={!props.selectedSpaceId}><Plus size={17} /></button>
        </div>
      </div>
      <WorkspaceDirectoryTree directories={props.directories} notes={props.notes} selectedNoteId={props.selectedNoteId} onSelectNote={props.onSelectNote} />
      <button className="trash-link" onClick={props.onOpenTrash}><Trash2 size={15} /> 回收站</button>
    </aside>
  );
}
