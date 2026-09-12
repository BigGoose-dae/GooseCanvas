import { useEffect, useState } from "react";
import { api } from "../api/client";
import { useLanguage } from "../i18n/LanguageContext";

export const isRunning = (status) =>
  ["pending", "submitting", "processing", "archiving"].includes(status);

function GenerationCard({ task, onRetry, onLocate, detailed = false }) {
  const { t, locale } = useLanguage();
  const taskLabels = {
    pending: t("taskPending"),
    submitting: t("taskSubmitting"),
    processing: t("taskProcessing"),
    archiving: t("taskArchiving"),
    succeeded: t("taskSucceeded"),
    failed: t("taskFailed"),
  };
  const date = (value) =>
    value ? new Date(value).toLocaleString(locale, { hour12: false }) : "—";
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const retry = async () => {
    setBusy(true);
    setError("");
    try {
      await api.retry(task.id);
      await onRetry?.();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <article className="generation-card">
      <div className="generation-heading">
        <button
          className="text-button"
          onClick={() => onLocate?.(task.nodeId)}
          disabled={!onLocate}
        >
          {task.nodeTitle || t("nodeNumber", { id: task.nodeId })}
        </button>
        <span className={`task-badge ${task.status}`}>
          {taskLabels[task.status] || task.status}
        </span>
      </div>
      <small>
        #{task.id} · {task.modelKey}
      </small>
      <p className="generation-prompt">{task.prompt}</p>
      {detailed && task.taskType === "text" && task.resultText && (
        <div className="history-text">
          <small>{t("generatedText")}</small>
          <p>{task.resultText}</p>
        </div>
      )}
      {detailed &&
        task.asset &&
        (task.taskType === "video" ? (
          <video
            className="history-preview"
            src={task.asset.url}
            controls
            preload="metadata"
          />
        ) : (
          <a href={task.asset.url} target="_blank" rel="noreferrer">
            <img
              className="history-preview"
              src={task.asset.url}
              alt={t("resultAlt", { id: task.id })}
              loading="lazy"
            />
          </a>
        ))}
      <small>{t("createdAt", { date: date(task.createdAt) })}</small>
      {task.finishedAt && <small>{t("finishedAt", { date: date(task.finishedAt) })}</small>}
      {detailed && (
        <details>
          <summary>{t("taskDetails")}</summary>
          <pre>{JSON.stringify(JSON.parse(task.params || "{}"), null, 2)}</pre>
          {task.externalTaskId && (
            <small>{t("externalTask", { id: task.externalTaskId })}</small>
          )}
        </details>
      )}
      {(task.errorMessage || task.taskError) && (
        <p className="task-error">{task.errorMessage || task.taskError}</p>
      )}
      <div className="generation-actions">
        {task.asset && (
          <a
            className="download-button"
            href={api.downloadURL(task.asset.id)}
            download
          >
            ↓ {t("downloadMedia", { type: t(task.taskType) })}
          </a>
        )}
        {task.status === "failed" && (
          <button disabled={busy} onClick={retry}>
            {busy
              ? t("processing")
              : task.retryMode === "archive"
                ? t("retryArchive")
                : task.retryMode === "query"
                  ? t("continueQuery")
                  : t("resubmit")}
          </button>
        )}
      </div>
      {task.status === "failed" && !task.retryMode && (
        <small>
          {t("retryWarning")}
        </small>
      )}
      {error && <p className="task-error">{error}</p>}
    </article>
  );
}

export default function TaskPanel({
  tasks,
  connection,
  concurrency,
  historyNode,
  onClose,
  onLocate,
}) {
  const { t } = useLanguage();
  const [filter, setFilter] = useState("all");
  const [history, setHistory] = useState([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const nodeId = historyNode?.id;
  const refresh = async (before) => {
    if (!nodeId) return;
    setLoading(true);
    setError("");
    try {
      const response = await api.history(nodeId, before);
      setHistory((current) =>
        before ? [...current, ...response.items] : response.items,
      );
      setHasMore(response.hasMore);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    let live = true;
    setHistory([]);
    setHasMore(false);
    setError("");
    if (!nodeId) return;
    setLoading(true);
    api
      .history(nodeId)
      .then((data) => {
        if (live) {
          setHistory(data.items);
          setHasMore(data.hasMore);
        }
      })
      .catch((err) => {
        if (live) setError(err.message);
      })
      .finally(() => {
        if (live) setLoading(false);
      });
    return () => {
      live = false;
    };
  }, [nodeId]);
  // Merge pushed status into loaded history while preserving older pages.
  useEffect(() => {
    if (!nodeId) return;
    setHistory((current) => {
      const merged = new Map(current.map((task) => [task.id, task]));
      for (const task of tasks)
        if (task.nodeId === Number(nodeId)) merged.set(task.id, task);
      return [...merged.values()].sort((a, b) => b.id - a.id);
    });
  }, [tasks, nodeId]);
  const active = tasks.filter((task) => isRunning(task.status)).length;
  const list = nodeId
    ? history
    : tasks.filter(
        (task) =>
          filter === "all" ||
          (filter === "active"
            ? isRunning(task.status)
            : task.status === filter),
      );
  return (
    <aside className="task-panel" aria-label={nodeId ? t("generationHistory") : t("taskQueue")}>
      <div className="task-panel-heading">
        <div>
          <h2>{nodeId ? t("generationHistory") : t("taskQueue")}</h2>
          <p>
            {nodeId
              ? historyNode.title
              : t("activeCount", { active, concurrency })}
          </p>
        </div>
        <button aria-label={t("closeTaskPanel")} onClick={onClose}>
          ×
        </button>
      </div>
      <div className={`connection-state ${connection}`} role="status">
        {connection === "connected"
          ? t("stateSynced")
          : connection === "connecting"
            ? t("connecting")
            : t("connectionLost")}
      </div>
      {!nodeId && (
        <div className="task-filters">
          {[
            ["all", t("all")],
            ["active", t("active")],
            ["succeeded", t("completed")],
            ["failed", t("taskFailed")],
          ].map(([value, label]) => (
            <button
              key={value}
              className={filter === value ? "active" : ""}
              onClick={() => setFilter(value)}
            >
              {label}
            </button>
          ))}
        </div>
      )}
      <div className="task-panel-list">
        {!list.length && (
          <p className="panel-empty">
            {loading
              ? t("loading")
              : nodeId
                ? t("noHistory")
                : t("noTasks")}
          </p>
        )}
        {list.map((task) => (
          <GenerationCard
            key={task.id}
            task={task}
            detailed={Boolean(nodeId)}
            onLocate={onLocate}
            onRetry={nodeId ? () => refresh() : undefined}
          />
        ))}
        {nodeId && hasMore && (
          <button
            className="load-more"
            disabled={loading}
            onClick={() => refresh(history.at(-1)?.id)}
          >
            {loading ? t("loading") : t("loadEarlier")}
          </button>
        )}
        {!nodeId && list.length > 0 && (
          <p className="panel-footnote">
            {t("taskFootnote")}
          </p>
        )}
        {error && <p className="task-error">{error}</p>}
      </div>
    </aside>
  );
}
