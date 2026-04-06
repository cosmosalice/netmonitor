import { useState, useEffect, useRef } from 'react'
import {
  getCollectors,
  getCollectorStats,
  startNetFlowCollector,
  stopNetFlowCollector,
  startSFlowCollector,
  stopSFlowCollector,
} from '../api/index'
import { theme } from '../theme'

// ── 颜色常量（Sentry 紫色调主题）──
const COLORS = {
  bg: theme.colors.bgPrimary,
  card: theme.colors.bgDeep,
  text: theme.colors.textPrimary,
  textDim: theme.colors.textDim,
  accent: theme.colors.deepViolet,
  highlight: theme.colors.sentryPurple,
  success: theme.colors.success,
  error: theme.colors.error,
  warning: theme.colors.warning,
  netflow: theme.colors.sentryPurple,
  sflow: theme.colors.pink,
}

interface CollectorStatus {
  type: string
  running: boolean
  port: number
  packets_received: number
  flows_received: number
  bytes_received: number
  errors: number
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    padding: 24,
    background: COLORS.bg,
    minHeight: '100%',
    color: COLORS.text,
    fontFamily: theme.typography.fontFamily,
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 24,
  },
  title: {
    fontSize: 24,
    fontWeight: 600,
    margin: 0,
  },
  subtitle: {
    fontSize: 14,
    color: COLORS.textDim,
    marginTop: 4,
  },
  refreshBtn: {
    background: COLORS.accent,
    color: COLORS.text,
    border: 'none',
    padding: '10px 20px',
    borderRadius: 8,
    cursor: 'pointer',
    fontSize: 14,
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  cardsRow: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, 1fr)',
    gap: 24,
    marginBottom: 24,
  },
  collectorCard: {
    background: COLORS.card,
    borderRadius: 12,
    padding: 24,
    border: '1px solid transparent',
    transition: 'all 0.3s',
  },
  collectorCardActive: {
    borderColor: COLORS.success,
  },
  collectorHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 20,
  },
  collectorTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  },
  collectorIcon: {
    width: 48,
    height: 48,
    borderRadius: 12,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 24,
  },
  collectorName: {
    fontSize: 18,
    fontWeight: 600,
    color: COLORS.text,
  },
  collectorType: {
    fontSize: 12,
    color: COLORS.textDim,
  },
  statusBadge: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    padding: '6px 12px',
    borderRadius: 12,
    fontSize: 12,
    fontWeight: 600,
  },
  statusRunning: {
    background: 'rgba(0, 230, 118, 0.15)',
    color: COLORS.success,
  },
  statusStopped: {
    background: 'rgba(136, 146, 164, 0.15)',
    color: COLORS.textDim,
  },
  configSection: {
    marginBottom: 20,
  },
  configLabel: {
    fontSize: 12,
    color: COLORS.textDim,
    marginBottom: 8,
  },
  configInput: {
    width: '100%',
    padding: '10px 14px',
    background: COLORS.bg,
    border: '1px solid rgba(255, 255, 255, 0.1)',
    borderRadius: 8,
    color: COLORS.text,
    fontSize: 14,
    outline: 'none',
  },
  statsGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, 1fr)',
    gap: 12,
    marginBottom: 20,
  },
  statItem: {
    background: COLORS.bg,
    borderRadius: 8,
    padding: 12,
  },
  statLabel: {
    fontSize: 11,
    color: COLORS.textDim,
    marginBottom: 4,
  },
  statValue: {
    fontSize: 16,
    fontWeight: 600,
    color: COLORS.highlight,
  },
  actionButtons: {
    display: 'flex',
    gap: 12,
  },
  btn: {
    flex: 1,
    padding: '12px 20px',
    borderRadius: 8,
    border: 'none',
    cursor: 'pointer',
    fontSize: 14,
    fontWeight: 600,
    transition: 'all 0.2s',
  },
  btnStart: {
    background: COLORS.success,
    color: theme.colors.bgPrimary,
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  btnStop: {
    background: COLORS.error,
    color: theme.colors.textPrimary,
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  btnDisabled: {
    opacity: 0.5,
    cursor: 'not-allowed',
  },
  flowPreview: {
    background: COLORS.card,
    borderRadius: 12,
    padding: 24,
  },
  flowPreviewTitle: {
    fontSize: 16,
    fontWeight: 600,
    marginBottom: 16,
    color: COLORS.text,
  },
  flowTable: {
    width: '100%',
    borderCollapse: 'collapse',
  },
  flowTableHeader: {
    textAlign: 'left',
    padding: '12px 16px',
    fontSize: 12,
    color: COLORS.textDim,
    fontWeight: 600,
    borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
  },
  flowTableCell: {
    padding: '12px 16px',
    fontSize: 13,
    color: COLORS.text,
    borderBottom: '1px solid rgba(255, 255, 255, 0.05)',
  },
  flowTypeNetFlow: {
    color: COLORS.netflow,
    fontWeight: 600,
  },
  flowTypeSFlow: {
    color: COLORS.sflow,
    fontWeight: 600,
  },
  emptyState: {
    textAlign: 'center',
    padding: 40,
    color: COLORS.textDim,
  },
  errorMessage: {
    background: 'rgba(255, 82, 82, 0.1)',
    color: COLORS.error,
    padding: 12,
    borderRadius: 8,
    marginBottom: 16,
    fontSize: 13,
  },
}

function formatNumber(num: number): string {
  if (num >= 1000000) return (num / 1000000).toFixed(2) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(2) + 'K'
  return num.toString()
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const idx = Math.min(i, units.length - 1)
  return (bytes / Math.pow(1024, idx)).toFixed(2) + ' ' + units[idx]
}

export default function Collectors() {
  const [collectors, setCollectors] = useState<CollectorStatus[]>([])
  const [collectorStats, setCollectorStats] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [netflowPort, setNetflowPort] = useState(2055)
  const [sflowPort, setSflowPort] = useState(6343)
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const intervalRef = useRef<NodeJS.Timeout | null>(null)

  const fetchData = async () => {
    try {
      const [collectorsData, statsData] = await Promise.all([
        getCollectors(),
        getCollectorStats(),
      ])
      setCollectors(collectorsData.collectors || [])
      setCollectorStats(statsData)
      setError(null)
      // Use collectorStats to avoid unused variable warning
      if (collectorStats) {
        console.log('Stats updated')
      }
    } catch (err) {
      setError('Failed to fetch collector data')
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    intervalRef.current = setInterval(fetchData, 3000)
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
    }
  }, [])

  const handleStartNetFlow = async () => {
    setActionLoading('netflow-start')
    try {
      await startNetFlowCollector(netflowPort)
      await fetchData()
    } catch (err) {
      setError('Failed to start NetFlow collector')
    } finally {
      setActionLoading(null)
    }
  }

  const handleStopNetFlow = async () => {
    setActionLoading('netflow-stop')
    try {
      await stopNetFlowCollector()
      await fetchData()
    } catch (err) {
      setError('Failed to stop NetFlow collector')
    } finally {
      setActionLoading(null)
    }
  }

  const handleStartSFlow = async () => {
    setActionLoading('sflow-start')
    try {
      await startSFlowCollector(sflowPort)
      await fetchData()
    } catch (err) {
      setError('Failed to start sFlow collector')
    } finally {
      setActionLoading(null)
    }
  }

  const handleStopSFlow = async () => {
    setActionLoading('sflow-stop')
    try {
      await stopSFlowCollector()
      await fetchData()
    } catch (err) {
      setError('Failed to stop sFlow collector')
    } finally {
      setActionLoading(null)
    }
  }

  const netflowCollector = collectors.find((c) => c.type === 'netflow')
  const sflowCollector = collectors.find((c) => c.type === 'sflow')

  if (loading) {
    return (
      <div style={styles.page}>
        <div style={{ textAlign: 'center', padding: 40 }}>Loading...</div>
      </div>
    )
  }

  return (
    <div style={styles.page}>
      <div style={styles.header}>
        <div>
          <h1 style={styles.title}>Flow 收集器</h1>
          <div style={styles.subtitle}>管理 NetFlow/sFlow 收集器</div>
        </div>
        <button style={styles.refreshBtn} onClick={fetchData}>
          <span>↻</span> 刷新
        </button>
      </div>

      {error && <div style={styles.errorMessage}>{error}</div>}

      <div style={styles.cardsRow}>
        {/* NetFlow Collector Card */}
        <div
          style={{
            ...styles.collectorCard,
            ...(netflowCollector?.running ? styles.collectorCardActive : {}),
          }}
        >
          <div style={styles.collectorHeader}>
            <div style={styles.collectorTitle}>
              <div
                style={{
                  ...styles.collectorIcon,
                  background: 'rgba(106, 95, 193, 0.1)',
                }}
              >
                📊
              </div>
              <div>
                <div style={styles.collectorName}>NetFlow 收集器</div>
                <div style={styles.collectorType}>v5/v9 协议</div>
              </div>
            </div>
            <span
              style={{
                ...styles.statusBadge,
                ...(netflowCollector?.running ? styles.statusRunning : styles.statusStopped),
              }}
            >
              <span
                style={{
                  width: 6,
                  height: 6,
                  borderRadius: '50%',
                  background: netflowCollector?.running ? COLORS.success : COLORS.textDim,
                }}
              />
              {netflowCollector?.running ? '运行中' : '已停止'}
            </span>
          </div>

          <div style={styles.configSection}>
            <div style={styles.configLabel}>监听端口</div>
            <input
              type="number"
              style={styles.configInput}
              value={netflowPort}
              onChange={(e) => setNetflowPort(parseInt(e.target.value) || 2055)}
              disabled={netflowCollector?.running}
              placeholder="2055"
            />
          </div>

          <div style={styles.statsGrid}>
            <div style={styles.statItem}>
              <div style={styles.statLabel}>接收包数</div>
              <div style={styles.statValue}>
                {formatNumber(netflowCollector?.packets_received || 0)}
              </div>
            </div>
            <div style={styles.statItem}>
              <div style={styles.statLabel}>Flow 数</div>
              <div style={styles.statValue}>
                {formatNumber(netflowCollector?.flows_received || 0)}
              </div>
            </div>
            <div style={styles.statItem}>
              <div style={styles.statLabel}>接收字节</div>
              <div style={styles.statValue}>
                {formatBytes(netflowCollector?.bytes_received || 0)}
              </div>
            </div>
            <div style={styles.statItem}>
              <div style={styles.statLabel}>错误数</div>
              <div style={{ ...styles.statValue, color: COLORS.error }}>
                {formatNumber(netflowCollector?.errors || 0)}
              </div>
            </div>
          </div>

          <div style={styles.actionButtons}>
            <button
              style={{
                ...styles.btn,
                ...styles.btnStart,
                ...(netflowCollector?.running || actionLoading === 'netflow-start'
                  ? styles.btnDisabled
                  : {}),
              }}
              onClick={handleStartNetFlow}
              disabled={netflowCollector?.running || actionLoading === 'netflow-start'}
            >
              {actionLoading === 'netflow-start' ? '启动中...' : '启动'}
            </button>
            <button
              style={{
                ...styles.btn,
                ...styles.btnStop,
                ...(!netflowCollector?.running || actionLoading === 'netflow-stop'
                  ? styles.btnDisabled
                  : {}),
              }}
              onClick={handleStopNetFlow}
              disabled={!netflowCollector?.running || actionLoading === 'netflow-stop'}
            >
              {actionLoading === 'netflow-stop' ? '停止中...' : '停止'}
            </button>
          </div>
        </div>

        {/* sFlow Collector Card */}
        <div
          style={{
            ...styles.collectorCard,
            ...(sflowCollector?.running ? styles.collectorCardActive : {}),
          }}
        >
          <div style={styles.collectorHeader}>
            <div style={styles.collectorTitle}>
              <div
                style={{
                  ...styles.collectorIcon,
                  background: 'rgba(250, 127, 170, 0.1)',
                }}
              >
                🌊
              </div>
              <div>
                <div style={styles.collectorName}>sFlow 收集器</div>
                <div style={styles.collectorType}>v5 协议</div>
              </div>
            </div>
            <span
              style={{
                ...styles.statusBadge,
                ...(sflowCollector?.running ? styles.statusRunning : styles.statusStopped),
              }}
            >
              <span
                style={{
                  width: 6,
                  height: 6,
                  borderRadius: '50%',
                  background: sflowCollector?.running ? COLORS.success : COLORS.textDim,
                }}
              />
              {sflowCollector?.running ? '运行中' : '已停止'}
            </span>
          </div>

          <div style={styles.configSection}>
            <div style={styles.configLabel}>监听端口</div>
            <input
              type="number"
              style={styles.configInput}
              value={sflowPort}
              onChange={(e) => setSflowPort(parseInt(e.target.value) || 6343)}
              disabled={sflowCollector?.running}
              placeholder="6343"
            />
          </div>

          <div style={styles.statsGrid}>
            <div style={styles.statItem}>
              <div style={styles.statLabel}>接收包数</div>
              <div style={styles.statValue}>
                {formatNumber(sflowCollector?.packets_received || 0)}
              </div>
            </div>
            <div style={styles.statItem}>
              <div style={styles.statLabel}>Flow 数</div>
              <div style={styles.statValue}>
                {formatNumber(sflowCollector?.flows_received || 0)}
              </div>
            </div>
            <div style={styles.statItem}>
              <div style={styles.statLabel}>接收字节</div>
              <div style={styles.statValue}>
                {formatBytes(sflowCollector?.bytes_received || 0)}
              </div>
            </div>
            <div style={styles.statItem}>
              <div style={styles.statLabel}>错误数</div>
              <div style={{ ...styles.statValue, color: COLORS.error }}>
                {formatNumber(sflowCollector?.errors || 0)}
              </div>
            </div>
          </div>

          <div style={styles.actionButtons}>
            <button
              style={{
                ...styles.btn,
                ...styles.btnStart,
                ...(sflowCollector?.running || actionLoading === 'sflow-start'
                  ? styles.btnDisabled
                  : {}),
              }}
              onClick={handleStartSFlow}
              disabled={sflowCollector?.running || actionLoading === 'sflow-start'}
            >
              {actionLoading === 'sflow-start' ? '启动中...' : '启动'}
            </button>
            <button
              style={{
                ...styles.btn,
                ...styles.btnStop,
                ...(!sflowCollector?.running || actionLoading === 'sflow-stop'
                  ? styles.btnDisabled
                  : {}),
              }}
              onClick={handleStopSFlow}
              disabled={!sflowCollector?.running || actionLoading === 'sflow-stop'}
            >
              {actionLoading === 'sflow-stop' ? '停止中...' : '停止'}
            </button>
          </div>
        </div>
      </div>

      {/* Flow Preview */}
      <div style={styles.flowPreview}>
        <div style={styles.flowPreviewTitle}>最近接收的 Flow 样本</div>
        <div style={styles.emptyState}>
          <div style={{ fontSize: 48, marginBottom: 16 }}>📈</div>
          <div>启动收集器后，这里将显示最近接收的 Flow 样本</div>
          <div style={{ fontSize: 12, marginTop: 8, opacity: 0.7 }}>
            NetFlow 默认端口: 2055 | sFlow 默认端口: 6343
          </div>
        </div>
      </div>
    </div>
  )
}
