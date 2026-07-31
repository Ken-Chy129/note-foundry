import { Folder } from "lucide-react";
import type { PublicSpaceDirectoryNode } from "./publicSpaceIndex";

interface PublicSpaceDirectoryNavProps {
  directories: PublicSpaceDirectoryNode[];
  rootNoteCount?: number;
}

function DirectoryNavBranch({ node }: { node: PublicSpaceDirectoryNode }) {
  return (
    <li>
      <a className="space-directory-link" href={`#${node.sectionId}`}>
        <span>{node.directory.name}</span>
        <small>{node.totalNotes}</small>
      </a>
      {node.children.length > 0 && (
        <ul>
          {node.children.map((child) => (
            <DirectoryNavBranch key={child.directory.id} node={child} />
          ))}
        </ul>
      )}
    </li>
  );
}

export function PublicSpaceDirectoryNav({ directories, rootNoteCount = 0 }: PublicSpaceDirectoryNavProps) {
  return (
    <nav className="space-directory-nav" aria-label="空间目录">
      <div className="space-directory-nav-title">
        <Folder aria-hidden="true" size={15} strokeWidth={1.7} />
        <span>空间目录</span>
      </div>
      <ul className="space-directory-tree">
        {rootNoteCount > 0 && (
          <li>
            <a className="space-directory-link" href="#未归类">
              <span>未归类</span>
              <small>{rootNoteCount}</small>
            </a>
          </li>
        )}
        {directories.map((node) => (
          <DirectoryNavBranch key={node.directory.id} node={node} />
        ))}
      </ul>
    </nav>
  );
}
