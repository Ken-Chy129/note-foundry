import assert from "node:assert/strict";
import test from "node:test";
import { buildPublicSpaceIndex } from "../src/components/public/publicSpaceIndex.ts";

const directories = [
  { id: "jvm", spaceId: "space-1", parentId: null, name: "JVM" },
  { id: "java", spaceId: "space-1", parentId: null, name: "Java" },
  { id: "graal", spaceId: "space-1", parentId: "jvm", name: "GraalVM" },
  { id: "orphan", spaceId: "space-1", parentId: "missing", name: "失联目录" }
];

function note(id, title, directoryId) {
  return {
    id,
    spaceId: "space-1",
    directoryId,
    title,
    slug: id,
    markdown: `# ${title}`,
    publishedAt: "2026-07-31T09:00:00Z"
  };
}

test("buildPublicSpaceIndex preserves the directory hierarchy and descendant note counts", () => {
  const index = buildPublicSpaceIndex(directories, [
    note("root", "根目录笔记", null),
    note("jvm-z", "字节码", "jvm"),
    note("jvm-a", "安全点", "jvm"),
    note("graal-note", "概念", "graal"),
    note("missing", "目录已失效", "missing")
  ]);

  assert.deepEqual(index.rootNotes.map((item) => item.id), ["root", "missing"]);
  assert.deepEqual(index.directories.map((item) => item.directory.id), ["orphan", "java", "jvm"]);

  const jvm = index.directories.find((item) => item.directory.id === "jvm");
  assert.equal(jvm.totalNotes, 3);
  assert.equal(jvm.sectionId, "目录-jvm");
  assert.deepEqual(jvm.notes.map((item) => item.title), ["安全点", "字节码"]);
  assert.equal(jvm.children[0].sectionId, "目录-jvm-graalvm");
  assert.equal(jvm.children[0].totalNotes, 1);
});

test("buildPublicSpaceIndex assigns duplicate readable paths deterministic suffixes", () => {
  const index = buildPublicSpaceIndex([
    { id: "first", spaceId: "space-1", parentId: null, name: "Java 基础" },
    { id: "second", spaceId: "space-1", parentId: null, name: "Java-基础" }
  ], []);

  assert.deepEqual(index.directories.map((item) => item.sectionId), ["目录-java-基础", "目录-java-基础-2"]);
});
