import { useTheme } from '../theme/ThemeContext'

export default function ThemeToggle() {
  const { theme, toggleTheme } = useTheme()
  const nextLabel = theme === 'dark' ? '切换到日间模式' : '切换到夜间模式'

  return <button type="button" className="theme-toggle" onClick={toggleTheme} aria-label={nextLabel} title={nextLabel}>
    <span aria-hidden="true">{theme === 'dark' ? '☀' : '☾'}</span>
  </button>
}
