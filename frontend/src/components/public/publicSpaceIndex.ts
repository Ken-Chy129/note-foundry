import type { Directory, PublishedNote } from "@/lib/types";

export interface PublicSpaceDirectoryNode {
  directory: Directory;
  notes: PublishedNote[];
  children: PublicSpaceDirectoryNode[];
  totalNotes: number;
  path: string[];
  sectionId: string;
}

export interface PublicSpaceIndex {
  rootNotes: PublishedNote[];
  directories: PublicSpaceDirectoryNode[];
}

const titleCollator = new Intl.Collator("zh-CN", { numeric: true, sensitivity: "base" });

function anchorSegment(value: string): string {
  return value
    .trim()
    .toLocaleLowerCase("zh-CN")
    .replace(/\s+/g, "-")
    .replace(/[^\p{L}\p{N}-]+/gu, "")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "") || "未命名";
}

export function buildPublicSpaceIndex(directories: Directory[], notes: PublishedNote[]): PublicSpaceIndex {
  const nodes = new Map<string, PublicSpaceDirectoryNode>();
  const roots: PublicSpaceDirectoryNode[] = [];
  const usedSectionIds = new Map<string, number>();

  for (const directory of directories) {
    nodes.set(directory.id, {
      directory,
      notes: [],
      children: [],
      totalNotes: 0,
      path: [],
      sectionId: ""
    });
  }

  for (const directory of directories) {
    const node = nodes.get(directory.id)!;
    const parent = directory.parentId ? nodes.get(directory.parentId) : undefined;
    if (parent && parent !== node) parent.children.push(node);
    else roots.push(node);
  }

  const rootNotes: PublishedNote[] = [];
  for (const note of notes) {
    const node = note.directoryId ? nodes.get(note.directoryId) : undefined;
    if (node) node.notes.push(note);
    else rootNotes.push(note);
  }

  function finalize(node: PublicSpaceDirectoryNode, parentPath: string[]): number {
    node.path = [...parentPath, node.directory.name];
    const baseSectionId = `目录-${node.path.map(anchorSegment).join("-")}`;
    const duplicateCount = (usedSectionIds.get(baseSectionId) ?? 0) + 1;
    usedSectionIds.set(baseSectionId, duplicateCount);
    node.sectionId = duplicateCount === 1 ? baseSectionId : `${baseSectionId}-${duplicateCount}`;
    node.notes.sort((left, right) => titleCollator.compare(left.title, right.title));
    node.children.sort((left, right) => titleCollator.compare(left.directory.name, right.directory.name));
    node.totalNotes = node.notes.length + node.children.reduce((total, child) => total + finalize(child, node.path), 0);
    return node.totalNotes;
  }

  roots.sort((left, right) => titleCollator.compare(left.directory.name, right.directory.name));
  roots.forEach((node) => finalize(node, []));
  rootNotes.sort((left, right) => titleCollator.compare(left.title, right.title));

  return { rootNotes, directories: roots };
}
