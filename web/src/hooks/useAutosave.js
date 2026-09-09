import { useEffect, useMemo, useState } from "react";
import { api } from "../api/client";
import { AutosaveQueue } from "../api/autosave";

export function useAutosave(workspaceId) {
  const [states, setStates] = useState({});
  const key = `goose-canvas-drafts:${workspaceId}`;
  const drafts = useMemo(() => {
    try {
      return JSON.parse(localStorage.getItem(key) || "{}");
    } catch {
      return {};
    }
  }, [key]);
  const queue = useMemo(
    () =>
      new AutosaveQueue({
        write: (id, payload) => api.updateNode(id, payload),
        onState: (id, status, error) =>
          setStates((current) => ({ ...current, [id]: { status, error } })),
        onDraft: (id, payload) => {
          if (payload) drafts[id] = payload;
          else delete drafts[id];
          try {
            localStorage.setItem(key, JSON.stringify(drafts));
          } catch {
            /* Network saves remain available. */
          }
        },
      }),
    [key, drafts],
  );

  useEffect(() => {
    const beforeUnload = (event) => {
      if (queue.dirty()) {
        queue.flushAll().catch(() => {});
        event.preventDefault();
        event.returnValue = "";
      }
    };
    const online = () => queue.flushAll().catch(() => {});
    const hide = () => {
      if (document.visibilityState === "hidden") online();
    };
    window.addEventListener("beforeunload", beforeUnload);
    window.addEventListener("online", online);
    document.addEventListener("visibilitychange", hide);
    return () => {
      queue.stopTimers();
      queue.flushAll().catch(() => {});
      window.removeEventListener("beforeunload", beforeUnload);
      window.removeEventListener("online", online);
      document.removeEventListener("visibilitychange", hide);
    };
  }, [queue]);

  const all = Object.values(states);
  const status = all.some((s) => s.status === "error")
    ? "error"
    : all.some((s) => s.status === "saving")
      ? "saving"
      : all.some((s) => s.status === "pending")
        ? "pending"
        : "saved";
  return { queue, states, status, drafts };
}
