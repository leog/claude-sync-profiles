// Tests for the npm postinstall installer.
//
// These exist because the redirect allowlist silently broke the installer: it
// pinned objects.githubusercontent.com, GitHub moved release downloads to
// release-assets.githubusercontent.com, and every install failed. Nobody
// noticed for months because npm publishing was also broken, so the change
// never reached a user. Run with: node --test

const { test } = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");

// install.js runs its install() on require, so pull the pure helpers out of the
// source rather than importing it.
function loadHelper(name) {
  const src = fs.readFileSync(path.join(__dirname, "install.js"), "utf8");
  const consts = src.match(
    /const ALLOWED_HOSTS = \[[^\]]*\];\nconst ALLOWED_HOST_SUFFIX = "[^"]*";/
  );
  const fn = src.match(new RegExp(`function ${name}\\(url\\) \\{[\\s\\S]*?\\n\\}`));
  assert.ok(consts, "ALLOWED_HOSTS/ALLOWED_HOST_SUFFIX not found in install.js");
  assert.ok(fn, `${name} not found in install.js`);
  // eslint-disable-next-line no-eval
  return eval(`${consts[0]}\n${fn[0]}\n${name}`);
}

test("isAllowedHost accepts GitHub release download hosts", () => {
  const isAllowedHost = loadHelper("isAllowedHost");

  for (const url of [
    "https://github.com/leog/claude-sync-profiles/releases/download/v1.0.0/x",
    "https://api.github.com/repos/leog/claude-sync-profiles/releases/latest",
    // The historical asset CDN.
    "https://objects.githubusercontent.com/foo",
    // The current asset CDN — the host whose absence broke every install.
    "https://release-assets.githubusercontent.com/foo",
  ]) {
    assert.strictEqual(isAllowedHost(url), true, `should allow ${url}`);
  }
});

test("isAllowedHost rejects lookalike and untrusted hosts", () => {
  const isAllowedHost = loadHelper("isAllowedHost");

  for (const url of [
    // Suffix match must require the leading dot.
    "https://evilgithubusercontent.com/foo",
    // Domain must be the parent, not a prefix of someone else's.
    "https://githubusercontent.com.evil.com/foo",
    "https://github.com.evil.com/foo",
    "https://evil.com/foo",
    // A downgrade to http would defeat the point of verifying a checksum.
    "http://github.com/foo",
    "http://release-assets.githubusercontent.com/foo",
    "not a url",
  ]) {
    assert.strictEqual(isAllowedHost(url), false, `should reject ${url}`);
  }
});
