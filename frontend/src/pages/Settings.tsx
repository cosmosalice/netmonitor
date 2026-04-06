import { useState, useEffect, useCallback } from 'react'
import {
  getCaptureStatus,
  getInterfaces,
  stopCapture,
  updateConfig,
} from '../api/index'
import api from '../api/index'
import { theme } from '../theme'

// ── Types ───────────────────────────────────────────────────────────────────

interface CaptureStatusData {
  is_running: boolean
  stats?: {
    packets_captured: number
    start_time?: string
  }
}

interface NetworkInterface {
  name: string
  description: string
  ip_address: string
  mac_address: string
  is_up: boolean
}

// ── Helpers ─────────────────────────────────────────────────────────────────

const formatDuration = (startTime?: string): string => {
  if (!startTime) return '-'
  const start = new Date(startTime).getTime()
  if (isNaN(start)) return '-'
  const diff = Math.floor((Date.now() - start) / 1000)
  if (diff < 0) return '-'
  const h = Math.floor(diff / 3600)
  const m = Math.floor((diff % 3600) / 60)
  const s = diff % 60
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
}

// ── Colors & Styles ─────────────────────────────────────────────────────────

const colors = {
  bg: theme.colors.bgPrimary,           // 替代 '#1a1a2e'
  card: theme.colors.bgDeep,            // 替代 '#16213e'
  text: theme.colors.textPrimary,       // 替代 '#e0e0e0' → '#ffffff'
  textDim: theme.colors.textDim,        // 替代 '#8892a4'
  accent: theme.colors.sentryPurple,    // 替代 '#00d4ff' → '#6a5fc1'
  accentDark: theme.colors.border,      // 替代 '#0f3460' → '#362d59'
  border: theme.colors.borderLight,     // 替代 '#2a3a5c' → '#584674'
  success: theme.colors.success,        // 替代 '#00c853' → '#c2ef4e'
  danger: theme.colors.error,           // '#ff5252'
  warn: theme.colors.warning,           // 替代 '#ffab00' → '#ffb287'
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    padding: '24px',
    minHeight: '100vh',
    backgroundColor: colors.bg,
    color: colors.text,
    fontFamily: theme.typography.fontFamily,
  },
  header: {
    fontSize: '22px',
    fontWeight: 700,
    marginBottom: '24px',
    color: colors.accent,
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
  },
  section: {
    marginBottom: '24px',
    borderRadius: theme.radii.lg,
    border: `1px solid ${theme.colors.border}`,
    backgroundColor: colors.card,
    padding: '20px',
    boxShadow: theme.shadows.elevated,
  },
  sectionTitle: {
    fontSize: '15px',
    fontWeight: 600,
    marginBottom: '16px',
    color: colors.accent,
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
  },
  statusRow: {
    display: 'flex',
    alignItems: 'center',
    gap: '20px',
    flexWrap: 'wrap' as const,
  },
  indicator: {
    width: '12px',
    height: '12px',
    borderRadius: '50%',
    display: 'inline-block',
  },
  statItem: {
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    padding: '10px 20px',
    borderRadius: '8px',
    backgroundColor: colors.accentDark,
    minWidth: '120px',
  },
  statLabel: {
    fontSize: '12px',
    color: colors.textDim,
    marginBottom: '4px',
  },
  statValue: {
    fontSize: '20px',
    fontWeight: 700,
    color: colors.accent,
  },
  formGroup: {
    marginBottom: '14px',
  },
  label: {
    display: 'block',
    fontSize: '13px',
    color: colors.textDim,
    marginBottom: '6px',
  },
  select: {
    width: '100%',
    padding: '10px 14px',
    borderRadius: '6px',
    border: '1px solid #cfcfdb',
    backgroundColor: '#ffffff',
    color: '#1f1633',
    fontSize: '14px',
    outline: 'none',
    cursor: 'pointer',
  },
  input: {
    width: '100%',
    padding: '10px 14px',
    borderRadius: '6px',
    border: '1px solid #cfcfdb',
    backgroundColor: '#ffffff',
    color: '#1f1633',
    fontSize: '14px',
    outline: 'none',
    boxSizing: 'border-box' as const,
  },
  chipRow: {
    display: 'flex',
    gap: '8px',
    flexWrap: 'wrap' as const,
    marginTop: '8px',
  },
  chip: {
    padding: '5px 12px',
    borderRadius: '14px',
    border: `1px solid ${colors.border}`,
    backgroundColor: colors.accentDark,
    color: colors.accent,
    fontSize: '12px',
    cursor: 'pointer',
    transition: 'all 0.15s',
  },
  btnPrimary: {
    padding: '10px 24px',
    borderRadius: theme.radii.pill,
    border: 'none',
    backgroundColor: theme.colors.assyrian,
    color: '#ffffff',
    fontSize: '14px',
    fontWeight: 600,
    cursor: 'pointer',
    transition: 'opacity 0.15s',
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
    boxShadow: theme.shadows.sunken,
  },
  btnDanger: {
    padding: '10px 24px',
    borderRadius: theme.radii.pill,
    border: 'none',
    backgroundColor: colors.danger,
    color: '#fff',
    fontSize: '14px',
    fontWeight: 600,
    cursor: 'pointer',
    transition: 'opacity 0.15s',
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  btnSecondary: {
    padding: '10px 24px',
    borderRadius: theme.radii.pill,
    border: `1px solid ${colors.border}`,
    backgroundColor: 'transparent',
    color: colors.text,
    fontSize: '14px',
    cursor: 'pointer',
    transition: 'all 0.15s',
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  toast: {
    position: 'fixed' as const,
    bottom: '24px',
    right: '24px',
    padding: '12px 20px',
    borderRadius: '8px',
    fontSize: '14px',
    fontWeight: 500,
    zIndex: 9999,
    boxShadow: '0 4px 20px rgba(0,0,0,0.4)',
    transition: 'opacity 0.3s',
  },
  controlRow: {
    display: 'flex',
    gap: '12px',
    alignItems: 'center',
    marginTop: '16px',
    flexWrap: 'wrap' as const,
  },
  ifaceInfo: {
    fontSize: '12px',
    color: colors.textDim,
    marginTop: '4px',
  },
}

// ── Component ───────────────────────────────────────────────────────────────

const Settings = () => {
  // Capture status
  const [captureStatus, setCaptureStatus] = useState<CaptureStatusData | null>(null)
  const [statusLoading, setStatusLoading] = useState(true)

  // Interfaces
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([])
  const [selectedIface, setSelectedIface] = useState('')

  // BPF filter
  const [bpfFilter, setBpfFilter] = useState('')

  // Data retention
  const [retentionHours, setRetentionHours] = useState(24)
  const [retentionDays, setRetentionDays] = useState('1')

  // Feedback toast
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'error' } | null>(null)

  // Duration ticker
  const [, setTick] = useState(0)

  const showToast = useCallback((msg: string, type: 'success' | 'error') => {
    setToast({ msg, type })
    setTimeout(() => setToast(null), 3000)
  }, [])

  // ── Fetch capture status ──────────────────────────────────────────────
  const fetchStatus = useCallback(async () => {
    try {
      const data = await getCaptureStatus()
      setCaptureStatus(data)
    } catch (err) {
      console.error('Failed to fetch capture status:', err)
    } finally {
      setStatusLoading(false)
    }
  }, [])

  // ── Fetch interfaces ──────────────────────────────────────────────────
  const fetchInterfaces = useCallback(async () => {
    try {
      const data = await getInterfaces()
      const list: NetworkInterface[] = data.interfaces || []
      setInterfaces(list)
      if (list.length > 0 && !selectedIface) {
        setSelectedIface(list[0].name)
      }
    } catch (err) {
      console.error('Failed to fetch interfaces:', err)
    }
  }, [selectedIface])

  // ── Fetch config ──────────────────────────────────────────────────────
  const fetchConfig = useCallback(async () => {
    try {
      const resp = await api.get('/config')
      const cfg = resp.data
      if (cfg.retention_hours) {
        setRetentionHours(cfg.retention_hours)
        setRetentionDays(String(Math.round(cfg.retention_hours / 24) || 1))
      }
      if (cfg.bpf_filter) setBpfFilter(cfg.bpf_filter)
      if (cfg.interface) setSelectedIface(cfg.interface)
    } catch {
      // ignore — use defaults
    }
  }, [])

  useEffect(() => {
    fetchStatus()
    fetchInterfaces()
    fetchConfig()
    const statusTimer = setInterval(fetchStatus, 5000)
    return () => clearInterval(statusTimer)
  }, [fetchStatus, fetchInterfaces, fetchConfig])

  // Duration ticker: re-render every second when capture running
  useEffect(() => {
    if (!captureStatus?.is_running) return
    const id = setInterval(() => setTick((t) => t + 1), 1000)
    return () => clearInterval(id)
  }, [captureStatus?.is_running])

  // ── Handlers ──────────────────────────────────────────────────────────

  const handleStart = async () => {
    if (!selectedIface) {
      showToast('请先选择网卡', 'error')
      return
    }
    try {
      // The backend accepts bpf_filter in the startCapture body.
      // Our api client only sends interface, so we call axios directly.
      await api.post('/capture/start', {
        interface: selectedIface,
        bpf_filter: bpfFilter,
      })
      showToast('抓包已启动', 'success')
      fetchStatus()
    } catch (err: any) {
      showToast(err?.response?.data?.error || '启动失败', 'error')
    }
  }

  const handleStop = async () => {
    try {
      await stopCapture()
      showToast('抓包已停止', 'success')
      fetchStatus()
    } catch (err: any) {
      showToast(err?.response?.data?.error || '停止失败', 'error')
    }
  }

  const handleSaveRetention = async () => {
    const days = parseInt(retentionDays, 10)
    if (isNaN(days) || days < 1) {
      showToast('请输入有效的天数（≥1）', 'error')
      return
    }
    try {
      await updateConfig({ retention_hours: days * 24 })
      setRetentionHours(days * 24)
      showToast('数据保留配置已保存', 'success')
    } catch (err: any) {
      showToast(err?.response?.data?.error || '保存失败', 'error')
    }
  }

  const isRunning = captureStatus?.is_running ?? false
  const packetsCaptured = captureStatus?.stats?.packets_captured ?? 0
  const startTime = captureStatus?.stats?.start_time

  const quickFilters = ['tcp', 'udp', 'port 80', 'port 443', 'icmp', 'port 53']

  // ── Render ────────────────────────────────────────────────────────────

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <span>⚙</span> 系统设置
      </div>

      {/* ─── Section 1: Capture Status ─────────────────────────────── */}
      <div style={styles.section}>
        <div style={styles.sectionTitle}>📡 抓包状态</div>

        {statusLoading ? (
          <div style={{ color: colors.textDim }}>加载中...</div>
        ) : (
          <div style={styles.statusRow}>
            {/* Running indicator */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span
                style={{
                  ...styles.indicator,
                  backgroundColor: isRunning ? colors.success : colors.danger,
                  boxShadow: isRunning
                    ? `0 0 8px ${colors.success}`
                    : `0 0 8px ${colors.danger}`,
                }}
              />
              <span style={{ fontWeight: 600 }}>{isRunning ? '运行中' : '已停止'}</span>
            </div>

            <div style={styles.statItem}>
              <div style={styles.statLabel}>已捕获包数</div>
              <div style={styles.statValue}>{packetsCaptured.toLocaleString()}</div>
            </div>

            <div style={styles.statItem}>
              <div style={styles.statLabel}>运行时间</div>
              <div style={{ ...styles.statValue, fontSize: '16px' }}>
                {isRunning ? formatDuration(startTime) : '-'}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* ─── Section 2: Capture Config ─────────────────────────────── */}
      <div style={styles.section}>
        <div style={styles.sectionTitle}>🔧 抓包配置</div>

        {/* Interface selector */}
        <div style={styles.formGroup}>
          <label style={styles.label}>网卡选择</label>
          <select
            style={styles.select}
            value={selectedIface}
            onChange={(e) => setSelectedIface(e.target.value)}
            disabled={isRunning}
          >
            {interfaces.length === 0 && <option value="">暂无可用网卡</option>}
            {interfaces.map((iface) => (
              <option key={iface.name} value={iface.name}>
                {iface.description || iface.name}
                {iface.ip_address ? ` (${iface.ip_address})` : ''}
              </option>
            ))}
          </select>
          {selectedIface && (
            <div style={styles.ifaceInfo}>
              {(() => {
                const iface = interfaces.find((i) => i.name === selectedIface)
                if (!iface) return null
                return (
                  <>
                    名称: {iface.name}
                    {iface.mac_address ? ` | MAC: ${iface.mac_address}` : ''}
                    {iface.is_up ? ' | 状态: UP' : ' | 状态: DOWN'}
                  </>
                )
              })()}
            </div>
          )}
        </div>

        {/* BPF Filter */}
        <div style={styles.formGroup}>
          <label style={styles.label}>BPF 过滤表达式</label>
          <input
            style={styles.input}
            placeholder="例如: tcp port 80 and host 192.168.1.1"
            value={bpfFilter}
            onChange={(e) => setBpfFilter(e.target.value)}
            disabled={isRunning}
          />
          <div style={styles.chipRow}>
            {quickFilters.map((f) => (
              <span
                key={f}
                style={{
                  ...styles.chip,
                  opacity: isRunning ? 0.5 : 1,
                  pointerEvents: isRunning ? 'none' : 'auto',
                }}
                onClick={() => setBpfFilter(f)}
              >
                {f}
              </span>
            ))}
          </div>
        </div>

        {/* Control buttons */}
        <div style={styles.controlRow}>
          {!isRunning ? (
            <button style={styles.btnPrimary} onClick={handleStart}>
              ▶ 开始抓包
            </button>
          ) : (
            <button style={styles.btnDanger} onClick={handleStop}>
              ■ 停止抓包
            </button>
          )}
        </div>
      </div>

      {/* ─── Section 3: Data Management ────────────────────────────── */}
      <div style={styles.section}>
        <div style={styles.sectionTitle}>💾 数据管理</div>

        <div style={styles.formGroup}>
          <label style={styles.label}>数据保留天数</label>
          <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
            <input
              type="number"
              min="1"
              style={{ ...styles.input, width: '120px' }}
              value={retentionDays}
              onChange={(e) => setRetentionDays(e.target.value)}
            />
            <span style={{ color: colors.textDim, fontSize: '13px' }}>
              天（当前: {retentionHours} 小时）
            </span>
          </div>
        </div>

        <div style={styles.controlRow}>
          <button style={styles.btnPrimary} onClick={handleSaveRetention}>
            保存配置
          </button>
        </div>
      </div>

      {/* Toast */}
      {toast && (
        <div
          style={{
            ...styles.toast,
            backgroundColor: toast.type === 'success' ? colors.success : colors.danger,
            color: '#fff',
          }}
        >
          {toast.msg}
        </div>
      )}
    </div>
  )
}

export default Settings
