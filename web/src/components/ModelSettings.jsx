import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import Brand from "./Brand";
import ThemeToggle from "./ThemeToggle";

const emptyForm = {
  key: "",
  name: "",
  taskType: "image",
  provider: "volcengine",
  protocol: "ark-image-v3",
  description: "",
  inputs: '["image"]',
  defaults: '{"ratio":"1:1","resolution":"2K"}',
  options: '{"ratio":["1:1","16:9","9:16"],"resolution":["1K","2K"]}',
  enabled: true,
};

export default function ModelSettings() {
  const navigate = useNavigate();
  const [models, setModels] = useState([]),
    [status, setStatus] = useState(null),
    [editingId, setEditingId] = useState(null);
  const [form, setForm] = useState(emptyForm),
    [error, setError] = useState(""),
    [saved, setSaved] = useState("");
  const load = async () => {
    try {
      const [items, appStatus] = await Promise.all([
        api.models(),
        api.status(),
      ]);
      setModels(items);
      setStatus(appStatus);
    } catch (e) {
      setError(e.message);
    }
  };
  useEffect(() => {
    load();
  }, []);
  const change = (key, value) =>
    setForm((current) => ({ ...current, [key]: value }));
  const changeType = (type) =>
    setForm((current) => ({
      ...current,
      taskType: type,
      protocol: type === "image" ? "ark-image-v3" : "ark-video-v3",
      defaults:
        type === "image"
          ? '{"ratio":"1:1","resolution":"2K"}'
          : '{"ratio":"16:9","resolution":"720p","duration":5}',
      options:
        type === "image"
          ? '{"ratio":["1:1","16:9","9:16"],"resolution":["1K","2K"]}'
          : '{"ratio":["16:9","9:16","1:1"],"resolution":["720p","1080p"],"duration":[5,10]}',
    }));
  const edit = (model) => {
    setEditingId(model.id);
    setForm({
      key: model.key,
      name: model.name,
      taskType: model.taskType,
      provider: model.provider,
      protocol: model.protocol,
      description: model.description || "",
      inputs: JSON.stringify(model.inputs || []),
      defaults: JSON.stringify(model.defaults || {}, null, 2),
      options: JSON.stringify(model.options || {}, null, 2),
      enabled: model.enabled,
    });
    setError("");
    setSaved("");
  };
  const reset = () => {
    setEditingId(null);
    setForm(emptyForm);
    setError("");
    setSaved("");
  };
  const submit = async (event) => {
    event.preventDefault();
    setError("");
    setSaved("");
    try {
      const payload = {
        ...form,
        inputs: JSON.parse(form.inputs),
        defaults: JSON.parse(form.defaults),
        options: JSON.parse(form.options),
      };
      if (editingId) await api.updateModel(editingId, payload);
      else await api.createModel(payload);
      setSaved(editingId ? "模型已更新" : "模型已添加");
      await load();
      if (!editingId) setForm(emptyForm);
    } catch (e) {
      setError(
        e instanceof SyntaxError
          ? "Inputs、默认参数或可选参数不是有效 JSON"
          : e.message,
      );
    }
  };
  const toggle = async (model) => {
    try {
      await api.updateModel(model.id, { ...model, enabled: !model.enabled });
      await load();
    } catch (e) {
      setError(e.message);
    }
  };
  const remove = async (model) => {
    if (!confirm(`删除模型“${model.name}”？`)) return;
    try {
      await api.deleteModel(model.id);
      if (editingId === model.id) reset();
      await load();
    } catch (e) {
      setError(e.message);
    }
  };

  return (
    <main className="models-page">
      <header className="topbar">
        <Brand app={status?.app} />
        <div className="settings-actions">
          <button onClick={() => navigate("/settings")}>创作设置</button>
          <ThemeToggle />
          <button className="settings-back" onClick={() => navigate(-1)}>
            返回
          </button>
        </div>
      </header>
      <div className="models-layout">
        <section className="model-list">
          <div className="model-title">
            <div>
              <span>MODEL REGISTRY</span>
              <h1>模型管理</h1>
              <p>
                模型 Key 与能力参数保存在
                SQLite；凭证可在创作设置中配置，由本机后端保存。
              </p>
            </div>
            <button onClick={reset}>＋ 添加模型</button>
          </div>
          {models.map((model) => (
            <article
              className={`model-row ${model.enabled ? "" : "disabled"}`}
              key={model.id}
            >
              <div className={`model-kind ${model.taskType}`}>
                {model.taskType === "image" ? "图" : "视"}
              </div>
              <div className="model-main">
                <div>
                  <h3>{model.name}</h3>
                  {model.builtin && <em>内置</em>}
                  <span>{model.enabled ? "已启用" : "已停用"}</span>
                </div>
                <code>{model.key}</code>
                <p>
                  {model.provider} · {model.protocol} ·{" "}
                  {model.description || "暂无描述"}
                </p>
              </div>
              <div className="model-actions">
                <button onClick={() => edit(model)}>编辑</button>
                <button onClick={() => toggle(model)}>
                  {model.enabled ? "停用" : "启用"}
                </button>
                {!model.builtin && (
                  <button className="danger" onClick={() => remove(model)}>
                    删除
                  </button>
                )}
              </div>
            </article>
          ))}
        </section>
        <form className="model-form" onSubmit={submit}>
          <div>
            <small>{editingId ? "EDIT MODEL" : "NEW MODEL"}</small>
            <h2>{editingId ? "编辑模型" : "接入模型"}</h2>
          </div>
          <label>
            显示名称
            <input
              required
              value={form.name}
              onChange={(e) => change("name", e.target.value)}
              placeholder="例如 Seedream 5.0"
            />
          </label>
          <label>
            模型 Key
            <input
              required
              disabled={Boolean(editingId)}
              value={form.key}
              onChange={(e) => change("key", e.target.value)}
              placeholder="控制台中的模型 ID"
            />
          </label>
          <div className="form-grid">
            <label>
              任务类型
              <select
                value={form.taskType}
                onChange={(e) => changeType(e.target.value)}
              >
                <option value="image">图片</option>
                <option value="video">视频</option>
              </select>
            </label>
            <label>
              供应商
              <select
                value={form.provider}
                onChange={(e) => change("provider", e.target.value)}
              >
                <option value="volcengine">火山引擎</option>
              </select>
            </label>
          </div>
          <label>
            调用协议
            <select
              value={form.protocol}
              onChange={(e) => change("protocol", e.target.value)}
            >
              <option value="ark-image-v3">Ark 图片生成 v3</option>
              <option value="ark-video-v3">Ark 视频任务 v3</option>
            </select>
          </label>
          <label>
            说明
            <input
              value={form.description}
              onChange={(e) => change("description", e.target.value)}
              placeholder="模型用途"
            />
          </label>
          <label>
            允许的输入类型（JSON）
            <textarea
              value={form.inputs}
              onChange={(e) => change("inputs", e.target.value)}
            />
          </label>
          <label>
            默认参数（JSON）
            <textarea
              value={form.defaults}
              onChange={(e) => change("defaults", e.target.value)}
            />
          </label>
          <label>
            可选参数（JSON）
            <textarea
              value={form.options}
              onChange={(e) => change("options", e.target.value)}
            />
          </label>
          <label className="check">
            <input
              type="checkbox"
              checked={form.enabled}
              onChange={(e) => change("enabled", e.target.checked)}
            />
            启用此模型
          </label>
          {error && <p className="form-error">{error}</p>}
          {saved && <p className="form-saved">{saved}</p>}
          <div className="form-actions">
            <button type="button" onClick={reset}>
              重置
            </button>
            <button className="primary">
              {editingId ? "保存修改" : "添加模型"}
            </button>
          </div>
        </form>
      </div>
    </main>
  );
}
