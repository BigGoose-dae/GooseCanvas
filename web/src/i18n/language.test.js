import test from "node:test";
import assert from "node:assert/strict";
import { preferredLanguage } from "./language.js";

test("saved language takes precedence over the browser language", () => {
  assert.equal(preferredLanguage("en", "zh-CN"), "en");
  assert.equal(preferredLanguage("zh", "en-US"), "zh");
});

test("browser language selects Chinese only for Chinese locales", () => {
  assert.equal(preferredLanguage(null, "zh-Hans-CN"), "zh");
  assert.equal(preferredLanguage(null, "en-US"), "en");
  assert.equal(preferredLanguage(null, ""), "en");
});
