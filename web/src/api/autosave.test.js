import test from "node:test";
import assert from "node:assert/strict";
import { AutosaveQueue, mergeTaskState, nodePayload } from "./autosave.js";

test("an edit during an in-flight save is persisted afterwards, in order", async () => {
  let release;
  const blocked = new Promise((resolve) => {
    release = resolve;
  });
  const writes = [],
    drafts = new Map();
  const queue = new AutosaveQueue({
    delay: 60000,
    write: async (_, payload) => {
      writes.push(payload.prompt);
      if (writes.length === 1) await blocked;
    },
    onDraft: (id, payload) =>
      payload ? drafts.set(id, payload) : drafts.delete(id),
  });
  queue.change(1, { prompt: "first" });
  const saved = queue.flush(1);
  queue.change(1, { prompt: "latest" });
  assert.equal(drafts.get("1").prompt, "latest");
  release();
  await saved;
  assert.deepEqual(writes, ["first", "latest"]);
  assert.equal(queue.dirty(), false);
  assert.equal(drafts.size, 0);
  queue.stopTimers();
});

test("failed writes retain drafts and can be retried without affecting other nodes", async () => {
  let fail = true;
  const drafts = new Map(),
    saved = [];
  const queue = new AutosaveQueue({
    delay: 60000,
    write: async (id, payload) => {
      if (id === "1" && fail) throw new Error("offline");
      saved.push(payload.prompt);
    },
    onDraft: (id, payload) =>
      payload ? drafts.set(id, payload) : drafts.delete(id),
  });
  queue.change(1, { prompt: "draft" });
  queue.change(2, { prompt: "independent" });
  await assert.rejects(queue.flush(1), /offline/);
  await queue.flush(2);
  assert.equal(drafts.get("1").prompt, "draft");
  assert.equal(queue.dirty(), true);
  fail = false;
  await queue.flush(1);
  assert.deepEqual(saved, ["independent", "draft"]);
  assert.equal(queue.dirty(), false);
  queue.stopTimers();
});

test("SSE updates never overwrite draft text, dimensions or layout; saves omit generated asset", () => {
  const node = {
    prompt: "my draft",
    params: "{}",
    posX: 12,
    posY: 40,
    width: 320,
    currentAssetId: 1,
  };
  const merged = mergeTaskState(node, {
    prompt: "stale",
    posX: 999,
    currentAssetId: 2,
    asset: { id: 2 },
    version: 3,
    generation: { status: "succeeded" },
  });
  assert.equal(merged.prompt, "my draft");
  assert.equal(merged.posX, 12);
  assert.equal(merged.width, 320);
  assert.equal(merged.currentAssetId, 2);
  assert.equal("assetId" in nodePayload(merged), false);
  assert.equal("currentAssetId" in nodePayload(merged), false);
});

test("a completed text task writes its result back only when the submitted instruction is unchanged", () => {
  const update = {
    version: 2,
    generation: {
      taskType: "text",
      status: "succeeded",
      nodePrompt: "write a title",
      resultText: "A Better Title",
    },
  };
  assert.equal(mergeTaskState({ prompt: "write a title" }, update).prompt, "A Better Title");
  assert.equal(mergeTaskState({ prompt: "new local draft" }, update).prompt, "new local draft");
});
