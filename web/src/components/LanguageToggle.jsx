import { useLanguage } from "../i18n/LanguageContext";

export default function LanguageToggle() {
  const { t, toggleLanguage } = useLanguage();
  return (
    <button
      type="button"
      className="language-toggle"
      onClick={toggleLanguage}
      aria-label={t("languageLabel")}
      title={t("languageLabel")}
    >
      {t("language")}
    </button>
  );
}
