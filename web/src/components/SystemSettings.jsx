import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import Brand from "./Brand";
import ThemeToggle from "./ThemeToggle";

export default function SystemSettings() {
  const navigate = useNavigate();
  const [form, setForm] = useState(null);
  const [saved, setSaved] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    api
      .settings()
      .then((data) =>
        setForm({ ...data, apiKey: "" }),
      )
      .catch((err) => setError(err.message));
  }, []);
  const change = (key, value) => {
    setForm((current) => ({ ...current, [key]: value }));
    setSaved("");
  };
  const save = async (event) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    setSaved("");
    try {
      const data = await api.saveSettings(form);
      setForm({ ...data, apiKey: "" });
      setSaved("设置已保存并生效，无需重启。");
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };
  const secret = (key, label) => (
    <label>
      {label}
      <input
        type="password"
        autoComplete="new-password"
        value={form[key]}
        onChange={(event) => change(key, event.target.value)}
        placeholder={
          form[`${key}Configured`]
            ? "已配置；留空保留现有凭证"
            : `请输入${label}`
        }
      />
      <small>
        {form[`${key}Configured`]
          ? "已安全提交至本机后端，不回显原值"
          : "尚未配置"}
      </small>
    </label>
  );
  return (
    <main className="settings-page">
      <header className="topbar">
        <Brand app={{ name: form?.appName }} />
        <div className="settings-actions">
          <button onClick={() => navigate("/settings/models")}>模型管理</button>
          <ThemeToggle />
          <button onClick={() => navigate(-1)}>返回</button>
        </div>
      </header>
      <div className="system-settings-shell">
        <div className="settings-intro">
          <small>YOUR CREATIVE WORKSPACE</small>
          <h1>创作设置</h1>
          <p>连接生成模型，上传素材与生成结果会自动保存在本机。</p>
          <p>
            设置保存在本机，密钥不会回显到页面。首次使用只需填写方舟 API Key。
          </p>
        </div>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        {!form ? (
          <p>正在加载配置…</p>
        ) : (
          <form onSubmit={save} className="system-settings-form">
            <fieldset disabled={busy}>
              <legend>
                <b>01</b> 基础设置
              </legend>
              <label>
                应用名称
                <input
                  required
                  value={form.appName}
                  onChange={(event) => change("appName", event.target.value)}
                />
              </label>
            </fieldset>
            <fieldset disabled={busy}>
              <legend>
                <b>02</b> 图片与视频生成
              </legend>
              <p>填写火山方舟控制台的 API Key，用于调用已开通的生成模型。</p>
              {secret("apiKey", "方舟 API Key")}
              <details>
                <summary>高级连接设置</summary>
                <label>
                  方舟服务地址
                  <input
                    required
                    type="url"
                    value={form.baseUrl}
                    onChange={(event) => change("baseUrl", event.target.value)}
                  />
                </label>
              </details>
            </fieldset>
            <fieldset disabled={busy}>
              <legend>
                <b>03</b> 素材存储
              </legend>
              <p>文件上传后保存在本地，调用模型时由 Go 服务编码为 Base64。</p>
              <label>
                本地素材目录
                <input readOnly value={form.assetDirectory} />
                <small>模型返回临时下载地址时，结果也会立即归档到此目录。</small>
              </label>
            </fieldset>
            <fieldset disabled={busy}>
              <legend>
                <b>04</b> 任务执行
              </legend>
              <div className="settings-grid">
                <label>
                  最大并发请求数
                  <input
                    required
                    type="number"
                    min="1"
                    max="32"
                    value={form.concurrency}
                    onChange={(event) =>
                      change("concurrency", Number(event.target.value))
                    }
                  />
                  <small>默认 4。等待视频生成的任务不占用请求名额。</small>
                </label>
                <label>
                  任务查询超时（分钟）
                  <input
                    required
                    type="number"
                    min="1"
                    max="1440"
                    value={form.timeoutMinutes}
                    onChange={(event) =>
                      change("timeoutMinutes", Number(event.target.value))
                    }
                  />
                  <small>超时停止本地查询，可在队列中继续查询原任务。</small>
                </label>
              </div>
            </fieldset>
            {saved && (
              <p className="form-saved" role="status">
                {saved}
              </p>
            )}
            <div className="settings-savebar">
              <button className="primary" type="submit" disabled={busy}>
                {busy ? "处理中…" : "保存设置"}
              </button>
            </div>
          </form>
        )}
      </div>
    </main>
  );
}
