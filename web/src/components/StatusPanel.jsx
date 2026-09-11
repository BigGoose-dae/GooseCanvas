import { useLanguage } from "../i18n/LanguageContext";

export default function StatusPanel({ status }) {
  const { t } = useLanguage();
  if (!status || status.ready) return null;
  return (
    <div className="status-panel">
      <div>
        <strong>{t("setupTitle")}</strong>
        <span>{t("setupDescription")}</span>
      </div>
      <a href="/settings">{t("openSettings")}</a>
      <div className="status-tags">
        {(status.missing || []).map((item) => (
          <code key={item}>
            {{
              VOLCENGINE_API_KEY: t("modelCredential"),
            }[item] || item}
          </code>
        ))}
        {status.storage?.status === "error" && <code>{t("storageUnavailable")}</code>}
      </div>
      {status.storage?.message && <small>{status.storage.message}</small>}
    </div>
  );
}
