export async function request(path, options = {}) {
  const headers = {
    ...(options.body instanceof FormData
      ? {}
      : { "Content-Type": "application/json" }),
    ...(options.headers || {}),
  };
  const response = await fetch(`/api/v1${path}`, { ...options, headers });
  if (response.status === 204) return null;
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const english = localStorage.getItem("goose-canvas-language") === "en";
    throw new Error(
      data.error ||
        (english
          ? `Request failed (${response.status})`
          : `请求失败（${response.status}）`),
    );
  }
  return data;
}

export const api = {
  settings: () => request("/settings"),
  saveSettings: (body) =>
    request("/settings", { method: "PUT", body: JSON.stringify(body) }),
  testConnection: (body) =>
    request("/settings/test-connection", { method: "POST", body: JSON.stringify(body) }),
  assetLibrary: (page = 1, groupId = "") => request(`/asset-library?page=${page}&pageSize=100${groupId ? `&groupId=${encodeURIComponent(groupId)}` : ""}`),
  importLibraryAsset: (workspaceId, body) => request(`/workspaces/${workspaceId}/asset-library/nodes`, { method: "POST", body: JSON.stringify(body) }),
  history: (id, before) =>
    request(`/nodes/${id}/history${before ? `?before=${before}` : ""}`),
  duplicateNode: (id) => request(`/nodes/${id}/duplicate`, { method: "POST" }),
  downloadURL: (id) => `/api/v1/assets/${id}/download`,
  status: () => request("/system/status"),
  models: () => request("/models"),
  createModel: (body) =>
    request("/models", { method: "POST", body: JSON.stringify(body) }),
  updateModel: (id, body) =>
    request(`/models/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteModel: (id) => request(`/models/${id}`, { method: "DELETE" }),
  workspaces: () => request("/workspaces"),
  createWorkspace: (body) =>
    request("/workspaces", { method: "POST", body: JSON.stringify(body) }),
  updateWorkspace: (id, body) =>
    request(`/workspaces/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteWorkspace: (id) => request(`/workspaces/${id}`, { method: "DELETE" }),
  graph: (id) => request(`/workspaces/${id}/graph`),
  createNode: (workspaceId, body) =>
    request(`/workspaces/${workspaceId}/nodes`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  updateNode: (id, body) =>
    request(`/nodes/${id}`, {
      method: "PUT",
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(15000),
    }),
  deleteNode: (id) => request(`/nodes/${id}`, { method: "DELETE" }),
  createEdge: (workspaceId, body) =>
    request(`/workspaces/${workspaceId}/edges`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteEdge: (id) => request(`/edges/${id}`, { method: "DELETE" }),
  upload: (workspaceId, file) => {
    const body = new FormData();
    body.append("workspaceId", workspaceId);
    body.append("file", file);
    return request("/assets/upload", { method: "POST", body });
  },
  runNode: (id, body) =>
    request(`/nodes/${id}/run`, { method: "POST", body: JSON.stringify(body) }),
  generation: (id) => request(`/generations/${id}`),
  retry: (id) => request(`/generations/${id}/retry`, { method: "POST" }),
};
