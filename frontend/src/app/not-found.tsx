import Link from "next/link";

export default function NotFound() {
  return (
    <main className="not-found">
      <p className="section-kicker">404 / 页面不存在</p>
      <h1>这篇学习笔记尚未公开。</h1>
      <p>它可能属于私有空间、仍是笔记草稿、已进入回收站，或不再通过此地址提供访问。</p>
      <Link className="button button-primary" href="/">浏览公开知识</Link>
    </main>
  );
}
