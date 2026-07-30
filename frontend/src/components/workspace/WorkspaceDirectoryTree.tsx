"use client";

import { BookOpen, ChevronRight, Files, Folder, FolderOpen } from "lucide-react";
import { useMemo, useState } from "react";
import type { Directory, LearningNote } from "@/lib/types";
import { buildDirectoryTree, directoryAncestorIds, initialCollapsedDirectoryIds, type DirectoryTreeNode } from "@/components/workspace/workspaceTree";

interface WorkspaceDirectoryTreeProps {
  directories: Directory[];
  notes: LearningNote[];
  selectedNoteId: string | null;
  onSelectNote: (note: LearningNote) => void;
}

export function WorkspaceDirectoryTree({ directories, notes, selectedNoteId, onSelectNote }: WorkspaceDirectoryTreeProps) {
  const tree = useMemo(() => buildDirectoryTree(directories, notes), [directories, notes]);
  const selectedNote = useMemo(() => notes.find((note) => note.id === selectedNoteId) ?? null, [notes, selectedNoteId]);
  const selectedDirectoryPath = useMemo(
    () => directoryAncestorIds(directories, selectedNote?.directoryId ?? null),
    [directories, selectedNote?.directoryId]
  );
  const selectedDirectoryIds = useMemo(() => new Set(selectedDirectoryPath), [selectedDirectoryPath]);
  const initiallyCollapsedIds = useMemo(
    () => initialCollapsedDirectoryIds(directories, selectedDirectoryPath),
    [directories, selectedDirectoryPath]
  );
  const directoryTreeKey = useMemo(() => directories.map((directory) => directory.id).join(":"), [directories]);

  if (notes.length === 0 && directories.length === 0) {
    return <div className="note-navigation"><p className="sidebar-empty">在这个空间中创建第一篇学习笔记。</p></div>;
  }

  return (
    <DirectoryTreeContent
      key={directoryTreeKey}
      tree={tree}
      initiallyCollapsedIds={initiallyCollapsedIds}
      selectedDirectoryIds={selectedDirectoryIds}
      selectedNoteId={selectedNoteId}
      onSelectNote={onSelectNote}
    />
  );
}

function DirectoryTreeContent({ tree, initiallyCollapsedIds, selectedDirectoryIds, selectedNoteId, onSelectNote }: {
  tree: ReturnType<typeof buildDirectoryTree>;
  initiallyCollapsedIds: Set<string>;
  selectedDirectoryIds: Set<string>;
  selectedNoteId: string | null;
  onSelectNote: (note: LearningNote) => void;
}) {
  const [collapseState, setCollapseState] = useState<{ selectedNoteId: string | null; ids: Set<string> }>(() => ({
    selectedNoteId,
    ids: new Set(initiallyCollapsedIds)
  }));
  const collapsedDirectoryIds = collapseState.selectedNoteId === selectedNoteId
    ? collapseState.ids
    : new Set([...collapseState.ids].filter((id) => !selectedDirectoryIds.has(id)));

  function toggleDirectory(id: string) {
    setCollapseState(() => {
      const next = new Set(collapsedDirectoryIds);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return { selectedNoteId, ids: next };
    });
  }

  return (
    <div className="note-navigation directory-tree" role="tree" aria-label="学习笔记目录">
      {tree.rootNotes.length > 0 && <div className="directory-tree-root-group">
        <div className="directory-tree-root-label"><Files size={15} /><span>根目录</span><small>{tree.rootNotes.length}</small></div>
        <div className="directory-tree-children is-root" role="group">
          {tree.rootNotes.map((note) => <NoteTreeItem key={note.id} note={note} selected={note.id === selectedNoteId} onSelect={onSelectNote} />)}
        </div>
      </div>}
      {tree.directories.map((node) => (
        <DirectoryBranch
          key={node.directory.id}
          node={node}
          collapsedDirectoryIds={collapsedDirectoryIds}
          selectedDirectoryIds={selectedDirectoryIds}
          selectedNoteId={selectedNoteId}
          onToggle={toggleDirectory}
          onSelectNote={onSelectNote}
        />
      ))}
    </div>
  );
}

function DirectoryBranch({ node, collapsedDirectoryIds, selectedDirectoryIds, selectedNoteId, onToggle, onSelectNote }: {
  node: DirectoryTreeNode;
  collapsedDirectoryIds: Set<string>;
  selectedDirectoryIds: Set<string>;
  selectedNoteId: string | null;
  onToggle: (id: string) => void;
  onSelectNote: (note: LearningNote) => void;
}) {
  const expanded = !collapsedDirectoryIds.has(node.directory.id);
  const groupId = `directory-${node.directory.id}`;

  return <div className="directory-tree-branch">
    <button
      type="button"
      className={`directory-tree-row${selectedDirectoryIds.has(node.directory.id) ? " is-selected-path" : ""}`}
      role="treeitem"
      aria-expanded={expanded}
      aria-selected={false}
      aria-controls={groupId}
      onClick={() => onToggle(node.directory.id)}
    >
      <ChevronRight className={expanded ? "is-expanded" : ""} size={14} />
      {expanded ? <FolderOpen size={15} /> : <Folder size={15} />}
      <span title={node.directory.name}>{node.directory.name}</span>
      <small>{node.totalNotes}</small>
    </button>
    {expanded && <div id={groupId} className="directory-tree-children" role="group">
      {node.notes.map((note) => <NoteTreeItem key={note.id} note={note} selected={note.id === selectedNoteId} onSelect={onSelectNote} />)}
      {node.children.map((child) => (
        <DirectoryBranch
          key={child.directory.id}
          node={child}
          collapsedDirectoryIds={collapsedDirectoryIds}
          selectedDirectoryIds={selectedDirectoryIds}
          selectedNoteId={selectedNoteId}
          onToggle={onToggle}
          onSelectNote={onSelectNote}
        />
      ))}
    </div>}
  </div>;
}

function NoteTreeItem({ note, selected, onSelect }: { note: LearningNote; selected: boolean; onSelect: (note: LearningNote) => void }) {
  return <button
    type="button"
    className={`directory-tree-note${selected ? " is-active" : ""}`}
    role="treeitem"
    aria-selected={selected}
    onClick={() => onSelect(note)}
  >
    <BookOpen size={14} />
    <strong title={note.title}>{note.title}</strong>
    {note.published && <i title="已发布" />}
  </button>;
}
