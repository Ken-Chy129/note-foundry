"use client";

import { FileText, Inbox, Link2, Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { KnowledgeSpace, LearningSource, LearningSourceSummary, PageResponse } from "@/lib/types";
import { WorkspaceDialog } from "@/components/workspace/WorkspaceDialog";

type InboxStatus = "loading" | "success" | "error";
type CreateKind = "manual" | "url";

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
  const [createKind, setCreateKind] = useState<CreateKind | null>(initialCreate ? "manual" : null);
  const [title, setTitle] = useState("");
  const [captureNote, setCaptureNote] = useState("");
  const [content, setContent] = useState("");
  const [originalUrl, setOriginalUrl] = useState("");
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState("");
  const [targetSpaceId, setTargetSpaceId] = useState("");
  const [organizing, setOrganizing] = useState(false);
  const [retrying, setRetrying] = useState(false);
  const [organizeError, setOrganizeError] = useState("");
  const selectedSpace = spaces.find((space) => space.id === selectedSpaceId) ?? null;
  const isInbox = view === "inbox";

  useEffect(() => {
    if (!selected || selected.kind !== "url" || !["pending", "processing"].includes(selected.processingStatus)) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      void apiFetch<LearningSource>(`/api/v1/sources/${selected.id}`, { signal: controller.signal })
        .then((source) => {
          setSelected(source);
          setSources((current) => current.map((item) => item.id === source.id ? source : item));
        })
        .catch(() => undefined);
    }, 1800);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [selected]);

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
    setCreateKind(null);
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
    if (!createKind || saving || (createKind === "manual" ? !title.trim() : !originalUrl.trim())) return;
    setSaving(true);
    setFormError("");
    try {
      const source = await apiFetch<LearningSource>("/api/v1/sources", {
        method: "POST",
        body: JSON.stringify(createKind === "manual"
          ? { kind: "manual", title, captureNote, content }
          : { kind: "url", title, captureNote, originalUrl })
      });
      setSources((current) => source.spaceId === null
        ? [source, ...current.filter((item) => item.id !== source.id)]
        : current.filter((item) => item.id !== source.id));
      setSelected(source);
      setTargetSpaceId(source.spaceId ?? "");
      setDetailStatus("success");
      setCreateKind(null);
      setTitle("");
      setCaptureNote("");
      setContent("");
      setOriginalUrl("");
    } catch {
      setFormError(createKind === "url" ? "网页链接保存失败，请检查地址后重试" : "资料保存失败，请稍后重试");
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

  async function retryExtraction() {
    if (!selected || retrying) return;
    setRetrying(true);
    try {
      const source = await apiFetch<LearningSource>(`/api/v1/sources/${selected.id}/retry-extraction`, { method: "POST" });
      setSelected(source);
      setSources((current) => current.map((item) => item.id === source.id ? source : item));
    } catch {
      setOrganizeError("重新提取失败，请稍后重试");
    } finally {
      setRetrying(false);
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
          {isInbox && <div className="source-create-actions">
            <button className="source-create-button" onClick={() => { setCreateKind("url"); setSelected(null); setDetailStatus("idle"); }}>
              <Link2 size={15} /> 网页链接
            </button>
            <button className="source-create-button" onClick={() => { setCreateKind("manual"); setSelected(null); setDetailStatus("idle"); }}>
              <Plus size={15} /> 手动资料
            </button>
          </div>}
          {status === "loading" && <div className="source-inbox-message" role="status">正在加载资料…</div>}
          {status === "error" && <div className="source-inbox-message is-error" role="status">{isInbox ? "资料收件箱暂时无法加载" : "当前空间资料暂时无法加载"}</div>}
          {status === "success" && sources.length === 0 && <div className="source-inbox-message is-empty" role="status">
            <Inbox size={20} />
            <strong>{isInbox ? "还没有待整理的学习资料" : "当前空间还没有学习资料"}</strong>
            <span>{isInbox ? "保存网页链接或手动资料后，它们会先出现在这里。" : "从资料收件箱中选择一份资料，把它整理到当前知识空间。"}</span>
          </div>}
          {sources.map((source) => (
            <button
              key={source.id}
              className={`source-inbox-item${selected?.id === source.id ? " is-active" : ""}`}
              onClick={() => void openSource(source.id)}
            >
              {source.kind === "url" ? <Link2 size={15} /> : <FileText size={15} />}
              <span><strong>{source.title}</strong><small>{sourceListMeta(source)}</small></span>
            </button>
          ))}
        </aside>

        <section className="source-inbox-detail">
          {createKind ? <form className="source-create-form" onSubmit={submit}>
            <p className="section-kicker">{createKind === "url" ? "网页链接" : "手动资料"}</p>
            <h3>{createKind === "url" ? "保存一个待阅读网页" : "记录一份学习输入"}</h3>
            {createKind === "url" && <label>网页地址<input autoFocus type="url" value={originalUrl} onChange={(event) => setOriginalUrl(event.target.value)} placeholder="https://example.com/article" /></label>}
            <label>{createKind === "url" ? "标题（可选）" : "标题"}<input autoFocus={createKind === "manual"} value={title} onChange={(event) => setTitle(event.target.value)} maxLength={240} placeholder={createKind === "url" ? "留空时会自动读取网页标题" : undefined} /></label>
            <label>保存备注<textarea value={captureNote} onChange={(event) => setCaptureNote(event.target.value)} rows={3} placeholder="为什么保存，准备重点看什么" /></label>
            {createKind === "manual" && <label>资料正文<textarea value={content} onChange={(event) => setContent(event.target.value)} rows={10} placeholder="粘贴或整理当前资料内容" /></label>}
            {formError && <p className="source-form-error" role="alert">{formError}</p>}
            <div className="dialog-actions"><button type="button" className="button button-quiet" onClick={() => setCreateKind(null)}>取消</button><button className="button button-primary" disabled={saving || (createKind === "manual" ? !title.trim() : !originalUrl.trim())}>{saving ? "保存中…" : "保存到收件箱"}</button></div>
          </form> : detailStatus === "loading" ? <div className="source-detail-empty" role="status">正在加载资料内容…</div>
            : detailStatus === "error" ? <div className="source-detail-empty is-error" role="status"><strong>资料内容暂时无法加载</strong><span>请稍后重新选择这份资料。</span></div>
            : selected ? <article className="source-detail-article">
            <p className="section-kicker">{selected.kind === "url" ? "网页链接" : "手动资料"} · {sourceStatusLabel(selected.processingStatus)}</p>
            <h3>{selected.title}</h3>
            {selected.originalUrl && <a className="source-origin-link" href={selected.originalUrl} target="_blank" rel="noreferrer"><Link2 size={14} /><span>{selected.originalUrl}</span></a>}
            {selected.captureNote && <div className="source-capture-note"><small>保存备注</small><p>{selected.captureNote}</p></div>}
            <div className={`source-content${selected.processingStatus === "failed" ? " is-failed" : ""}`}><small>资料正文</small><p>{sourceContent(selected)}</p></div>
            {selected.kind === "url" && selected.processingStatus === "failed" && <div className="source-retry-row"><button className="button button-primary" disabled={retrying} onClick={() => void retryExtraction()}>{retrying ? "正在提交…" : "重新提取"}</button></div>}
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
            <span>也可以保存网页链接或创建手动资料，先放入收件箱。</span>
          </div>}
        </section>
      </div>
    </WorkspaceDialog>
  );
}

function sourceStatusLabel(status: LearningSourceSummary["processingStatus"]) {
  switch (status) {
    case "pending": return "等待提取";
    case "processing": return "正在提取";
    case "failed": return "提取失败";
    default: return "已就绪";
  }
}

function sourceContent(source: LearningSource) {
  if (source.processingStatus === "pending") return "网页已保存，正在等待后台提取正文";
  if (source.processingStatus === "processing") return "正在读取网页并整理正文，请稍候";
  if (source.processingStatus === "failed") return "网页内容提取失败，可以重新提取";
  return source.content || "这份资料还没有正文内容";
}

function sourceListMeta(source: LearningSourceSummary) {
  const detail = source.captureNote || source.originalUrl || "暂无保存备注";
  return source.processingStatus === "ready" ? detail : `${sourceStatusLabel(source.processingStatus)} · ${detail}`;
}
