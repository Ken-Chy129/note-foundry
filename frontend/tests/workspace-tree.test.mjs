import assert from "node:assert/strict";
import test from "node:test";
import { buildDirectoryTree, directoryAncestorIds, initialCollapsedDirectoryIds } from "../src/components/workspace/workspaceTree.ts";

const directories = [
  { id: "network", spaceId: "space-1", parentId: null, name: "计算机网络" },
  { id: "linux", spaceId: "space-1", parentId: null, name: "Linux" },
  { id: "tcp", spaceId: "space-1", parentId: "network", name: "TCP" },
  { id: "congestion", spaceId: "space-1", parentId: "tcp", name: "拥塞控制" },
  { id: "orphan", spaceId: "space-1", parentId: "missing", name: "失联目录" }
];

function note(id, title, directoryId) {
  return { id, spaceId: "space-1", directoryId, title, slug: id, markdown: "", version: 1, published: null };
}

test("buildDirectoryTree groups root and nested notes without losing orphaned content", () => {
  const tree = buildDirectoryTree(directories, [
    note("root-note", "根目录笔记", null),
    note("network-note", "网络总览", "network"),
    note("tcp-note", "三次握手", "tcp"),
    note("congestion-note", "拥塞窗口", "congestion"),
    note("missing-note", "目录已失效", "missing")
  ]);

  assert.deepEqual(tree.rootNotes.map((item) => item.id), ["root-note", "missing-note"]);
  assert.deepEqual(tree.directories.map((item) => item.directory.id), ["network", "orphan", "linux"]);

  const network = tree.directories[0];
  assert.equal(network.totalNotes, 3);
  assert.deepEqual(network.notes.map((item) => item.id), ["network-note"]);
  assert.deepEqual(network.children[0].notes.map((item) => item.id), ["tcp-note"]);
  assert.deepEqual(network.children[0].children[0].notes.map((item) => item.id), ["congestion-note"]);
});

test("directoryAncestorIds returns the selected directory path from root to leaf", () => {
  assert.deepEqual(directoryAncestorIds(directories, "congestion"), ["network", "tcp", "congestion"]);
  assert.deepEqual(directoryAncestorIds(directories, "missing"), []);
  assert.deepEqual(directoryAncestorIds(directories, null), []);
});

test("initialCollapsedDirectoryIds starts with every directory closed except the selected path", () => {
  assert.deepEqual([...initialCollapsedDirectoryIds(directories)], ["network", "linux", "tcp", "congestion", "orphan"]);
  assert.deepEqual([...initialCollapsedDirectoryIds(directories, ["network", "tcp"])], ["linux", "congestion", "orphan"]);
});
