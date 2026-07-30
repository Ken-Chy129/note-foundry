import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

function source(path) {
  return readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
}

test("CSP permits only the GitHub avatar host in addition to local images", () => {
  const caddyfile = source("../deploy/Caddyfile");

  assert.match(caddyfile, /img-src 'self' data: https:\/\/avatars\.githubusercontent\.com;/);
  assert.doesNotMatch(caddyfile, /img-src[^;]*\shttps:;/);
});

test("the v0.1 interface declares Simplified Chinese and uses Chinese core copy", () => {
  const layout = source("src/app/layout.tsx");
  const home = source("src/app/page.tsx");
  const publicSearch = source("src/components/public/PublicSearch.tsx");
  const workspace = source("src/components/workspace/WorkspaceApp.tsx");
  const workspaceSearch = source("src/components/workspace/WorkspaceSearchDialog.tsx");
  const editor = source("src/components/workspace/EditorPane.tsx");
  const inspector = source("src/components/workspace/InspectorPanel.tsx");

  assert.match(layout, /<html lang="zh-CN">/);
  assert.match(home, /知识空间/);
  assert.match(publicSearch, /搜索公开笔记/);
  assert.match(workspaceSearch, /搜索笔记/);
  assert.match(editor, /已保存/);
  assert.match(inspector, /修订记录/);

  const interfaceCopy = `${home}\n${publicSearch}\n${workspace}\n${workspaceSearch}\n${editor}\n${inspector}`;
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

  assert.doesNotMatch(home, /href="\/workspace"/);
  assert.doesNotMatch(siteHeader, /href="\/workspace"/);
  assert.doesNotMatch(`${home}\n${siteHeader}`, /所有者工作区/);
  assert.match(home, /学习笔记/);
  assert.match(publicSearch, /搜索公开笔记/);
  assert.doesNotMatch(home, /catalog-count|home-intro-copy|<dl>/);
  assert.doesNotMatch(siteHeader, /<nav/);
});

test("the owner workspace lives only at /workspace", () => {
  const workspaceRoute = new URL("../src/app/workspace/page.tsx", import.meta.url);
  const legacyAppRoute = new URL("../src/app/app/page.tsx", import.meta.url);
  const identityHTTP = source("../backend/internal/identity/http.go");
  const apiMain = source("../backend/cmd/api/main.go");

  assert.equal(existsSync(workspaceRoute), true);
  assert.equal(existsSync(legacyAppRoute), false);
  assert.match(identityHTTP, /postLoginPath = "\/workspace"/);
  assert.match(apiMain, /PostLoginPath:\s+"\/workspace"/);
  assert.doesNotMatch(`${identityHTTP}\n${apiMain}`, /"\/app"/);
});

test("the workspace note list owns a shrinkable vertical scroll area", () => {
  const styles = source("src/app/globals.css");

  assert.match(
    styles,
    /\.workspace-sidebar\s*\{[^}]*min-height:\s*0;[^}]*overflow:\s*hidden;/s
  );
  assert.match(
    styles,
    /\.note-navigation\s*\{[^}]*min-height:\s*0;[^}]*flex:\s*1 1 auto;[^}]*overflow-y:\s*auto;/s
  );
});

test("workspace search opens from the sidebar and keeps results in a bounded scroller", () => {
  const workspace = source("src/components/workspace/WorkspaceApp.tsx");
  const sidebar = source("src/components/workspace/KnowledgeSidebar.tsx");
  const searchDialogPath = new URL("../src/components/workspace/WorkspaceSearchDialog.tsx", import.meta.url);
  const styles = source("src/app/globals.css");

  assert.doesNotMatch(workspace, /className="workspace-search-trigger"/);
  assert.match(workspace, /<WorkspaceSearchDialog/);
  assert.match(sidebar, /className="sidebar-search-button"/);
  assert.match(sidebar, /onOpenSearch/);
  assert.equal(existsSync(searchDialogPath), true);

  const searchDialog = source("src/components/workspace/WorkspaceSearchDialog.tsx");
  assert.match(searchDialog, /aria-live="polite"/);
  assert.match(searchDialog, /正在搜索/);
  assert.match(searchDialog, /没有找到相关笔记/);
  assert.match(
    styles,
    /\.workspace-search-dialog\s*\{[^}]*display:\s*flex;[^}]*overflow:\s*hidden;/s
  );
  assert.match(
    styles,
    /\.workspace-search-results\s*\{[^}]*min-height:\s*0;[^}]*overflow-y:\s*auto;/s
  );
});

test("the workspace sidebar renders notes in an accessible nested directory tree", () => {
  const sidebar = source("src/components/workspace/KnowledgeSidebar.tsx");
  const treePath = new URL("../src/components/workspace/WorkspaceDirectoryTree.tsx", import.meta.url);
  const styles = source("src/app/globals.css");

  assert.match(sidebar, /<WorkspaceDirectoryTree/);
  assert.doesNotMatch(sidebar, /props\.notes\.map/);
  assert.equal(existsSync(treePath), true);

  const tree = source("src/components/workspace/WorkspaceDirectoryTree.tsx");
  assert.match(tree, /role="tree"/);
  assert.match(tree, /role="group"/);
  assert.match(tree, /aria-expanded=/);
  assert.match(tree, /directoryAncestorIds/);
  assert.match(styles, /\.directory-tree-children\s*\{[^}]*border-left:/s);
});
