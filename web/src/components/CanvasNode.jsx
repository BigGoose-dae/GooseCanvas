import { useEffect, useState } from "react";
import { Handle, Position } from "@xyflow/react";
import { api } from "../api/client";

const labels = { text: "文本", image: "图片", video: "视频" };
const icons = { text: "T", image: "▧", video: "▶" };

export default function CanvasNode({ id, data, selected }) {
  const node = data.node;
  const generation = node.generation;
  const running = ["pending", "submitting", "processing", "archiving"].includes(
    generation?.status,
  );
  const failed = generation?.status === "failed";
  const [editingText, setEditingText] = useState(false);
  const [videoActivated, setVideoActivated] = useState(false);

  useEffect(() => {
    setVideoActivated(false);
  }, [node.asset?.url]);

  const beginTextEditing = (event) => {
    event.stopPropagation();
    setEditingText(true);
    data.editing(id, true);
  };

  const finishTextEditing = () => {
    setEditingText(false);
    data.editing(id, false);
    data.save(id);
  };

  return (
    <div
      className={`workshop-node ${node.nodeType} ${selected ? "selected" : ""} ${node.asset?.url ? "media-filled" : ""} ${running ? "generating" : ""}`}
    >
      <Handle
        type="target"
        position={Position.Left}
        id="left"
        className="workshop-handle left"
      />
      <div className="workshop-node-title">
        {node.title || labels[node.nodeType]}
      </div>
      {node.nodeType === "text" ? (
        editingText ? (
          <textarea
            autoFocus
            className="workshop-text nodrag nowheel"
            value={node.prompt || ""}
            placeholder="在此输入文本…"
            onPointerDown={(event) => event.stopPropagation()}
            onChange={(event) =>
              data.change(id, { prompt: event.target.value })
            }
            onBlur={finishTextEditing}
            onKeyDown={(event) => {
              if (event.key === "Escape") event.currentTarget.blur();
            }}
          />
        ) : (
          <div
            className={`workshop-text-view ${node.prompt ? "" : "empty"}`}
            onDoubleClick={beginTextEditing}
          >
            {node.prompt || "在此输入文本…"}
          </div>
        )
      ) : (
        <div className="workshop-media">
          {node.asset?.url && node.nodeType === "image" && (
            <img
              src={node.asset.url}
              alt={node.title || "图片"}
              draggable="false"
            />
          )}
          {node.asset?.url &&
            node.nodeType === "video" &&
            (videoActivated ? (
              <video
                src={node.asset.url}
                controls
                autoPlay
                muted
                playsInline
                preload="metadata"
                className="nowheel"
                onPointerDown={(event) => event.stopPropagation()}
              />
            ) : (
              <div className="workshop-video-poster">
                <video
                  src={node.asset.url}
                  muted
                  playsInline
                  preload="metadata"
                  draggable="false"
                />
                <button
                  type="button"
                  className="nodrag"
                  aria-label="加载并播放视频"
                  onPointerDown={(event) => event.stopPropagation()}
                  onClick={(event) => {
                    event.stopPropagation();
                    setVideoActivated(true);
                  }}
                >
                  ▶
                </button>
              </div>
            ))}
          {!node.asset?.url && (
            <div className="workshop-placeholder">
              <b>{icons[node.nodeType]}</b>
              <span>{running ? "生成处理中" : labels[node.nodeType]}</span>
            </div>
          )}
          {failed && (
            <div className="workshop-failed" title={generation.errorMessage}>
              ! {generation.errorMessage || "生成失败"}
            </div>
          )}
        </div>
      )}
      {selected && (
        <div
          className="node-quick-actions nodrag"
          onPointerDown={(event) => event.stopPropagation()}
        >
          <span className={`node-save-state ${data.saveState}`} role="status">
            {
              {
                saved: "已保存",
                pending: "待保存",
                saving: "保存中",
                error: "保存失败",
              }[data.saveState]
            }
          </span>
          <button onClick={() => data.copy(id)}>复制</button>
          {node.nodeType !== "text" && (
            <button onClick={() => data.history(id)}>历史</button>
          )}
          {node.currentAssetId && (
            <a href={api.downloadURL(node.currentAssetId)} download>
              ↓ 下载
            </a>
          )}
          {data.saveState === "error" && (
            <button onClick={() => data.save(id)}>重试保存</button>
          )}
        </div>
      )}
      {running && (
        <div className="workshop-progress">
          <i />
          处理中
        </div>
      )}
      <button
        className="workshop-resize nodrag"
        aria-label="调整节点大小"
        onPointerDown={(event) => data.resize(event, id)}
      />
      <Handle
        type="source"
        position={Position.Right}
        id="right"
        className="workshop-handle right"
      />
    </div>
  );
}
