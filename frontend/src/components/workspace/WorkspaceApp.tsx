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
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
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
    setNotice("Published Content updated.");
    const revisionResponse = await apiFetch<PageResponse<RevisionSummary>>(`/api/v1/notes/${id}/revisions?pageSize=30`);
    setRevisions(revisionResponse.data);
    return published;
  }

  async function trashNote(id: string, version: number) {
    await apiFetch<void>(`/api/v1/notes/${id}/trash`, { method: "POST", body: JSON.stringify({ expectedVersion: version }) });
    setNotes((current) => current.filter((note) => note.id !== id));
    setSelectedNote(null);
    setNotice("Moved to Trash.");
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
    setNotice("Attachment uploaded; its stable URI was copied.");
  }

  async function restoreRevision(revisionId: string) {
    if (!selectedNote || !window.confirm("Restore this revision into the current draft? Published Content will not change.")) return;
    const restored = await apiFetch<LearningNote>(`/api/v1/notes/${selectedNote.id}/revisions/${revisionId}/restore`, { method: "POST", body: JSON.stringify({ expectedVersion: selectedNote.version }) });
    updateNote(restored);
    setEditorEpoch((current) => current + 1);
    setNotice("Revision restored to the current draft.");
  }

  async function openTrash() {
    const response = await apiFetch<PageResponse<TrashEntry>>("/api/v1/trash/notes?pageSize=100");
    setTrashEntries(response.data);
    setTrashOpen(true);
  }

  async function restoreTrash(entry: TrashEntry) {
    const space = spaces.find((item) => item.id === entry.note.spaceId);
    const confirmPublish = space?.visibility === "public" ? window.confirm("Restoring to a public space republishes the current draft. Continue?") : false;
    if (space?.visibility === "public" && !confirmPublish) return;
    await apiFetch<LearningNote>(`/api/v1/trash/notes/${entry.note.id}/restore`, { method: "POST", body: JSON.stringify({ confirmPublish }) });
    setTrashEntries((current) => current.filter((item) => item.note.id !== entry.note.id));
    if (selectedSpaceId === entry.note.spaceId) await refreshSpace(selectedSpaceId);
    setNotice("Learning Note restored with its stable identity.");
  }

  async function permanentDelete(entry: TrashEntry) {
    if (!window.confirm(`Permanently delete “${entry.note.title}”? This cannot be undone.`)) return;
    await apiFetch<void>(`/api/v1/trash/notes/${entry.note.id}`, { method: "DELETE" });
    setTrashEntries((current) => current.filter((item) => item.note.id !== entry.note.id));
  }

  async function search(event: FormEvent) {
    event.preventDefault();
    if (!searchQuery.trim()) return;
    const response = await apiFetch<PageResponse<SearchResult>>(`/api/v1/search?q=${encodeURIComponent(searchQuery)}&pageSize=20`);
    setSearchResults(response.data);
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
        onOpenTrash={() => void openTrash()}
      />
      <div className="workspace-main">
        <header className="workspace-topbar">
          <div><strong>{selectedSpace?.name ?? "Workspace"}</strong><span>{selectedSpace?.visibility ?? "owner"}</span></div>
          <button className="workspace-search-trigger" onClick={() => setSearchOpen(true)}><Search size={16} /> Search Chinese or English <kbd>⌘K</kbd></button>
          <div className="owner-chip">
            {session?.owner.avatarUrl ? <>
              {/* The configured GitHub owner avatar is an arbitrary remote URL. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={session.owner.avatarUrl} alt="" width="28" height="28" />
            </> : <span className="owner-avatar-fallback" aria-hidden="true">{session?.owner.login.slice(0, 1).toUpperCase()}</span>}
            <span>{session?.owner.login}</span><button className="icon-button" onClick={() => void logout()} aria-label="Sign out"><LogOut size={16} /></button>
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
      {notice && <div className="workspace-toast" role="status">{notice}<button onClick={() => setNotice("")} aria-label="Dismiss"><X size={14} /></button></div>}
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
      {trashOpen && <WorkspaceDialog title="Trash" description="Items never expire automatically. Restore stable identities or delete explicitly." onClose={() => setTrashOpen(false)}>
        <div className="trash-list">
          {trashEntries.length === 0 && <p>Trash is empty.</p>}
          {trashEntries.map((entry) => <div key={entry.note.id}><span><strong>{entry.note.title}</strong><small>{new Date(entry.trashedAt).toLocaleString()}</small></span><button onClick={() => void restoreTrash(entry)}>Restore</button><button className="danger-link" onClick={() => void permanentDelete(entry)}><Trash2 size={14} /> Delete</button></div>)}
        </div>
      </WorkspaceDialog>}
      {searchOpen && <WorkspaceDialog title="Search the workspace" onClose={() => setSearchOpen(false)}>
        <form className="workspace-search-form" onSubmit={search}><Search size={18} /><input autoFocus value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} placeholder="memory / 上下文 / tools" /><button className="button button-primary">Search</button></form>
        <div className="workspace-search-results">{searchResults.map((result) => <button key={result.id} onClick={() => void openSearchResult(result)}><strong>{result.title}</strong><span>{result.snippet}</span></button>)}</div>
      </WorkspaceDialog>}
    </main>
  );
}

function CreateDialog({ kind, spaceId, directories, onClose, onCreated }: { kind: Exclude<DialogKind, null>; spaceId: string | null; directories: Directory[]; onClose: () => void; onCreated: (kind: Exclude<DialogKind, null>, value: KnowledgeSpace | Directory | LearningNote | Tag) => void }) {
  const [name, setName] = useState("");
  const [visibility, setVisibility] = useState<"private" | "public">("private");
  const [parentId, setParentId] = useState("");
  const [busy, setBusy] = useState(false);
  const labels = { space: "Knowledge Space", directory: "Directory", note: "Learning Note", tag: "Tag" };

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!name.trim()) return;
    setBusy(true);
    try {
      let value: KnowledgeSpace | Directory | LearningNote | Tag;
      if (kind === "space") value = await apiFetch<KnowledgeSpace>("/api/v1/spaces", { method: "POST", body: JSON.stringify({ name, visibility }) });
      else if (kind === "directory") value = await apiFetch<Directory>(`/api/v1/spaces/${spaceId}/directories`, { method: "POST", body: JSON.stringify({ name, parentId: parentId || null }) });
      else if (kind === "note") value = await apiFetch<LearningNote>("/api/v1/notes", { method: "POST", body: JSON.stringify({ spaceId, directoryId: parentId || null, title: name, markdown: `# ${name}\n\nStart writing what you understand.` }) });
      else value = await apiFetch<Tag>("/api/v1/tags", { method: "POST", body: JSON.stringify({ name }) });
      onCreated(kind, value);
    } finally { setBusy(false); }
  }

  return <WorkspaceDialog title={`Create ${labels[kind]}`} onClose={onClose}><form className="create-form" onSubmit={submit}><label>Name<input autoFocus value={name} onChange={(event) => setName(event.target.value)} /></label>{kind === "space" && <label>Visibility<select value={visibility} onChange={(event) => setVisibility(event.target.value as "private" | "public")}><option value="private">Private</option><option value="public">Public</option></select></label>}{(kind === "directory" || kind === "note") && <label>{kind === "directory" ? "Parent directory" : "Directory"}<select value={parentId} onChange={(event) => setParentId(event.target.value)}><option value="">Root</option>{directories.map((directory) => <option key={directory.id} value={directory.id}>{directory.name}</option>)}</select></label>}<div className="dialog-actions"><button type="button" className="button button-quiet" onClick={onClose}>Cancel</button><button className="button button-primary" disabled={busy}>{busy ? "Creating…" : `Create ${labels[kind]}`}</button></div></form></WorkspaceDialog>;
}

function WorkspaceLoading() { return <main className="workspace-gate"><span className="gate-mark">NF</span><p>Opening your learning workspace…</p></main>; }
function SignIn() { return <main className="workspace-gate"><span className="gate-mark">NF</span><p className="section-kicker">Knowledge Owner</p><h1>One workspace.<br />One accountable owner.</h1><p>Sign in with the configured GitHub identity to write, organize, and publish.</p><a className="button button-primary" href="/auth/github/start">Continue with GitHub</a><Link href="/">Return to public knowledge</Link></main>; }
function WorkspaceError() { return <main className="workspace-gate"><span className="gate-mark">!</span><h1>The workspace could not open.</h1><p>Check API readiness and reload this page.</p><button className="button button-primary" onClick={() => window.location.reload()}>Reload</button></main>; }
