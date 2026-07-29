"use client";

import Link from "next/link";
import ReactMarkdown, { defaultUrlTransform } from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import { MermaidDiagram } from "@/components/markdown/MermaidDiagram";

interface MarkdownRendererProps {
  markdown: string;
  attachmentScope?: "owner" | "public";
}

const stableIdPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function stableTarget(value: string | undefined, scheme: string): string | null {
  if (!value?.startsWith(`${scheme}:`)) return null;
  const target = value.slice(scheme.length + 1);
  return stableIdPattern.test(target) ? target.toLowerCase() : null;
}

function noteFoundryUrlTransform(value: string): string {
  if (stableTarget(value, "note") || stableTarget(value, "attachment")) {
    return value;
  }
  return defaultUrlTransform(value);
}

export function MarkdownRenderer({ markdown, attachmentScope = "public" }: MarkdownRendererProps) {
  return (
    <article className="markdown-body">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
        urlTransform={noteFoundryUrlTransform}
        components={{
          h1({ children }) {
            return <h2>{children}</h2>;
          },
          a({ href, children }) {
            const noteId = stableTarget(href, "note");
            if (noteId) {
              return <Link href={`/notes/${noteId}`}>{children}</Link>;
            }
            return <a href={href} rel="noreferrer noopener">{children}</a>;
          },
          img({ src, alt }) {
            const attachmentId = stableTarget(typeof src === "string" ? src : undefined, "attachment");
            const resolved = attachmentId
              ? `/api/v1/${attachmentScope === "public" ? "public/" : ""}attachments/${attachmentId}`
              : typeof src === "string" ? src : "";
            // Markdown images have unknown intrinsic dimensions; the containing prose reserves a stable block.
            // Managed Attachment images have arbitrary intrinsic dimensions.
            // eslint-disable-next-line @next/next/no-img-element
            return <img src={resolved} alt={alt ?? ""} loading="lazy" />;
          },
          code({ className, children }) {
            const language = /language-([^ ]+)/.exec(className ?? "")?.[1];
            const source = String(children).replace(/\n$/, "");
            if (language === "mermaid") {
              return <MermaidDiagram source={source} />;
            }
            return <code className={className}>{children}</code>;
          }
        }}
      >
        {markdown}
      </ReactMarkdown>
    </article>
  );
}
