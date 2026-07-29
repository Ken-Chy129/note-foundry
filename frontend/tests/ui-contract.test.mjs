import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(path) {
  return readFileSync(new URL(`../${path}`, import.meta.url), "utf8");
}

test("CSP permits only the GitHub avatar host in addition to local images", () => {
  const caddyfile = source("../deploy/Caddyfile");

  assert.match(caddyfile, /img-src 'self' data: https:\/\/avatars\.githubusercontent\.com;/);
  assert.doesNotMatch(caddyfile, /img-src[^;]*\shttps:;/);
});
