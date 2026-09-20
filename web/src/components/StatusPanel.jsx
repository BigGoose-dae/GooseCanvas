import { useLanguage } from "../i18n/LanguageContext";

export default function StatusPanel({ status }) {
  const { t } = useLanguage();
  if (!status) return null;
  const configured = status.model?.status === "configured";
  return (
    <section className={`model-key-panel ${configured ? "configured" : "missing"}`}>
      <div className="model-key-mark" aria-hidden="true">
        <span>KEY</span>
        <i />
      </div>
      <div className="model-key-copy">
        <small>{t("modelAccess")}</small>
        <strong>{t(configured ? "modelKeyConfigured" : "modelKeyMissing")}</strong>
        <span>
          {t(
            configured
              ? "modelKeyConfiguredDescription"
              : "modelKeyMissingDescription",
          )}
        </span>
      </div>
      <div className="model-key-meta">
        <span>
          <i /> {configured ? t("available") : t("actionRequired")}
        </span>
        {status.storage?.status === "error" && (
          <em title={status.storage?.message || ""}>{t("storageUnavailable")}</em>
        )}
      </div>
      <a href="/settings">{t(configured ? "manageModelKey" : "configureModelKey")} →</a>
    </section>
  );
}
