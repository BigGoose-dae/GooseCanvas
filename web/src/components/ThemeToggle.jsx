import { useTheme } from '../theme/ThemeContext'
import { useLanguage } from '../i18n/LanguageContext'

export default function ThemeToggle() {
  const { theme, toggleTheme } = useTheme()
  const { t } = useLanguage()
  const nextLabel = theme === 'dark' ? t('themeLight') : t('themeDark')

  return <button type="button" className="theme-toggle" onClick={toggleTheme} aria-label={nextLabel} title={nextLabel}>
    <span aria-hidden="true">{theme === 'dark' ? '☀' : '☾'}</span>
  </button>
}
