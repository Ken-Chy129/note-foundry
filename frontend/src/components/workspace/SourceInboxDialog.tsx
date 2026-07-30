"use client";

import { FileText, Inbox, Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { LearningSource, LearningSourceSummary, PageResponse } from "@/lib/types";
import { WorkspaceDialog } from "@/components/workspace/WorkspaceDialog";

type InboxStatus = "loading" | "success" | "error";

export function SourceInboxDialog({ initialCreate, onClose }: { initialCreate: boolean; onClose: () => void }) {
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

  useEffect(() => {
    const controller = new AbortController();
    void apiFetch<PageResponse<LearningSourceSummary>>("/api/v1/sources?inbox=true&pageSize=100", { signal: controller.signal })
      .then((response) => {
        setSources(response.data);
        setStatus("success");
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setStatus("error");
      });
    return () => controller.abort();
  }, []);

  async function openSource(sourceId: string) {
    setCreating(false);
    setDetailStatus("loading");
    try {
      setSelected(await apiFetch<LearningSource>(`/api/v1/sources/${sourceId}`));
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

  return (
    <WorkspaceDialog
      title="资料收件箱"
      description="先保存，再决定它属于哪个知识空间。学习资料不会自动成为学习笔记。"
      className="source-inbox-dialog"
      onClose={onClose}
    >
      <div className="source-inbox-shell">
        <aside className="source-inbox-list" aria-label="待整理的学习资料">
          <button className="source-create-button" onClick={() => { setCreating(true); setSelected(null); setDetailStatus("idle"); }}>
            <Plus size={15} /> 手动资料
          </button>
          {status === "loading" && <div className="source-inbox-message" role="status">正在加载资料…</div>}
          {status === "error" && <div className="source-inbox-message is-error" role="status">资料收件箱暂时无法加载</div>}
          {status === "success" && sources.length === 0 && <div className="source-inbox-message is-empty" role="status">
            <Inbox size={20} />
            <strong>还没有待整理的学习资料</strong>
            <span>保存网页、PDF 或手动资料后，它们会先出现在这里。</span>
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
