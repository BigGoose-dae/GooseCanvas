import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import Brand from "./Brand";
import ThemeToggle from "./ThemeToggle";
import LanguageToggle from "./LanguageToggle";
import { useLanguage } from "../i18n/LanguageContext";

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
  const { t } = useLanguage();
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
      protocol: {
        text: "ark-chat-v3",
        image: "ark-image-v3",
        audio: "doubao-audio-v3",
        video: "ark-video-v3",
      }[type],
      inputs: type === "text" ? '["text"]' : type === "audio" ? '["audio"]' : '["image"]',
      defaults:
        type === "text"
          ? '{"temperature":0.7,"maxTokens":4096,"thinking":"disabled"}'
          : type === "image"
          ? '{"ratio":"1:1","resolution":"2K"}'
          : type === "audio"
          ? '{"responseFormat":"mp3","sampleRate":24000,"speechRate":0,"loudnessRate":0,"pitchRate":0}'
          : '{"ratio":"16:9","resolution":"720p","duration":5}',
      options:
        type === "text"
          ? '{"temperature":[0.2,0.7,1],"maxTokens":[1024,2048,4096,8192],"thinking":["disabled","auto","enabled"]}'
          : type === "image"
          ? '{"ratio":["1:1","16:9","9:16"],"resolution":["1K","2K"]}'
          : type === "audio"
          ? '{"responseFormat":["mp3","wav"],"sampleRate":[8000,16000,24000,32000,44100,48000],"speechRate":[-50,0,50,100],"loudnessRate":[-50,0,50,100],"pitchRate":[-12,0,12]}'
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
      setSaved(editingId ? t("modelUpdated") : t("modelAdded"));
      await load();
      if (!editingId) setForm(emptyForm);
    } catch (e) {
      setError(
        e instanceof SyntaxError
          ? t("invalidModelJson")
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
    if (!confirm(t("confirmDeleteModel", { name: model.name }))) return;
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
          <button onClick={() => navigate("/settings")}>{t("creativeSettings")}</button>
          <LanguageToggle />
          <ThemeToggle />
          <button className="settings-back" onClick={() => navigate(-1)}>
            {t("back")}
          </button>
        </div>
      </header>
      <div className="models-layout">
        <section className="model-list">
          <div className="model-title">
            <div>
              <span>{t("modelRegistry")}</span>
              <h1>{t("modelManagement")}</h1>
              <p>{t("modelSettingsDescription")}</p>
            </div>
            <button onClick={reset}>＋ {t("addModel")}</button>
          </div>
          {models.map((model) => (
            <article
              className={`model-row ${model.enabled ? "" : "disabled"}`}
              key={model.id}
            >
              <div className={`model-kind ${model.taskType}`}>
                {
                  {
                    text: t("textShort"),
                    image: t("imageShort"),
                    audio: t("audioShort"),
                    video: t("videoShort"),
                  }[model.taskType]
                }
              </div>
              <div className="model-main">
                <div>
                  <h3>{model.name}</h3>
                  {model.builtin && <em>{t("builtin")}</em>}
                  <span>{model.enabled ? t("enabled") : t("disabled")}</span>
                </div>
                <code>{model.key}</code>
                <p>
                  {model.provider} · {model.protocol} ·{" "}
                  {model.description || t("noDescription")}
                </p>
              </div>
              <div className="model-actions">
                <button onClick={() => edit(model)}>{t("edit")}</button>
                <button onClick={() => toggle(model)}>
                  {model.enabled ? t("disable") : t("enable")}
                </button>
                {!model.builtin && (
                  <button className="danger" onClick={() => remove(model)}>
                    {t("delete")}
                  </button>
                )}
              </div>
            </article>
          ))}
        </section>
        <form className="model-form" onSubmit={submit}>
          <div>
            <small>{editingId ? "EDIT MODEL" : "NEW MODEL"}</small>
            <h2>{editingId ? t("editModel") : t("connectModel")}</h2>
          </div>
          <label>
            {t("displayName")}
            <input
              required
              value={form.name}
              onChange={(e) => change("name", e.target.value)}
              placeholder={t("displayNamePlaceholder")}
            />
          </label>
          <label>
            {t("modelKey")}
            <input
              required
              disabled={Boolean(editingId)}
              value={form.key}
              onChange={(e) => change("key", e.target.value)}
              placeholder={t("modelKeyPlaceholder")}
            />
          </label>
          <div className="form-grid">
            <label>
              {t("taskType")}
              <select
                value={form.taskType}
                onChange={(e) => changeType(e.target.value)}
              >
                <option value="text">{t("text")}</option>
                <option value="image">{t("image")}</option>
                <option value="audio">{t("audio")}</option>
                <option value="video">{t("video")}</option>
              </select>
            </label>
            <label>
              {t("provider")}
              <select
                value={form.provider}
                onChange={(e) => change("provider", e.target.value)}
              >
                <option value="volcengine">{t("volcengine")}</option>
              </select>
            </label>
          </div>
          <label>
            {t("protocol")}
            <select
              value={form.protocol}
              onChange={(e) => change("protocol", e.target.value)}
            >
              <option value="ark-chat-v3">{t("arkChatProtocol")}</option>
              <option value="ark-image-v3">{t("arkImageProtocol")}</option>
              <option value="doubao-audio-v3">{t("doubaoAudioProtocol")}</option>
              <option value="ark-video-v3">{t("arkVideoProtocol")}</option>
            </select>
          </label>
          <label>
            {t("description")}
            <input
              value={form.description}
              onChange={(e) => change("description", e.target.value)}
              placeholder={t("modelPurpose")}
            />
          </label>
          <label>
            {t("allowedInputs")}
            <textarea
              value={form.inputs}
              onChange={(e) => change("inputs", e.target.value)}
            />
          </label>
          <label>
            {t("defaultParams")}
            <textarea
              value={form.defaults}
              onChange={(e) => change("defaults", e.target.value)}
            />
          </label>
          <label>
            {t("optionalParams")}
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
            {t("enableModel")}
          </label>
          {error && <p className="form-error">{error}</p>}
          {saved && <p className="form-saved">{saved}</p>}
          <div className="form-actions">
            <button type="button" onClick={reset}>
              {t("reset")}
            </button>
            <button className="primary">
              {editingId ? t("saveChanges") : t("addModel")}
            </button>
          </div>
        </form>
      </div>
    </main>
  );
}
