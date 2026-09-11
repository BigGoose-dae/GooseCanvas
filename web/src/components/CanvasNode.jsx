import { useEffect, useState } from "react";
import { Handle, Position } from "@xyflow/react";
import { api } from "../api/client";
import { useLanguage } from "../i18n/LanguageContext";

const icons = { text: "T", image: "▧", video: "▶" };

export default function CanvasNode({ id, data, selected }) {
  const { t } = useLanguage();
  const labels = { text: t("text"), image: t("image"), video: t("video") };
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
            placeholder={t("textPlaceholder")}
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
            {node.prompt || t("textPlaceholder")}
          </div>
        )
      ) : (
        <div className="workshop-media">
          {node.asset?.url && node.nodeType === "image" && (
            <img
              src={node.asset.url}
              alt={node.title || t("image")}
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
                  aria-label={t("playVideo")}
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
              <span>{running ? t("generating") : labels[node.nodeType]}</span>
            </div>
          )}
          {failed && (
            <div className="workshop-failed" title={generation.errorMessage}>
              ! {generation.errorMessage || t("generationFailed")}
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
                saved: t("saved"),
                pending: t("savePending"),
                saving: t("saving"),
                error: t("saveFailed"),
              }[data.saveState]
            }
          </span>
          <button onClick={() => data.copy(id)}>{t("copy")}</button>
          {node.nodeType !== "text" && (
            <button onClick={() => data.history(id)}>{t("history")}</button>
          )}
          {node.currentAssetId && (
            <a href={api.downloadURL(node.currentAssetId)} download>
              ↓ {t("download")}
            </a>
          )}
          {data.saveState === "error" && (
            <button onClick={() => data.save(id)}>{t("retrySave")}</button>
          )}
        </div>
      )}
      {running && (
        <div className="workshop-progress">
          <i />
          {t("processing")}
        </div>
      )}
      <button
        className="workshop-resize nodrag"
        aria-label={t("resizeNode")}
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
