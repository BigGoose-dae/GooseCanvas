import { useTheme } from '../theme/ThemeContext'

export default function Brand({ app, compact = false }) {
  const { theme } = useTheme()
  const defaultLogo = theme === 'dark'
    ? '/brand/goose-canvas-mark-white-on-black.png?v=1'
    : '/brand/goose-canvas-mark-black.png?v=1'

  return <div className={`brand ${compact ? 'compact' : ''}`}>
    <img src={app?.logoUrl || defaultLogo} alt={`${app?.name || 'Goose Canvas'} logo`} />
    <div><strong>{app?.name || 'Goose Canvas'}</strong>{!compact && <span>开放式 AI 创作画布</span>}</div>
  </div>
}
