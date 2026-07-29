"use client";

import { useEffect, useId, useState } from "react";

export function MermaidDiagram({ source }: { source: string }) {
  const reactId = useId();
  const [svg, setSvg] = useState<string>("");
  const [error, setError] = useState<string>("");

  useEffect(() => {
    let cancelled = false;
    const render = async () => {
      try {
        const { default: mermaid } = await import("mermaid");
        mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: "neutral", fontFamily: "IBM Plex Sans" });
        const id = `mermaid-${reactId.replace(/[^a-zA-Z0-9]/g, "")}`;
        const result = await mermaid.render(id, source);
        if (!cancelled) {
          setSvg(result.svg);
          setError("");
        }
      } catch {
        if (!cancelled) {
          setError("无法渲染 Mermaid 图表。");
        }
      }
    };
    void render();
    return () => {
      cancelled = true;
    };
  }, [reactId, source]);

  if (error) {
    return <pre className="markdown-diagram-error">{error}{"\n"}{source}</pre>;
  }
  if (!svg) {
    return <div className="markdown-diagram-loading" aria-label="正在渲染图表" />;
  }
  return <div className="markdown-diagram" dangerouslySetInnerHTML={{ __html: svg }} />;
}
