"use client";

import { FileText, Inbox, Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { KnowledgeSpace, LearningSource, LearningSourceSummary, PageResponse } from "@/lib/types";
import { WorkspaceDialog } from "@/components/workspace/WorkspaceDialog";

type InboxStatus = "loading" | "success" | "error";

interface SourceInboxDialogProps {
  initialCreate: boolean;
  view: "inbox" | "space";
  spaces: KnowledgeSpace[];
  selectedSpaceId: string | null;
  onClose: () => void;
}

export function SourceInboxDialog({ initialCreate, view, spaces, selectedSpaceId, onClose }: SourceInboxDialogProps) {
  const [sources, setSources] = useState<LearningSourceSummary[]>([]);
  const [status, setStatus] = useState<InboxStatus>("loading");
  const [selected, setSelected] = useState<LearningSource | null>(null);
  const [detailStatus, setDetailStatus] = useState<"idle" | "loading" | "success" | "error">("idle");
  const [creating, setCreating] = useState(initialCreate);
  const [title, setTitle] = useState("");
  const [captureNote, setCaptureNote] = useState("");
  const [content, setContent] = useState("");
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState("");
  const [targetSpaceId, setTargetSpaceId] = useState("");
  const [organizing, setOrganizing] = useState(false);
  const [organizeError, setOrganizeError] = useState("");
  const selectedSpace = spaces.find((space) => space.id === selectedSpaceId) ?? null;
  const isInbox = view === "inbox";

  useEffect(() => {
    const controller = new AbortController();
    const path = isInbox
      ? "/api/v1/sources?inbox=true&pageSize=100"
      : `/api/v1/sources?spaceId=${encodeURIComponent(selectedSpaceId ?? "")}&pageSize=100`;
    void apiFetch<PageResponse<LearningSourceSummary>>(path, { signal: controller.signal })
      .then((response) => {
        setSources(response.data);
        setStatus("success");
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setStatus("error");
      });
    return () => controller.abort();
  }, [isInbox, selectedSpaceId]);

  async function openSource(sourceId: string) {
    setCreating(false);
    setDetailStatus("loading");
    try {
      const source = await apiFetch<LearningSource>(`/api/v1/sources/${sourceId}`);
      setSelected(source);
      setTargetSpaceId(source.spaceId ?? "");
      setOrganizeError("");
      setDetailStatus("success");
    } catch {
      setSelected(null);
      setDetailStatus("error");
    }
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!title.trim() || saving) return;
    setSaving(true);
    setFormError("");
    try {
      const source = await apiFetch<LearningSource>("/api/v1/sources", {
        method: "POST",
        body: JSON.stringify({ kind: "manual", title, captureNote, content })
      });
      setSources((current) => [source, ...current]);
      setSelected(source);
      setTargetSpaceId(source.spaceId ?? "");
      setDetailStatus("success");
      setCreating(false);
      setTitle("");
      setCaptureNote("");
      setContent("");
    } catch {
      setFormError("资料保存失败，请稍后重试");
    } finally {
      setSaving(false);
    }
  }

  async function organizeSource() {
    if (!selected || organizing || targetSpaceId === (selected.spaceId ?? "")) return;
    setOrganizing(true);
    setOrganizeError("");
    try {
      const source = await apiFetch<LearningSource>(`/api/v1/sources/${selected.id}`, {
        method: "PATCH",
        body: JSON.stringify({ spaceId: targetSpaceId || null })
      });
      const remainsVisible = isInbox ? source.spaceId === null : source.spaceId === selectedSpaceId;
      setSources((current) => remainsVisible
        ? current.map((item) => item.id === source.id ? source : item)
        : current.filter((item) => item.id !== source.id));
      if (remainsVisible) {
        setSelected(source);
      } else {
        setSelected(null);
        setDetailStatus("idle");
      }
    } catch {
      setOrganizeError("资料位置保存失败，请稍后重试");
    } finally {
      setOrganizing(false);
    }
  }

  return (
    <WorkspaceDialog
      title={isInbox ? "资料收件箱" : `${selectedSpace?.name ?? "当前空间"} · 学习资料`}
      description={isInbox ? "先保存，再决定它属于哪个知识空间。学习资料不会自动成为学习笔记。" : "这里集中展示当前知识空间的学习资料，与笔记目录分开整理。"}
      className="source-inbox-dialog"
      onClose={onClose}
    >
      <div className="source-inbox-shell">
        <aside className="source-inbox-list" aria-label={isInbox ? "待整理的学习资料" : "当前知识空间的学习资料"}>
          {isInbox && <button className="source-create-button" onClick={() => { setCreating(true); setSelected(null); setDetailStatus("idle"); }}>
            <Plus size={15} /> 手动资料
          </button>}
          {status === "loading" && <div className="source-inbox-message" role="status">正在加载资料…</div>}
          {status === "error" && <div className="source-inbox-message is-error" role="status">{isInbox ? "资料收件箱暂时无法加载" : "当前空间资料暂时无法加载"}</div>}
          {status === "success" && sources.length === 0 && <div className="source-inbox-message is-empty" role="status">
            <Inbox size={20} />
            <strong>{isInbox ? "还没有待整理的学习资料" : "当前空间还没有学习资料"}</strong>
            <span>{isInbox ? "保存网页、PDF 或手动资料后，它们会先出现在这里。" : "从资料收件箱中选择一份资料，把它整理到当前知识空间。"}</span>
          </div>}
          {sources.map((source) => (
            <button
              key={source.id}
              className={`source-inbox-item${selected?.id === source.id ? " is-active" : ""}`}
              onClick={() => void openSource(source.id)}
            >
              <FileText size={15} />
              <span><strong>{source.title}</strong><small>{source.captureNote || "暂无保存备注"}</small></span>
            </button>
          ))}
        </aside>

        <section className="source-inbox-detail">
          {creating ? <form className="source-create-form" onSubmit={submit}>
            <p className="section-kicker">手动资料</p>
            <h3>记录一份学习输入</h3>
            <label>标题<input autoFocus value={title} onChange={(event) => setTitle(event.target.value)} maxLength={240} /></label>
            <label>保存备注<textarea value={captureNote} onChange={(event) => setCaptureNote(event.target.value)} rows={3} placeholder="为什么保存，准备重点看什么" /></label>
            <label>资料正文<textarea value={content} onChange={(event) => setContent(event.target.value)} rows={10} placeholder="粘贴或整理当前资料内容" /></label>
            {formError && <p className="source-form-error" role="alert">{formError}</p>}
            <div className="dialog-actions"><button type="button" className="button button-quiet" onClick={() => setCreating(false)}>取消</button><button className="button button-primary" disabled={saving || !title.trim()}>{saving ? "保存中…" : "保存到收件箱"}</button></div>
          </form> : detailStatus === "loading" ? <div className="source-detail-empty" role="status">正在加载资料内容…</div>
            : detailStatus === "error" ? <div className="source-detail-empty is-error" role="status"><strong>资料内容暂时无法加载</strong><span>请稍后重新选择这份资料。</span></div>
            : selected ? <article className="source-detail-article">
            <p className="section-kicker">手动资料 · 已就绪</p>
            <h3>{selected.title}</h3>
            {selected.captureNote && <div className="source-capture-note"><small>保存备注</small><p>{selected.captureNote}</p></div>}
            <div className="source-content"><small>资料正文</small><p>{selected.content || "这份资料还没有正文内容"}</p></div>
            <div className="source-organization">
              <label>所属位置<select value={targetSpaceId} disabled={organizing} onChange={(event) => setTargetSpaceId(event.target.value)}>
                <option value="">资料收件箱</option>
                {spaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
              </select></label>
              <button className="button button-primary" disabled={organizing || targetSpaceId === (selected.spaceId ?? "")} onClick={() => void organizeSource()}>
                {organizing ? "保存中…" : targetSpaceId === "" && selected.spaceId ? "移回资料收件箱" : "保存位置"}
              </button>
            </div>
            {organizeError && <p className="source-form-error" role="alert">{organizeError}</p>}
          </article> : <div className="source-detail-empty">
            <FileText size={22} />
            <strong>选择一份资料查看内容</strong>
            <span>也可以创建一份手动资料，先放入收件箱。</span>
          </div>}
        </section>
      </div>
    </WorkspaceDialog>
  );
}
