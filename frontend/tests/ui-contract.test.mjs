import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(path) {
  return readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
}

test("CSP permits the curated remote image hosts used by imported notes", () => {
  const caddyfile = source("../deploy/Caddyfile");

  for (const host of [
    "ask.qcloudimg.com",
    "assets.leetcode-cn.com",
    "cdn.ken-chy129.cn",
    "gitee.com",
    "guide-blog-images.oss-cn-shenzhen.aliyuncs.com",
    "img-blog.csdn.net",
    "img-blog.csdnimg.cn",
    "img1.tbcdn.cn",
    "imgconvert.csdnimg.cn",
    "kdi72slpqf.feishu.cn",
    "mmbiz.qpic.cn",
    "my-blog-to-use.oss-cn-beijing.aliyuncs.com",
    "oss.javaguide.cn",
    "p1-juejin.byteimg.com",
    "p3-juejin.byteimg.com",
    "p6-juejin.byteimg.com",
    "pic1.zhimg.com",
    "pic2.zhimg.com",
    "pic3.zhimg.com",
    "pic4.zhimg.com",
    "picx.zhimg.com",
    "seazean.oss-cn-beijing.aliyuncs.com"
  ]) {
    assert.ok(caddyfile.includes(`https://${host}`), `missing CSP image host ${host}`);
  }
  assert.ok(caddyfile.includes("https://avatars.githubusercontent.com"));
  assert.doesNotMatch(caddyfile, /img-src[^;]*\shttps:;/);
});

test("the v0.1 interface declares Simplified Chinese and uses Chinese core copy", () => {
  const layout = source("src/app/layout.tsx");
  const home = source("src/app/page.tsx");
  const publicSearch = source("src/components/public/PublicSearch.tsx");
  const workspace = source("src/components/workspace/WorkspaceApp.tsx");
  const editor = source("src/components/workspace/EditorPane.tsx");
  const inspector = source("src/components/workspace/InspectorPanel.tsx");

  assert.match(layout, /<html lang="zh-CN">/);
  assert.match(home, /知识空间/);
  assert.match(publicSearch, /搜索公开笔记/);
  assert.match(workspace, /搜索工作区/);
  assert.match(editor, /已保存/);
  assert.match(inspector, /修订记录/);

  const interfaceCopy = `${home}\n${publicSearch}\n${workspace}\n${editor}\n${inspector}`;
  for (const english of [
    "Knowledge spaces",
    "Owner workspace",
    "Search the workspace",
    "Select a Learning Note",
    "Moved to Trash.",
    "Details appear when a note is selected."
  ]) {
    assert.doesNotMatch(interfaceCopy, new RegExp(english.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }
});

test("the public home is a compact reading index without owner or catalog decoration", () => {
  const home = source("src/app/page.tsx");
  const publicSearch = source("src/components/public/PublicSearch.tsx");
  const siteHeader = source("src/components/public/SiteHeader.tsx");

  assert.doesNotMatch(home, /href="\/app"/);
  assert.doesNotMatch(siteHeader, /href="\/app"/);
  assert.doesNotMatch(`${home}\n${siteHeader}`, /所有者工作区/);
  assert.match(home, /学习笔记/);
  assert.match(publicSearch, /搜索公开笔记/);
  assert.doesNotMatch(home, /catalog-count|home-intro-copy|<dl>/);
  assert.doesNotMatch(siteHeader, /<nav/);
});
