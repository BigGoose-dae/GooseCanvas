import { useTheme } from '../theme/ThemeContext'

export default function Brand({ app, compact = false }) {
  const { theme } = useTheme()
  const customLogo = Boolean(app?.logoUrl)
  const defaultLogo = theme === 'dark'
    ? '/brand/goose-canvas-mark-white-transparent.png?v=1'
    : '/brand/goose-canvas-mark-black-transparent-v2.png?v=1'
  const logoUrl = app?.logoUrl || defaultLogo

  return <div className={`brand ${compact ? 'compact' : ''}`}>
    <img
      className={customLogo ? 'custom-logo' : 'default-logo'}
      src={logoUrl}
      alt={`${app?.name || 'Goose Canvas'} logo`}
    />
    <div><strong>{app?.name || 'Goose Canvas'}</strong>{!compact && <span>开放式 AI 创作画布</span>}</div>
  </div>
}
