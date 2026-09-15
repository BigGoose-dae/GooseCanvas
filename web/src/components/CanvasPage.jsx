import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ReactFlow,
  Background,
  SelectionMode,
  addEdge,
  useEdgesState,
  useNodesState,
} from "@xyflow/react";
import { api } from "../api/client";
import Brand from "./Brand";
import CanvasNode from "./CanvasNode";
import ThemeToggle from "./ThemeToggle";
import { useTheme } from "../theme/ThemeContext";
import { useAutosave } from "../hooks/useAutosave";
import { nodePayload, mergeTaskState } from "../api/autosave";
import TaskPanel, { isRunning } from "./TaskPanel";
import AssetLibraryPanel from "./AssetLibraryPanel";
import LanguageToggle from "./LanguageToggle";
import { useLanguage } from "../i18n/LanguageContext";
import { AUDIO_GENERATION_ENABLED } from "../features";

const nodeTypes = { canvasNode: CanvasNode };
const clamp = (value, min, max) => Math.max(min, Math.min(max, value));

export default function CanvasPage() {
  const { id } = useParams();
  const { theme } = useTheme();
  const { t } = useLanguage();
  const labels = { text: t("text"), image: t("image"), audio: t("audio"), video: t("video") };
  const navigate = useNavigate();
  const {
    queue,
    states: saveStates,
    status: saveStatus,
    drafts,
  } = useAutosave(id);
  const [loaded, setLoaded] = useState(false);
  const [tasks, setTasks] = useState([]);
  const [connection, setConnection] = useState("connecting");
  const [concurrency, setConcurrency] = useState(4);
  const [panel, setPanel] = useState(null);
  const [submitting, setSubmitting] = useState(new Set());
  const wrapperRef = useRef(null),
    uploadRef = useRef(null),
    flowRef = useRef(null);
  const rawRef = useRef(new Map()),
    edgeRef = useRef([]),
    connectRef = useRef(null);
  const rightPointerRef = useRef({ down: false, moved: false, x: 0, y: 0 });
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [workspace, setWorkspace] = useState(null),
    [models, setModels] = useState([]),
    [status, setStatus] = useState(null);
  const [message, setMessage] = useState(""),
    [messageKind, setMessageKind] = useState("error"),
    [selectedId, setSelectedId] = useState(null),
    [spacePressed, setSpacePressed] = useState(false);
  const showMessage = useCallback((text, kind = "error") => {
    setMessage(text);
    setMessageKind(kind);
  }, []);
  useEffect(() => {
    if (!message || messageKind !== "success") return undefined;
    const timer = window.setTimeout(() => setMessage(""), 3000);
    return () => window.clearTimeout(timer);
  }, [message, messageKind]);
  const [viewport, setViewportState] = useState({ x: 0, y: 0, zoom: 1 });
  const [contextMenu, setContextMenu] = useState({
    visible: false,
    mode: "root",
    x: 0,
    y: 0,
    flowX: 0,
    flowY: 0,
    sourceId: null,
  });
  const [nodeMenu, setNodeMenu] = useState({
    visible: false,
    mode: "root",
    x: 0,
    y: 0,
    nodeId: null,
    name: "",
  });

  const toFlowNode = useCallback(
    (node) => ({
      id: String(node.id),
      type: "canvasNode",
      position: { x: node.posX || 0, y: node.posY || 0 },
      style: {
        width: node.width || (node.nodeType === "text" ? 320 : 280),
        height: node.height || 220,
      },
      data: { node },
    }),
    [],
  );
  useEffect(() => {
    let live = true;
    Promise.all([api.graph(id), api.models(), api.status()])
      .then(([graph, modelList, appStatus]) => {
        if (!live) return;
        setWorkspace(graph.workspace);
        setModels(modelList.filter((model) => model.enabled));
        setStatus(appStatus);
        edgeRef.current = graph.edges;
        const restored = graph.nodes.map((node) => {
          const draft = drafts[String(node.id)];
          if (!draft) return node;
          const normalizedDraft =
            node.nodeType === "text" &&
            !Object.prototype.hasOwnProperty.call(draft, "content")
              ? { ...draft, content: draft.prompt ?? node.content, prompt: node.prompt }
              : draft;
          const result = {
            ...node,
            ...normalizedDraft,
            params: JSON.stringify(normalizedDraft.params || {}),
          };
          queue.change(node.id, nodePayload(result));
          return result;
        });
        rawRef.current = new Map(
          restored.map((node) => [String(node.id), node]),
        );
        setNodes(restored.map(toFlowNode));
        setEdges(
          graph.edges.map((edge) => ({
            id: String(edge.id),
            source: String(edge.fromNodeId),
            target: String(edge.toNodeId),
            markerEnd: { type: "arrowclosed" },
          })),
        );
        setLoaded(true);
      })
      .catch((error) => {
        if (live) showMessage(error.message);
      });
    return () => {
      live = false;
    };
  }, [id, queue, drafts, setNodes, setEdges, toFlowNode]);

  useEffect(() => {
    if (!loaded) return;
    const stream = new EventSource(`/api/v1/workspaces/${id}/events`);
    stream.addEventListener("snapshot", (event) => {
      try {
        const snapshot = JSON.parse(event.data);
        setConnection("connected");
        setTasks(snapshot.tasks);
        setConcurrency(snapshot.concurrency);
        setStatus((current) => ({ ...current, ready: snapshot.configured }));
        const updates = new Map(
          snapshot.nodes.map((node) => [String(node.id), node]),
        );
        for (const [key, update] of updates) {
          const raw = rawRef.current.get(key);
          if (raw) rawRef.current.set(key, mergeTaskState(raw, update));
        }
        setNodes((list) =>
          list.map((node) =>
            updates.has(node.id)
              ? {
                  ...node,
                  data: { ...node.data, node: rawRef.current.get(node.id) },
                }
              : node,
          ),
        );
      } catch {
        setConnection("reconnecting");
      }
    });
    stream.onerror = () => setConnection("reconnecting");
    return () => stream.close();
  }, [id, loaded, setNodes]);
  useEffect(() => {
    const down = (event) => {
      if (
        event.code === "Space" &&
        !event.target.closest('input,textarea,select,[contenteditable="true"]')
      ) {
        event.preventDefault();
        setSpacePressed(true);
      }
    };
    const up = (event) => {
      if (event.code === "Space") setSpacePressed(false);
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
    };
  }, []);

  const patchLocal = useCallback(
    (nodeId, patch) => {
      const key = String(nodeId),
        raw = rawRef.current.get(key);
      if (raw) {
        const next = { ...raw, ...patch };
        rawRef.current.set(key, next);
        queue.change(key, nodePayload(next));
      }
      setNodes((list) =>
        list.map((item) =>
          item.id === key
            ? {
                ...item,
                data: { ...item.data, node: { ...item.data.node, ...patch } },
              }
            : item,
        ),
      );
    },
    [setNodes, queue],
  );
  const save = useCallback(
    (nodeId) => queue.flush(nodeId).catch((error) => showMessage(error.message)),
    [queue],
  );
  const changeAndSave = useCallback(
    (nodeId, patch) => {
      patchLocal(nodeId, patch);
      queueMicrotask(() => save(nodeId));
    },
    [patchLocal, save],
  );
  const setNodeEditing = useCallback(
    (nodeId, editing) => {
      setNodes((list) =>
        list.map((item) =>
          item.id === String(nodeId)
            ? { ...item, draggable: !editing, data: { ...item.data, editing } }
            : item,
        ),
      );
    },
    [setNodes],
  );
  const changeParam = useCallback(
    (nodeId, key, value) => {
      const raw = rawRef.current.get(String(nodeId));
      let params = {};
      try {
        params = JSON.parse(raw?.params || "{}") || {};
      } catch {}
      params[key] = value;
      changeAndSave(nodeId, { params: JSON.stringify(params) });
    },
    [changeAndSave],
  );

  const addLocalNode = useCallback(
    (created) => {
      const node = {
        ...created,
        asset:
          created.asset ||
          (created.currentAssetId
            ? {
                id: created.currentAssetId,
                url: `/api/v1/assets/${created.currentAssetId}/content`,
              }
            : null),
      };
      rawRef.current.set(String(node.id), node);
      setNodes((list) => [
        ...list.map((item) => ({ ...item, selected: false })),
        { ...toFlowNode(node), selected: true },
      ]);
      setSelectedId(String(node.id));
      return node;
    },
    [setNodes, toFlowNode],
  );
  const createNode = useCallback(
    async (type, point, sourceId = null, assetId = null, title = "") => {
      const model = models.find((item) => item.taskType === type);
      const created = await api.createNode(id, {
        nodeType: type,
        title: title || t("newNode", { type: labels[type] }),
        modelKey: model?.key || "",
        params: model?.defaults || {},
        posX: point.x,
        posY: point.y,
        width: type === "text" ? 320 : 280,
        height: 220,
        assetId,
      });
      addLocalNode(created);
      if (sourceId) {
        const edge = await api.createEdge(id, {
          fromNodeId: Number(sourceId),
          toNodeId: created.id,
        });
        edgeRef.current = [...edgeRef.current, edge];
        setEdges((list) => [
          ...list,
          {
            id: String(edge.id),
            source: String(edge.fromNodeId),
            target: String(edge.toNodeId),
            markerEnd: { type: "arrowclosed" },
          },
        ]);
      }
      return created;
    },
    [id, models, addLocalNode, setEdges, t],
  );
  const duplicate = useCallback(
    async (nodeId) => {
      try {
        await queue.flush(nodeId);
        addLocalNode(await api.duplicateNode(nodeId));
      } catch (error) {
        showMessage(error.message);
      }
    },
    [queue, addLocalNode],
  );
  const openHistory = useCallback(
    (nodeId) => setPanel({ type: "history", nodeId: String(nodeId) }),
    [],
  );
  const remove = useCallback(
    async (nodeId) => {
      try {
        await queue.flush(nodeId);
        await api.deleteNode(nodeId);
        queue.remove(nodeId);
        setNodes((list) => list.filter((node) => node.id !== String(nodeId)));
        setEdges((list) =>
          list.filter(
            (edge) =>
              edge.source !== String(nodeId) && edge.target !== String(nodeId),
          ),
        );
        edgeRef.current = edgeRef.current.filter(
          (edge) =>
            String(edge.fromNodeId) !== String(nodeId) &&
            String(edge.toNodeId) !== String(nodeId),
        );
        rawRef.current.delete(String(nodeId));
        if (selectedId === String(nodeId)) setSelectedId(null);
      } catch (error) {
        showMessage(error.message);
      }
    },
    [queue, selectedId, setEdges, setNodes],
  );
  const run = useCallback(
    async (nodeId) => {
      const key = String(nodeId);
      setSubmitting((current) => new Set(current).add(key));
      try {
        await queue.flushAll();
        const raw = rawRef.current.get(key);
        if (!raw) return;
        const params = {
          ...(models.find((model) => model.key === raw.modelKey)?.defaults ||
            {}),
          ...JSON.parse(raw.params || "{}"),
        };
        const inputNodeIds = edgeRef.current
          .filter((edge) => String(edge.toNodeId) === key)
          .map((edge) => edge.fromNodeId);
        const session = await api.runNode(nodeId, {
          prompt: raw.prompt,
          modelKey: raw.modelKey,
          params,
          inputNodeIds,
        });
        // SSE may already have advanced this task. Never roll its state backwards.
        const latest = rawRef.current.get(key);
        if (
          latest &&
          (!latest.generation || latest.generation.id < session.id)
        ) {
          rawRef.current.set(key, { ...latest, generation: session });
          setNodes((list) =>
            list.map((node) =>
              node.id === key
                ? {
                    ...node,
                    data: { ...node.data, node: rawRef.current.get(key) },
                  }
                : node,
            ),
          );
        }
      } catch (error) {
        showMessage(error.message);
      } finally {
        setSubmitting((current) => {
          const next = new Set(current);
          next.delete(key);
          return next;
        });
      }
    },
    [queue, models, setNodes],
  );
  const leave = async (destination) => {
    try {
      await queue.flushAll();
      navigate(destination);
    } catch {
      showMessage(t("unsavedLeave"));
    }
  };
  const resize = useCallback(
    (event, nodeId) => {
      event.preventDefault();
      event.stopPropagation();
      const flowNode = nodes.find((node) => node.id === String(nodeId));
      if (!flowNode) return;
      const start = {
          x: event.clientX,
          y: event.clientY,
          width: Number(flowNode.style?.width || 280),
          height: Number(flowNode.style?.height || 220),
        },
        zoom = flowRef.current?.getViewport()?.zoom || 1;
      const move = (moveEvent) => {
        const width = Math.max(
            220,
            start.width + (moveEvent.clientX - start.x) / zoom,
          ),
          height = Math.max(
            160,
            start.height + (moveEvent.clientY - start.y) / zoom,
          );
        setNodes((list) =>
          list.map((node) =>
            node.id === String(nodeId)
              ? { ...node, style: { ...node.style, width, height } }
              : node,
          ),
        );
        const raw = rawRef.current.get(String(nodeId));
        if (raw) {
          raw.width = width;
          raw.height = height;
        }
      };
      const up = () => {
        window.removeEventListener("pointermove", move);
        window.removeEventListener("pointerup", up);
        const raw = rawRef.current.get(String(nodeId));
        if (raw) queue.change(nodeId, nodePayload(raw));
        save(nodeId);
      };
      window.addEventListener("pointermove", move);
      window.addEventListener("pointerup", up);
    },
    [nodes, queue, save, setNodes],
  );

  const displayedNodes = useMemo(
    () =>
      nodes.map((item) => ({
        ...item,
        draggable: !item.data?.editing,
        selected: Boolean(item.selected || item.id === selectedId),
        data: {
          ...item.data,
          node: rawRef.current.get(item.id) || item.data.node,
          change: patchLocal,
          save,
          resize,
          editing: setNodeEditing,
          copy: duplicate,
          history: openHistory,
          saveState: saveStates[item.id]?.status || "saved",
        },
      })),
    [
      nodes,
      patchLocal,
      resize,
      save,
      selectedId,
      setNodeEditing,
      duplicate,
      openHistory,
      saveStates,
    ],
  );
  const selectedNode = selectedId ? rawRef.current.get(selectedId) : null;
  const selectedModels = models.filter(
      (model) => model.taskType === selectedNode?.nodeType,
    ),
    selectedModel =
      selectedModels.find((model) => model.key === selectedNode?.modelKey) ||
      selectedModels[0];
  let selectedParams = { ...(selectedModel?.defaults || {}) };
  try {
    selectedParams = {
      ...selectedParams,
      ...(JSON.parse(selectedNode?.params || "{}") || {}),
    };
  } catch {}

  const connect = async (connection) => {
    try {
      const edge = await api.createEdge(id, {
        fromNodeId: Number(connection.source),
        toNodeId: Number(connection.target),
      });
      edgeRef.current = [...edgeRef.current, edge];
      setEdges((list) =>
        addEdge(
          {
            ...connection,
            id: String(edge.id),
            markerEnd: { type: "arrowclosed" },
          },
          list,
        ),
      );
    } catch (error) {
      showMessage(error.message);
    }
  };
  const deleteEdges = async (removed) => {
    for (const edge of removed) await api.deleteEdge(edge.id).catch(() => {});
    edgeRef.current = edgeRef.current.filter(
      (edge) => !removed.some((item) => String(item.id) === String(edge.id)),
    );
  };
  const dragStop = (_, flowNode, dragged) => {
    for (const node of dragged?.length ? dragged : [flowNode]) {
      patchLocal(node.id, { posX: node.position.x, posY: node.position.y });
      save(node.id);
    }
  };
  const openPaneMenu = (event) => {
    event.preventDefault();
    if (rightPointerRef.current.moved) return;
    const point = flowRef.current?.screenToFlowPosition({
      x: event.clientX,
      y: event.clientY,
    }) || { x: 0, y: 0 };
    setContextMenu({
      visible: true,
      mode: "root",
      x: event.clientX,
      y: event.clientY,
      flowX: point.x,
      flowY: point.y,
      sourceId: null,
    });
    setNodeMenu((menu) => ({ ...menu, visible: false }));
  };
  const openNodeMenu = (event, node) => {
    event.preventDefault();
    setSelectedId(node.id);
    setNodeMenu({
      visible: true,
      mode: "root",
      x: event.clientX,
      y: event.clientY,
      nodeId: node.id,
      name: rawRef.current.get(node.id)?.title || "",
    });
    setContextMenu((menu) => ({ ...menu, visible: false }));
  };
  const selectContextType = async (type) => {
    try {
      await createNode(
        type,
        { x: contextMenu.flowX, y: contextMenu.flowY },
        contextMenu.sourceId,
      );
    } catch (error) {
      showMessage(error.message);
    }
    setContextMenu((menu) => ({ ...menu, visible: false, mode: "root" }));
    connectRef.current = null;
  };
  const upload = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    try {
      showMessage(t("uploading"));
      const asset = await api.upload(id, file),
        type = file.type.startsWith("image/")
          ? "image"
          : file.type.startsWith("audio/")
            ? "audio"
            : "video",
        point = Number.isFinite(contextMenu.flowX)
          ? { x: contextMenu.flowX, y: contextMenu.flowY }
          : { x: 160, y: 140 };
      await createNode(type, point, null, asset.id, file.name);
      showMessage("");
    } catch (error) {
      showMessage(error.message);
    }
  };
  const wheel = (event) => {
    if (
      !(event.metaKey || event.ctrlKey) ||
      !flowRef.current ||
      !wrapperRef.current
    )
      return;
    event.preventDefault();
    const current = flowRef.current.getViewport(),
      rect = wrapperRef.current.getBoundingClientRect(),
      cursorX = event.clientX - rect.left,
      cursorY = event.clientY - rect.top,
      worldX = (cursorX - current.x) / current.zoom,
      worldY = (cursorY - current.y) / current.zoom,
      zoom = clamp(current.zoom - event.deltaY * 0.002, 0.25, 3);
    flowRef.current.setViewport(
      { x: cursorX - worldX * zoom, y: cursorY - worldY * zoom, zoom },
      { duration: 0 },
    );
  };

  return (
    <div
      className={`canvas-page workshop-canvas ${spacePressed ? "space-pan" : ""}`}
    >
      <header className="canvas-bar">
        <button className="back" onClick={() => leave("/")}>
          ←
        </button>
        <Brand app={status?.app} compact />
        <div className="project-name">{workspace?.name || t("loading")}</div>
        <button
          className={`autosave-status ${saveStatus}`}
          onClick={() =>
            queue.flushAll().catch((error) => showMessage(error.message))
          }
          title={t("retrySaveTitle")}
          aria-live="polite"
        >
          {
            {
              saved: t("savedStatus"),
              pending: t("waitingSave"),
              saving: t("savingStatus"),
              error: t("retrySaveStatus"),
            }[saveStatus]
          }
        </button>
        <span
          className={`sse-indicator ${connection}`}
          title={
            connection === "connected"
              ? t("realtimeTitle")
              : t("disconnectedTitle")
          }
        >
          {connection === "connected" ? t("realtime") : t("reconnecting")}
        </span>
        <div className="canvas-actions">
          <button onClick={() => setPanel({ type: "assets" })}>{t("assetLibrary")}</button>
          <button onClick={() => setPanel({ type: "queue" })}>
            {t("taskQueue")}{" "}
            {tasks.filter((task) => isRunning(task.status)).length || ""}
          </button>
          <button onClick={() => leave("/settings")}>{t("settings")}</button>
          <button onClick={() => leave("/settings/models")}>{t("models")}</button>
          <button onClick={() => uploadRef.current?.click()}>{t("uploadAsset")}</button>
          <input
            ref={uploadRef}
            type="file"
            accept="image/*,audio/*,video/*"
            hidden
            onChange={upload}
          />
        </div>
        <LanguageToggle />
        <ThemeToggle />
        <div className={`ready-dot ${status?.ready ? "ok" : ""}`} />
      </header>
      {message && (
        <div className={`canvas-message ${messageKind}`} onClick={() => showMessage("")}>
          {message}
        </div>
      )}
      <div
        ref={wrapperRef}
        className="flow-stage"
        onWheelCapture={wheel}
        onPointerDownCapture={(event) => {
          if (event.button === 2)
            rightPointerRef.current = {
              down: true,
              moved: false,
              x: event.clientX,
              y: event.clientY,
            };
        }}
        onPointerMoveCapture={(event) => {
          const state = rightPointerRef.current;
          if (
            state.down &&
            Math.hypot(event.clientX - state.x, event.clientY - state.y) > 4
          )
            state.moved = true;
        }}
        onPointerUpCapture={() => {
          rightPointerRef.current.down = false;
        }}
      >
        <ReactFlow
          nodes={displayedNodes}
          edges={edges}
          nodeTypes={nodeTypes}
          onInit={(instance) => {
            flowRef.current = instance;
          }}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={connect}
          onNodeDragStop={dragStop}
          onNodesDelete={(removed) =>
            removed.forEach((node) => remove(node.id))
          }
          onEdgesDelete={deleteEdges}
          onSelectionChange={({ nodes: selected }) =>
            setSelectedId(selected.length === 1 ? selected[0].id : null)
          }
          onNodeClick={(_, node) => {
            setSelectedId(node.id);
            setContextMenu((menu) => ({ ...menu, visible: false }));
            setNodeMenu((menu) => ({ ...menu, visible: false }));
          }}
          onPaneClick={() => {
            setSelectedId(null);
            setContextMenu((menu) => ({ ...menu, visible: false }));
            setNodeMenu((menu) => ({ ...menu, visible: false }));
          }}
          onPaneContextMenu={openPaneMenu}
          onNodeContextMenu={openNodeMenu}
          onConnectStart={(_, params) => {
            connectRef.current =
              params?.handleType === "source"
                ? { sourceId: String(params.nodeId) }
                : null;
          }}
          onConnectEnd={(event, state) => {
            if (!connectRef.current || state?.isValid) {
              connectRef.current = null;
              return;
            }
            const point = flowRef.current?.screenToFlowPosition({
              x: event.clientX,
              y: event.clientY,
            });
            if (!point) return;
            setContextMenu({
              visible: true,
              mode: "types",
              x: event.clientX,
              y: event.clientY,
              flowX: point.x,
              flowY: point.y,
              sourceId: connectRef.current.sourceId,
            });
          }}
          panOnScroll
          panOnScrollMode="free"
          panOnDrag={spacePressed ? [0, 2] : [2]}
          nodesDraggable={!spacePressed}
          selectionOnDrag
          selectionMode={SelectionMode.Partial}
          zoomOnScroll={false}
          zoomOnDoubleClick={false}
          zoomOnPinch
          minZoom={0.25}
          maxZoom={3}
          onMove={(_, next) => setViewportState(next)}
          fitView
          fitViewOptions={{ maxZoom: 1 }}
          proOptions={{ hideAttribution: true }}
          deleteKeyCode={["Backspace", "Delete"]}
        >
          <Background
            variant="dots"
            gap={20}
            size={1}
            color={
              theme === "dark" ? "rgba(255,255,255,.28)" : "rgba(22,25,31,.24)"
            }
          />
        </ReactFlow>
      </div>
      <div className="canvas-scale">
        <span>{Math.round(viewport.zoom * 100)}%</span>
        <input
          type="range"
          min="25"
          max="300"
          step="5"
          value={Math.round(viewport.zoom * 100)}
          onChange={(event) => {
            const zoom = Number(event.target.value) / 100;
            flowRef.current?.zoomTo(zoom);
            setViewportState((current) => ({ ...current, zoom }));
          }}
        />
      </div>
      {!panel && selectedNode && (
        <aside className="node-inspector">
          <div className="inspector-head">
            <div>
              <small>{t("nodeType", { type: labels[selectedNode.nodeType] })}</small>
              <input
                value={selectedNode.title || ""}
                onChange={(event) =>
                  patchLocal(selectedId, { title: event.target.value })
                }
                onBlur={() => save(selectedId)}
              />
            </div>
            <button onClick={() => setSelectedId(null)}>×</button>
          </div>
          <div className="inspector-utilities">
            <button onClick={() => duplicate(selectedId)}>{t("copyNode")}</button>
            <button onClick={() => openHistory(selectedId)}>{t("generationHistory")}</button>
            {selectedNode.currentAssetId && selectedNode.asset?.storageProvider === "local" && (
              <a href={api.downloadURL(selectedNode.currentAssetId)} download>
                ↓ {t("downloadResult")}
              </a>
            )}
            {selectedNode.asset?.storageProvider === "volcengine" && <span className="trusted-asset-label">{t("trustedAsset")}</span>}
          </div>
          {selectedNode.nodeType === "audio" && !AUDIO_GENERATION_ENABLED ? (
            <p className="inspector-notice">{t("audioUploadOnly")}</p>
          ) : (
            <>
          <label>
            {t("model")}
            <select
              value={selectedNode.modelKey || selectedModel?.key || ""}
              onChange={(event) =>
                changeAndSave(selectedId, { modelKey: event.target.value })
              }
            >
              {selectedModels.map((model) => (
                <option key={model.key} value={model.key}>
                  {model.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            {t("prompt")}
            <textarea
              value={selectedNode.prompt || ""}
              onChange={(event) =>
                patchLocal(selectedId, { prompt: event.target.value })
              }
              onBlur={() => save(selectedId)}
              placeholder={t("promptPlaceholder")}
            />
          </label>
          {selectedModel?.options?.ratio?.length > 0 && (
            <label>
              {t("ratio")}
              <select
                value={selectedParams.ratio || ""}
                onChange={(event) =>
                  changeParam(selectedId, "ratio", event.target.value)
                }
              >
                {selectedModel.options.ratio.map((value) => (
                  <option key={value}>{value}</option>
                ))}
              </select>
            </label>
          )}
          {selectedModel?.options?.resolution?.length > 0 && (
            <label>
              {t("resolution")}
              <select
                value={selectedParams.resolution || ""}
                onChange={(event) =>
                  changeParam(selectedId, "resolution", event.target.value)
                }
              >
                {selectedModel.options.resolution.map((value) => (
                  <option key={value}>{value}</option>
                ))}
              </select>
            </label>
          )}
          {selectedModel?.options?.duration?.length > 0 && (
            <label>
              {t("duration")}
              <select
                value={selectedParams.duration || ""}
                onChange={(event) =>
                  changeParam(
                    selectedId,
                    "duration",
                    Number(event.target.value),
                  )
                }
              >
                {selectedModel.options.duration.map((value) => (
                  <option key={value} value={value}>
                    {t("seconds", { value })}
                  </option>
                ))}
              </select>
            </label>
          )}
          {selectedModel?.options?.temperature?.length > 0 && (
            <label>
              {t("temperature")}
              <select
                value={selectedParams.temperature ?? ""}
                onChange={(event) =>
                  changeParam(selectedId, "temperature", Number(event.target.value))
                }
              >
                {selectedModel.options.temperature.map((value) => (
                  <option key={value} value={value}>{value}</option>
                ))}
              </select>
            </label>
          )}
          {selectedModel?.options?.maxTokens?.length > 0 && (
            <label>
              {t("maxTokens")}
              <select
                value={selectedParams.maxTokens ?? ""}
                onChange={(event) =>
                  changeParam(selectedId, "maxTokens", Number(event.target.value))
                }
              >
                {selectedModel.options.maxTokens.map((value) => (
                  <option key={value} value={value}>{value}</option>
                ))}
              </select>
            </label>
          )}
          {selectedModel?.options?.thinking?.length > 0 && (
            <label>
              {t("thinkingMode")}
              <select
                value={selectedParams.thinking || "disabled"}
                onChange={(event) =>
                  changeParam(selectedId, "thinking", event.target.value)
                }
              >
                {selectedModel.options.thinking.map((value) => (
                  <option key={value} value={value}>
                    {t(
                      {
                        disabled: "thinkingDisabled",
                        auto: "thinkingAuto",
                        enabled: "thinkingEnabled",
                      }[value] || value,
                    )}
                  </option>
                ))}
              </select>
            </label>
          )}
          {["responseFormat", "sampleRate", "speechRate", "loudnessRate", "pitchRate"].map(
            (parameter) =>
              selectedModel?.options?.[parameter]?.length > 0 && (
                <label key={parameter}>
                  {t(parameter)}
                  <select
                    value={selectedParams[parameter] ?? ""}
                    onChange={(event) =>
                      changeParam(
                        selectedId,
                        parameter,
                        parameter === "responseFormat"
                          ? event.target.value
                          : Number(event.target.value),
                      )
                    }
                  >
                    {selectedModel.options[parameter].map((value) => (
                      <option key={value} value={value}>{value}</option>
                    ))}
                  </select>
                </label>
              ),
          )}
          <button
            className="inspector-run"
            disabled={
              !status?.ready ||
              submitting.has(selectedId) ||
              isRunning(selectedNode.generation?.status) ||
              !selectedModel
            }
            onClick={() => run(selectedId)}
          >
            {submitting.has(selectedId)
              ? t("submitting")
              : isRunning(selectedNode.generation?.status)
                ? t("generating")
                : t("startGeneration")}
          </button>
          {selectedNode.generation?.status === "failed" && (
            <p className="inspector-error">
              {selectedNode.generation.errorMessage}
            </p>
          )}
            </>
          )}
        </aside>
      )}
      {panel?.type === "assets" ? (
        <AssetLibraryPanel onClose={() => setPanel(null)} onAdd={async (item) => {
          const bounds = wrapperRef.current?.getBoundingClientRect();
          const point = flowRef.current?.screenToFlowPosition({ x: (bounds?.left || 0) + (bounds?.width || 800) / 2, y: (bounds?.top || 0) + (bounds?.height || 600) / 2 }) || { x: 100, y: 100 };
          const created = await api.importLibraryAsset(id, { ...item, posX: point.x, posY: point.y });
          addLocalNode(created); setPanel(null); showMessage(t("assetAddedToCanvas"), "success");
        }} />
      ) : panel && (
        <TaskPanel
          tasks={tasks}
          connection={connection}
          concurrency={concurrency}
          historyNode={
            panel.type === "history" ? rawRef.current.get(panel.nodeId) : null
          }
          onClose={() => setPanel(null)}
          onLocate={(nodeId) => {
            const key = String(nodeId);
            if (!rawRef.current.has(key)) {
              showMessage(t("deletedNodeHistory"));
              return;
            }
            setSelectedId(key);
            setNodes((list) =>
              list.map((node) => ({ ...node, selected: node.id === key })),
            );
            flowRef.current?.fitView({
              nodes: [{ id: key }],
              maxZoom: 1,
              padding: 0.5,
              duration: 200,
            });
          }}
        />
      )}
      {contextMenu.visible && (
        <div
          className="context-menu"
          style={menuPosition(contextMenu.x, contextMenu.y)}
          onPointerDown={(event) => event.stopPropagation()}
        >
          {contextMenu.mode === "root" ? (
            <>
              <button onClick={() => uploadRef.current?.click()}>
                {t("uploadAsset")}
              </button>
              <hr />
              <button
                className="primary"
                onClick={() =>
                  setContextMenu((menu) => ({ ...menu, mode: "types" }))
                }
              >
                {t("addNode")} <span>›</span>
              </button>
            </>
          ) : (
            <>
              <small>{t("selectNodeType")}</small>
              {[
                "video",
                "image",
                ...(AUDIO_GENERATION_ENABLED ? ["audio"] : []),
                "text",
              ].map((type) => (
                <button key={type} onClick={() => selectContextType(type)}>
                  {labels[type]}
                </button>
              ))}
              <hr />
              <button
                onClick={() =>
                  setContextMenu((menu) => ({ ...menu, mode: "root" }))
                }
              >
                {t("back")}
              </button>
            </>
          )}
        </div>
      )}
      {nodeMenu.visible && (
        <div
          className="context-menu"
          style={menuPosition(nodeMenu.x, nodeMenu.y)}
          onPointerDown={(event) => event.stopPropagation()}
        >
          {nodeMenu.mode === "root" ? (
            <>
              <button
                onClick={() =>
                  setNodeMenu((menu) => ({ ...menu, mode: "rename" }))
                }
              >
                {t("rename")}
              </button>
              <button
                onClick={() => {
                  duplicate(nodeMenu.nodeId);
                  setNodeMenu((menu) => ({ ...menu, visible: false }));
                }}
              >
                {t("copyNode")}
              </button>
              <button
                onClick={() => {
                  openHistory(nodeMenu.nodeId);
                  setNodeMenu((menu) => ({ ...menu, visible: false }));
                }}
              >
                {t("generationHistory")}
              </button>
              <button
                onClick={() => {
                  setSelectedId(nodeMenu.nodeId);
                  setNodeMenu((menu) => ({ ...menu, visible: false }));
                }}
              >
                {t("editParameters")}
              </button>
              <hr />
              <button
                className="danger"
                onClick={() => {
                  if (confirm(t("confirmDeleteNode"))) remove(nodeMenu.nodeId);
                  setNodeMenu((menu) => ({ ...menu, visible: false }));
                }}
              >
                {t("deleteNode")}
              </button>
            </>
          ) : (
            <form
              onSubmit={(event) => {
                event.preventDefault();
                changeAndSave(nodeMenu.nodeId, {
                  title: nodeMenu.name.trim() || t("untitledNode"),
                });
                setNodeMenu((menu) => ({ ...menu, visible: false }));
              }}
            >
              <input
                autoFocus
                value={nodeMenu.name}
                onChange={(event) =>
                  setNodeMenu((menu) => ({ ...menu, name: event.target.value }))
                }
              />
              <button className="primary">{t("confirm")}</button>
            </form>
          )}
        </div>
      )}
    </div>
  );
}

function menuPosition(x, y) {
  return {
    left: Math.min(x, window.innerWidth - 240),
    top: Math.min(y, window.innerHeight - 260),
  };
}
