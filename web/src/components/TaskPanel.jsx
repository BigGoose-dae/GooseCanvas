import { useEffect, useState } from "react";
import { api } from "../api/client";

export const taskLabels = {
  pending: "排队中",
  submitting: "提交中",
  processing: "生成中",
  archiving: "归档中",
  succeeded: "已完成",
  failed: "失败",
};
export const isRunning = (status) =>
  ["pending", "submitting", "processing", "archiving"].includes(status);
const date = (value) =>
  value ? new Date(value).toLocaleString("zh-CN", { hour12: false }) : "—";

function GenerationCard({ task, onRetry, onLocate, detailed = false }) {
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
          {task.nodeTitle || `节点 ${task.nodeId}`}
        </button>
        <span className={`task-badge ${task.status}`}>
          {taskLabels[task.status] || task.status}
        </span>
      </div>
      <small>
        #{task.id} · {task.modelKey}
      </small>
      <p className="generation-prompt">{task.prompt}</p>
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
              alt={`第 ${task.id} 次生成结果`}
              loading="lazy"
            />
          </a>
        ))}
      <small>创建于 {date(task.createdAt)}</small>
      {task.finishedAt && <small>结束于 {date(task.finishedAt)}</small>}
      {detailed && (
        <details>
          <summary>生成参数与任务信息</summary>
          <pre>{JSON.stringify(JSON.parse(task.params || "{}"), null, 2)}</pre>
          {task.externalTaskId && (
            <small>第三方任务：{task.externalTaskId}</small>
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
            ↓ 下载{task.taskType === "video" ? "视频" : "图片"}
          </a>
        )}
        {task.status === "failed" && (
          <button disabled={busy} onClick={retry}>
            {busy
              ? "处理中…"
              : task.retryMode === "archive"
                ? "重新归档"
                : task.retryMode === "query"
                  ? "继续查询原任务"
                  : "重新提交生成"}
          </button>
        )}
      </div>
      {task.status === "failed" && !task.retryMode && (
        <small>
          重新提交会创建新任务，可能再次计费；提交中断时请先核实第三方记录。
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
    <aside className="task-panel" aria-label={nodeId ? "生成历史" : "任务队列"}>
      <div className="task-panel-heading">
        <div>
          <h2>{nodeId ? "生成历史" : "任务队列"}</h2>
          <p>
            {nodeId
              ? historyNode.title
              : `${active} 个进行中 · 最大并发 ${concurrency}`}
          </p>
        </div>
        <button aria-label="关闭任务面板" onClick={onClose}>
          ×
        </button>
      </div>
      <div className={`connection-state ${connection}`} role="status">
        {connection === "connected"
          ? "● 状态实时同步"
          : connection === "connecting"
            ? "○ 正在连接…"
            : "○ 连接中断，正在重连…"}
      </div>
      {!nodeId && (
        <div className="task-filters">
          {[
            ["all", "全部"],
            ["active", "进行中"],
            ["succeeded", "完成"],
            ["failed", "失败"],
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
              ? "加载中…"
              : nodeId
                ? "这个节点还没有生成记录"
                : "暂无任务，选择图片或视频节点开始生成"}
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
            {loading ? "加载中…" : "加载更早记录"}
          </button>
        )}
        {!nodeId && list.length > 0 && (
          <p className="panel-footnote">
            显示全部进行中任务和最近 100 条记录。节点历史可查看更早记录。
          </p>
        )}
        {error && <p className="task-error">{error}</p>}
      </div>
    </aside>
  );
}
