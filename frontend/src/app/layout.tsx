import type { Metadata } from "next";
import "@fontsource-variable/ibm-plex-sans/index.css";
import "@fontsource-variable/newsreader/index.css";
import "katex/dist/katex.min.css";
import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "NoteFoundry — 个人学习工作台",
    template: "%s — NoteFoundry"
  },
  description: "面向单一知识所有者的可靠学习笔记工作台。"
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="zh-CN">
      <body>{children}</body>
    </html>
  );
}
