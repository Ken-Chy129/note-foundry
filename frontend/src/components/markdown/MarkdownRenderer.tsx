"use client";

import Link from "next/link";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import { MermaidDiagram } from "@/components/markdown/MermaidDiagram";

interface MarkdownRendererProps {
  markdown: string;
  attachmentScope?: "owner" | "public";
}

function stableTarget(value: string | undefined, scheme: string): string | null {
  if (!value?.startsWith(`${scheme}:`)) return null;
  return value.slice(scheme.length + 1);
}

export function MarkdownRenderer({ markdown, attachmentScope = "public" }: MarkdownRendererProps) {
  return (
    <article className="markdown-body">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
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
