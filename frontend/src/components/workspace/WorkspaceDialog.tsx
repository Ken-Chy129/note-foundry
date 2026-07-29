"use client";

import { X } from "lucide-react";
import type { ReactNode } from "react";

export function WorkspaceDialog({ title, description, children, onClose }: { title: string; description?: string; children: ReactNode; onClose: () => void }) {
  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="workspace-dialog" role="dialog" aria-modal="true" aria-labelledby="dialog-title">
        <header>
          <div><h2 id="dialog-title">{title}</h2>{description && <p>{description}</p>}</div>
          <button className="icon-button" onClick={onClose} aria-label="关闭对话框"><X size={18} /></button>
        </header>
        {children}
      </section>
    </div>
  );
}
