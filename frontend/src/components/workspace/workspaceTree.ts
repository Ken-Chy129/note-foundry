import type { Directory, LearningNoteSummary } from "@/lib/types";

export interface DirectoryTreeNode {
  directory: Directory;
  notes: LearningNoteSummary[];
  children: DirectoryTreeNode[];
  totalNotes: number;
}

export interface WorkspaceDirectoryTree {
  rootNotes: LearningNoteSummary[];
  directories: DirectoryTreeNode[];
}

const directoryNameCollator = new Intl.Collator("zh-CN", { numeric: true, sensitivity: "base" });

export function initialCollapsedDirectoryIds(directories: Directory[], expandedDirectoryIds: string[] = []): Set<string> {
  const expanded = new Set(expandedDirectoryIds);
  return new Set(directories.map((directory) => directory.id).filter((id) => !expanded.has(id)));
}

export function buildDirectoryTree(directories: Directory[], notes: LearningNoteSummary[]): WorkspaceDirectoryTree {
  const nodes = new Map<string, DirectoryTreeNode>();
  const roots: DirectoryTreeNode[] = [];

  for (const directory of directories) {
    nodes.set(directory.id, { directory, notes: [], children: [], totalNotes: 0 });
  }

  for (const directory of directories) {
    const node = nodes.get(directory.id)!;
    const parent = directory.parentId ? nodes.get(directory.parentId) : undefined;
    if (parent && parent !== node) parent.children.push(node);
    else roots.push(node);
  }

  const rootNotes: LearningNoteSummary[] = [];
  for (const note of notes) {
    const directory = note.directoryId ? nodes.get(note.directoryId) : undefined;
    if (directory) directory.notes.push(note);
    else rootNotes.push(note);
  }

  function finalize(node: DirectoryTreeNode): number {
    node.children.sort((left, right) => directoryNameCollator.compare(left.directory.name, right.directory.name));
    node.totalNotes = node.notes.length + node.children.reduce((total, child) => total + finalize(child), 0);
    return node.totalNotes;
  }

  roots.sort((left, right) => directoryNameCollator.compare(left.directory.name, right.directory.name));
  roots.forEach(finalize);

  return { rootNotes, directories: roots };
}

export function directoryAncestorIds(directories: Directory[], directoryId: string | null): string[] {
  if (!directoryId) return [];

  const directoriesById = new Map(directories.map((directory) => [directory.id, directory]));
  if (!directoriesById.has(directoryId)) return [];

  const path: string[] = [];
  const visited = new Set<string>();
  let currentId: string | null = directoryId;

  while (currentId && !visited.has(currentId)) {
    const directory = directoriesById.get(currentId);
    if (!directory) break;
    path.unshift(directory.id);
    visited.add(directory.id);
    currentId = directory.parentId;
  }

  return path;
}
