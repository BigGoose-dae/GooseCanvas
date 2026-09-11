import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import Brand from "./Brand";
import ThemeToggle from "./ThemeToggle";
import LanguageToggle from "./LanguageToggle";
import { useLanguage } from "../i18n/LanguageContext";

export default function SystemSettings() {
  const navigate = useNavigate();
  const { t } = useLanguage();
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
      setSaved(t("settingsSaved"));
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
            ? t("configured")
            : t("enterSecret", { label })
        }
      />
      <small>
        {form[`${key}Configured`]
          ? t("secretStored")
          : t("notConfigured")}
      </small>
    </label>
  );
  return (
    <main className="settings-page">
      <header className="topbar">
        <Brand app={{ name: form?.appName }} />
        <div className="settings-actions">
          <button onClick={() => navigate("/settings/models")}>{t("modelManagement")}</button>
          <LanguageToggle />
          <ThemeToggle />
          <button onClick={() => navigate(-1)}>{t("back")}</button>
        </div>
      </header>
      <div className="system-settings-shell">
        <div className="settings-intro">
          <small>{t("creativeWorkspace")}</small>
          <h1>{t("creativeSettings")}</h1>
          <p>{t("settingsLead")}</p>
          <p>{t("settingsPrivacy")}</p>
        </div>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        {!form ? (
          <p>{t("loadingSettings")}</p>
        ) : (
          <form onSubmit={save} className="system-settings-form">
            <fieldset disabled={busy}>
              <legend>
                <b>01</b> {t("basicSettings")}
              </legend>
              <label>
                {t("appName")}
                <input
                  required
                  value={form.appName}
                  onChange={(event) => change("appName", event.target.value)}
                />
              </label>
            </fieldset>
            <fieldset disabled={busy}>
              <legend>
                <b>02</b> {t("imageVideoGeneration")}
              </legend>
              <p>{t("apiKeyHelp")}</p>
              {secret("apiKey", t("arkApiKey"))}
              <details>
                <summary>{t("advancedConnection")}</summary>
                <label>
                  {t("arkBaseUrl")}
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
                <b>03</b> {t("assetStorage")}
              </legend>
              <p>{t("storageHelp")}</p>
              <label>
                {t("assetDirectory")}
                <input readOnly value={form.assetDirectory} />
                <small>{t("archiveHelp")}</small>
              </label>
            </fieldset>
            <fieldset disabled={busy}>
              <legend>
                <b>04</b> {t("taskExecution")}
              </legend>
              <div className="settings-grid">
                <label>
                  {t("maxConcurrency")}
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
                  <small>{t("concurrencyHelp")}</small>
                </label>
                <label>
                  {t("timeoutMinutes")}
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
                  <small>{t("timeoutHelp")}</small>
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
                {busy ? t("processing") : t("saveSettings")}
              </button>
            </div>
          </form>
        )}
      </div>
    </main>
  );
}
