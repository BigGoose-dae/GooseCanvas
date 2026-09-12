// Serialize writes per node. Edits made during a request always get a later write.
export class AutosaveQueue {
  constructor({ write, onState = () => {}, onDraft = () => {}, delay = 600 }) {
    Object.assign(this, { write, onState, onDraft, delay });
    this.entries = new Map();
  }

  change(id, payload) {
    id = String(id);
    const entry = this.entries.get(id) || { revision: 0, saved: 0 };
    entry.payload = payload;
    entry.revision++;
    this.entries.set(id, entry);
    this.onDraft(id, payload);
    this.onState(id, "pending");
    clearTimeout(entry.timer);
    entry.timer = setTimeout(() => this.flush(id).catch(() => {}), this.delay);
  }

  async flush(id) {
    id = String(id);
    const entry = this.entries.get(id);
    if (!entry) return;
    clearTimeout(entry.timer);
    if (entry.promise) return entry.promise;
    entry.promise = (async () => {
      while (entry.saved < entry.revision) {
        const revision = entry.revision;
        const payload = entry.payload;
        this.onState(id, "saving");
        try {
          await this.write(id, payload);
        } catch (error) {
          this.onState(id, "error", error.message);
          throw error;
        }
        entry.saved = revision;
        if (this.entries.get(id) !== entry) return;
      }
      this.onDraft(id, null);
      this.onState(id, "saved");
    })();
    try {
      await entry.promise;
    } finally {
      entry.promise = null;
    }
  }

  flushAll() {
    return Promise.all([...this.entries.keys()].map((id) => this.flush(id)));
  }
  dirty() {
    return [...this.entries.values()].some(
      (entry) => entry.saved < entry.revision,
    );
  }
  remove(id) {
    id = String(id);
    clearTimeout(this.entries.get(id)?.timer);
    this.entries.delete(id);
    this.onDraft(id, null);
    this.onState(id, "saved");
  }
  stopTimers() {
    for (const entry of this.entries.values()) clearTimeout(entry.timer);
  }
}

export function nodePayload(node) {
  let params = {};
  try {
    params = JSON.parse(node.params || "{}") || {};
  } catch {
    /* Preserve valid defaults. */
  }
  return {
    title: node.title,
    prompt: node.prompt,
    modelKey: node.modelKey,
    params,
    posX: node.posX,
    posY: node.posY,
    width: node.width,
    height: node.height,
  };
}

// Task snapshots must never contain editable fields or reset XYFlow interaction state.
export function mergeTaskState(node, update) {
  const merged = {
    ...node,
    generation: update.generation,
    currentAssetId: update.currentAssetId,
    asset: update.asset,
    version: update.version,
  };
  if (
    update.generation?.status === "succeeded" &&
    update.generation?.taskType === "text" &&
    update.generation?.resultText &&
    node.prompt === update.generation.nodePrompt
  ) {
    merged.prompt = update.generation.resultText;
  }
  return merged;
}
