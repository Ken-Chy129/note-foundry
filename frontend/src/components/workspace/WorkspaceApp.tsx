"use client";

import { LogOut, Search, Trash2, X } from "lucide-react";
import Link from "next/link";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { APIError, apiFetch } from "@/lib/api";
import type { Attachment, DataResponse, Directory, KnowledgeSpace, LearningNote, LinkedNote, PageResponse, SearchResult, SessionResponse, Tag } from "@/lib/types";
import { EditorPane } from "@/components/workspace/EditorPane";
import { InspectorPanel, type RevisionSummary } from "@/components/workspace/InspectorPanel";
import { KnowledgeSidebar } from "@/components/workspace/KnowledgeSidebar";
import { WorkspaceDialog } from "@/components/workspace/WorkspaceDialog";
import { WorkspaceSearchDialog } from "@/components/workspace/WorkspaceSearchDialog";

type DialogKind = "space" | "directory" | "note" | "tag" | null;
interface TrashEntry { note: LearningNote; trashedAt: string; }

export function WorkspaceApp() {
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [authState, setAuthState] = useState<"loading" | "signed-in" | "signed-out" | "error">("loading");
  const [spaces, setSpaces] = useState<KnowledgeSpace[]>([]);
  const [directories, setDirectories] = useState<Directory[]>([]);
  const [notes, setNotes] = useState<LearningNote[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [selectedSpaceId, setSelectedSpaceId] = useState<string | null>(null);
  const [selectedNote, setSelectedNote] = useState<LearningNote | null>(null);
  const [editorEpoch, setEditorEpoch] = useState(0);
  const [selectedTags, setSelectedTags] = useState<Set<string>>(new Set());
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [backlinks, setBacklinks] = useState<LinkedNote[]>([]);
  const [revisions, setRevisions] = useState<RevisionSummary[]>([]);
  const [dialog, setDialog] = useState<DialogKind>(null);
  const [trashOpen, setTrashOpen] = useState(false);
  const [trashEntries, setTrashEntries] = useState<TrashEntry[]>([]);
  const [searchOpen, setSearchOpen] = useState(false);
  const [notice, setNotice] = useState<string>("");

  const refreshSpace = useCallback(async (spaceId: string) => {
    const [directoryResponse, noteResponse] = await Promise.all([
      apiFetch<DataResponse<Directory>>(`/api/v1/spaces/${spaceId}/directories`),
      apiFetch<PageResponse<LearningNote>>(`/api/v1/notes?spaceId=${spaceId}&pageSize=100`)
    ]);
    setDirectories(directoryResponse.data);
    setNotes(noteResponse.data);
  }, []);

  useEffect(() => {
    const load = async () => {
      try {
        const currentSession = await apiFetch<SessionResponse>("/api/v1/session");
        const [spaceResponse, tagResponse] = await Promise.all([
          apiFetch<PageResponse<KnowledgeSpace>>("/api/v1/spaces?pageSize=100"),
          apiFetch<PageResponse<Tag>>("/api/v1/tags?pageSize=100")
        ]);
        setSession(currentSession);
        setSpaces(spaceResponse.data);
        setTags(tagResponse.data);
        setAuthState("signed-in");
        if (spaceResponse.data[0]) {
          setSelectedSpaceId(spaceResponse.data[0].id);
          await refreshSpace(spaceResponse.data[0].id);
        }
      } catch (error) {
        setAuthState(error instanceof APIError && error.status === 401 ? "signed-out" : "error");
      }
    };
    void load();
  }, [refreshSpace]);

  useEffect(() => {
    const handleShortcut = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        if (!dialog && !trashOpen) setSearchOpen(true);
      } else if (event.key === "Escape") {
        setSearchOpen(false);
      }
    };
    window.addEventListener("keydown", handleShortcut);
    return () => window.removeEventListener("keydown", handleShortcut);
  }, [dialog, trashOpen]);

  const loadNoteDetails = useCallback(async (note: LearningNote) => {
    setSelectedNote(note);
    setEditorEpoch((current) => current + 1);
    const [tagResponse, attachmentResponse, backlinkResponse, revisionResponse] = await Promise.all([
      apiFetch<DataResponse<Tag>>(`/api/v1/notes/${note.id}/tags`),
      apiFetch<DataResponse<Attachment>>(`/api/v1/notes/${note.id}/attachments`),
      apiFetch<DataResponse<LinkedNote>>(`/api/v1/notes/${note.id}/backlinks`),
      apiFetch<PageResponse<RevisionSummary>>(`/api/v1/notes/${note.id}/revisions?pageSize=30`)
    ]);
    setSelectedTags(new Set(tagResponse.data.map((tag) => tag.id)));
    setAttachments(attachmentResponse.data);
    setBacklinks(backlinkResponse.data);
    setRevisions(revisionResponse.data);
  }, []);

  async function selectSpace(spaceId: string) {
    setSelectedSpaceId(spaceId);
    setSelectedNote(null);
    await refreshSpace(spaceId);
  }

  function updateNote(updated: LearningNote) {
    setSelectedNote(updated);
    setNotes((current) => current.map((note) => note.id === updated.id ? updated : note));
  }

  async function autosave(id: string, version: number, title: string, markdown: string) {
    return apiFetch<LearningNote>(`/api/v1/notes/${id}`, { method: "PATCH", body: JSON.stringify({ expectedVersion: version, title, markdown }) });
  }

  async function publish(id: string, version: number) {
    const published = await apiFetch<LearningNote>(`/api/v1/notes/${id}/publish`, { method: "POST", body: JSON.stringify({ expectedVersion: version }) });
    setNotice("已发布内容已更新。");
    const revisionResponse = await apiFetch<PageResponse<RevisionSummary>>(`/api/v1/notes/${id}/revisions?pageSize=30`);
    setRevisions(revisionResponse.data);
    return published;
  }

  async function trashNote(id: string, version: number) {
    await apiFetch<void>(`/api/v1/notes/${id}/trash`, { method: "POST", body: JSON.stringify({ expectedVersion: version }) });
    setNotes((current) => current.filter((note) => note.id !== id));
    setSelectedNote(null);
    setNotice("已移入回收站。");
  }

  async function toggleTag(tagId: string) {
    if (!selectedNote) return;
    const next = new Set(selectedTags);
    if (next.has(tagId)) next.delete(tagId); else next.add(tagId);
    const response = await apiFetch<DataResponse<Tag>>(`/api/v1/notes/${selectedNote.id}/tags`, { method: "PUT", body: JSON.stringify({ tagIds: [...next] }) });
    setSelectedTags(new Set(response.data.map((tag) => tag.id)));
  }

  async function upload(file: File) {
    if (!selectedNote) return;
    const form = new FormData();
    form.append("file", file);
    const attachment = await apiFetch<Attachment>(`/api/v1/notes/${selectedNote.id}/attachments`, { method: "POST", body: form });
    setAttachments((current) => [...current, attachment]);
    await navigator.clipboard.writeText(`attachment:${attachment.id}`);
    setNotice("附件已上传，稳定 URI 已复制。");
  }

  async function restoreRevision(revisionId: string) {
    if (!selectedNote || !window.confirm("将这条修订恢复到当前笔记草稿吗？已发布内容不会改变。")) return;
    const restored = await apiFetch<LearningNote>(`/api/v1/notes/${selectedNote.id}/revisions/${revisionId}/restore`, { method: "POST", body: JSON.stringify({ expectedVersion: selectedNote.version }) });
    updateNote(restored);
    setEditorEpoch((current) => current + 1);
    setNotice("修订内容已恢复到当前笔记草稿。");
  }

  async function openTrash() {
    const response = await apiFetch<PageResponse<TrashEntry>>("/api/v1/trash/notes?pageSize=100");
    setTrashEntries(response.data);
    setTrashOpen(true);
  }

  async function restoreTrash(entry: TrashEntry) {
    const space = spaces.find((item) => item.id === entry.note.spaceId);
    const confirmPublish = space?.visibility === "public" ? window.confirm("恢复到公开知识空间会重新发布当前草稿，是否继续？") : false;
    if (space?.visibility === "public" && !confirmPublish) return;
    await apiFetch<LearningNote>(`/api/v1/trash/notes/${entry.note.id}/restore`, { method: "POST", body: JSON.stringify({ confirmPublish }) });
    setTrashEntries((current) => current.filter((item) => item.note.id !== entry.note.id));
    if (selectedSpaceId === entry.note.spaceId) await refreshSpace(selectedSpaceId);
    setNotice("学习笔记已恢复，稳定身份保持不变。");
  }

  async function permanentDelete(entry: TrashEntry) {
    if (!window.confirm(`永久删除《${entry.note.title}》吗？此操作无法撤销。`)) return;
    await apiFetch<void>(`/api/v1/trash/notes/${entry.note.id}`, { method: "DELETE" });
    setTrashEntries((current) => current.filter((item) => item.note.id !== entry.note.id));
  }

  async function openSearchResult(result: SearchResult) {
    const note = await apiFetch<LearningNote>(`/api/v1/notes/${result.id}`);
    if (selectedSpaceId !== note.spaceId) {
      setSelectedSpaceId(note.spaceId);
      await refreshSpace(note.spaceId);
    }
    await loadNoteDetails(note);
    setSearchOpen(false);
  }

  async function logout() {
    await apiFetch<void>("/auth/logout", { method: "POST" });
    window.location.reload();
  }

  const selectedSpace = useMemo(() => spaces.find((space) => space.id === selectedSpaceId) ?? null, [selectedSpaceId, spaces]);

  if (authState === "loading") return <WorkspaceLoading />;
  if (authState === "signed-out") return <SignIn />;
  if (authState === "error") return <WorkspaceError />;

  return (
    <main className="workspace-root">
      <KnowledgeSidebar
        spaces={spaces}
        directories={directories}
        notes={notes}
        selectedSpaceId={selectedSpaceId}
        selectedNoteId={selectedNote?.id ?? null}
        onSelectSpace={(id) => void selectSpace(id)}
        onSelectNote={(note) => void loadNoteDetails(note)}
        onCreateSpace={() => setDialog("space")}
        onCreateDirectory={() => setDialog("directory")}
        onCreateNote={() => setDialog("note")}
        onOpenSearch={() => setSearchOpen(true)}
        onOpenTrash={() => void openTrash()}
      />
      <div className="workspace-main">
        <header className="workspace-topbar">
          <div className="workspace-space-meta"><strong>{selectedSpace?.name ?? "学习工作台"}</strong><span>{selectedSpace ? selectedSpace.visibility === "public" ? "公开" : "私有" : "知识所有者"}</span></div>
          <button className="workspace-mobile-search icon-button" onClick={() => setSearchOpen(true)} aria-label="搜索笔记"><Search size={17} /></button>
          <div className="owner-chip">
            {session?.owner.avatarUrl ? <>
              {/* The configured GitHub owner avatar is an arbitrary remote URL. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={session.owner.avatarUrl} alt={`${session.owner.login} 的 GitHub 头像`} width="28" height="28" />
            </> : <span className="owner-avatar-fallback" aria-hidden="true">{session?.owner.login.slice(0, 1).toUpperCase()}</span>}
            <span>{session?.owner.login}</span><button className="icon-button" onClick={() => void logout()} aria-label="退出登录"><LogOut size={16} /></button>
          </div>
        </header>
        <div className="workspace-content">
          <EditorPane key={`${selectedNote?.id ?? "empty"}:${editorEpoch}`} note={selectedNote} onAutosave={autosave} onPublish={publish} onTrash={trashNote} onNoteChange={updateNote} />
          <InspectorPanel
            note={selectedNote}
            tags={tags}
            selectedTagIds={selectedTags}
            attachments={attachments}
            backlinks={backlinks}
            revisions={revisions}
            onToggleTag={(id) => void toggleTag(id)}
            onUpload={(file) => void upload(file)}
            onRestoreRevision={(id) => void restoreRevision(id)}
            onCreateTag={() => setDialog("tag")}
          />
        </div>
      </div>
      {notice && <div className="workspace-toast" role="status">{notice}<button onClick={() => setNotice("")} aria-label="关闭提示"><X size={14} /></button></div>}
      {dialog && <CreateDialog kind={dialog} spaceId={selectedSpaceId} directories={directories} onClose={() => setDialog(null)} onCreated={async (kind, value) => {
        setDialog(null);
        if (kind === "space") {
          setSpaces((current) => [...current, value as KnowledgeSpace]);
          await selectSpace((value as KnowledgeSpace).id);
        } else if (kind === "directory" && selectedSpaceId) {
          await refreshSpace(selectedSpaceId);
        } else if (kind === "note") {
          const note = value as LearningNote;
          setNotes((current) => [note, ...current]);
          await loadNoteDetails(note);
        } else if (kind === "tag") {
          setTags((current) => [...current, value as Tag]);
        }
      }} />}
      {trashOpen && <WorkspaceDialog title="回收站" description="回收站内容不会自动过期。你可以恢复其稳定身份，或明确执行永久删除。" onClose={() => setTrashOpen(false)}>
        <div className="trash-list">
          {trashEntries.length === 0 && <p>回收站为空。</p>}
          {trashEntries.map((entry) => <div key={entry.note.id}><span><strong>{entry.note.title}</strong><small>{new Date(entry.trashedAt).toLocaleString("zh-CN")}</small></span><button onClick={() => void restoreTrash(entry)}>恢复</button><button className="danger-link" onClick={() => void permanentDelete(entry)}><Trash2 size={14} /> 永久删除</button></div>)}
        </div>
      </WorkspaceDialog>}
      {searchOpen && <WorkspaceSearchDialog spaces={spaces} onClose={() => setSearchOpen(false)} onSelectResult={openSearchResult} />}
    </main>
  );
}

function CreateDialog({ kind, spaceId, directories, onClose, onCreated }: { kind: Exclude<DialogKind, null>; spaceId: string | null; directories: Directory[]; onClose: () => void; onCreated: (kind: Exclude<DialogKind, null>, value: KnowledgeSpace | Directory | LearningNote | Tag) => void }) {
  const [name, setName] = useState("");
  const [visibility, setVisibility] = useState<"private" | "public">("private");
  const [parentId, setParentId] = useState("");
  const [busy, setBusy] = useState(false);
  const labels = { space: "知识空间", directory: "目录", note: "学习笔记", tag: "标签" };

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!name.trim()) return;
    setBusy(true);
    try {
      let value: KnowledgeSpace | Directory | LearningNote | Tag;
      if (kind === "space") value = await apiFetch<KnowledgeSpace>("/api/v1/spaces", { method: "POST", body: JSON.stringify({ name, visibility }) });
      else if (kind === "directory") value = await apiFetch<Directory>(`/api/v1/spaces/${spaceId}/directories`, { method: "POST", body: JSON.stringify({ name, parentId: parentId || null }) });
      else if (kind === "note") value = await apiFetch<LearningNote>("/api/v1/notes", { method: "POST", body: JSON.stringify({ spaceId, directoryId: parentId || null, title: name, markdown: `# ${name}\n\n从这里开始记录你的理解。` }) });
      else value = await apiFetch<Tag>("/api/v1/tags", { method: "POST", body: JSON.stringify({ name }) });
      onCreated(kind, value);
    } finally { setBusy(false); }
  }

  return <WorkspaceDialog title={`创建${labels[kind]}`} onClose={onClose}><form className="create-form" onSubmit={submit}><label>名称<input autoFocus value={name} onChange={(event) => setName(event.target.value)} /></label>{kind === "space" && <label>可见性<select value={visibility} onChange={(event) => setVisibility(event.target.value as "private" | "public")}><option value="private">私有</option><option value="public">公开</option></select></label>}{(kind === "directory" || kind === "note") && <label>{kind === "directory" ? "上级目录" : "所属目录"}<select value={parentId} onChange={(event) => setParentId(event.target.value)}><option value="">根目录</option>{directories.map((directory) => <option key={directory.id} value={directory.id}>{directory.name}</option>)}</select></label>}<div className="dialog-actions"><button type="button" className="button button-quiet" onClick={onClose}>取消</button><button className="button button-primary" disabled={busy}>{busy ? "创建中…" : `创建${labels[kind]}`}</button></div></form></WorkspaceDialog>;
}

function WorkspaceLoading() { return <main className="workspace-gate"><span className="gate-mark">NF</span><p>正在打开学习工作台…</p></main>; }
function SignIn() { return <main className="workspace-gate"><span className="gate-mark">NF</span><p className="section-kicker">知识所有者</p><h1>一个工作台，<br />一位明确负责的所有者。</h1><p>使用已配置的 GitHub 身份登录，即可编写、整理和发布学习笔记。</p><a className="button button-primary" href="/auth/github/start">使用 GitHub 继续</a><Link href="/">返回公开知识</Link></main>; }
function WorkspaceError() { return <main className="workspace-gate"><span className="gate-mark">!</span><h1>无法打开学习工作台。</h1><p>请检查 API 是否就绪，然后重新加载此页面。</p><button className="button button-primary" onClick={() => window.location.reload()}>重新加载</button></main>; }
