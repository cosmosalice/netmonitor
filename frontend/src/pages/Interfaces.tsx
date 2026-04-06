import { useState, useEffect, useRef } from 'react'
import {
  Chart,
  CategoryScale,
  LinearScale,
  BarElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js'
import { Bar } from 'react-chartjs-2'
import {
  getAllInterfaces,
  getActiveInterfaceList,
  enableInterface,
  disableInterface,
  getInterfacesAggregateStats,
} from '../api/index'
import { theme } from '../theme'

Chart.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend)

// ── 颜色常量 ──
const COLORS = {
  bg: theme.colors.bgPrimary,
  card: theme.colors.bgDeep,
  text: theme.colors.textPrimary,
  textDim: theme.colors.textDim,
  accent: theme.colors.border,
  highlight: theme.colors.sentryPurple,
  success: theme.colors.success,
  error: theme.colors.error,
  warning: theme.colors.warning,
  chartColors: [theme.colors.sentryPurple, '#e94560', theme.colors.success, theme.colors.warning, '#ab47bc', '#ff7043', '#42a5f5', '#66bb6a'],
}

interface InterfaceStatus {
  name: string
  description: string
  ip_address: string
  mac_address: string
  is_up: boolean
  is_enabled: boolean
  is_capturing: boolean
  packets_captured: number
  bytes_captured: number
  last_started?: string
  error?: string
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
    background: theme.colors.sentryPurple,
    color: '#fff',
    border: 'none',
    padding: '10px 20px',
    borderRadius: 8,
    cursor: 'pointer',
    fontSize: 14,
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    textTransform: 'uppercase' as const,
    letterSpacing: '0.2px',
  },
  statsRow: {
    display: 'grid',
    gridTemplateColumns: 'repeat(4, 1fr)',
    gap: 16,
    marginBottom: 24,
  },
  statCard: {
    background: COLORS.card,
    borderRadius: 12,
    padding: '20px 24px',
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
  },
  statLabel: {
    fontSize: 13,
    color: COLORS.textDim,
    letterSpacing: 0.5,
  },
  statValue: {
    fontSize: 24,
    fontWeight: 700,
    color: COLORS.highlight,
  },
  contentGrid: {
    display: 'grid',
    gridTemplateColumns: '2fr 1fr',
    gap: 24,
  },
  card: {
    background: COLORS.card,
    borderRadius: 12,
    padding: 20,
  },
  cardTitle: {
    fontSize: 16,
    fontWeight: 600,
    marginBottom: 16,
    color: COLORS.text,
  },
  interfaceList: {
    display: 'flex',
    flexDirection: 'column',
    gap: 12,
  },
  interfaceItem: {
    background: COLORS.bg,
    borderRadius: 8,
    padding: 16,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    border: '1px solid transparent',
    transition: 'all 0.2s',
  },
  interfaceItemActive: {
    borderColor: COLORS.success,
  },
  interfaceInfo: {
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
    flex: 1,
  },
  interfaceName: {
    fontSize: 14,
    fontWeight: 600,
    color: COLORS.text,
  },
  interfaceDesc: {
    fontSize: 12,
    color: COLORS.textDim,
  },
  interfaceMeta: {
    fontSize: 11,
    color: COLORS.textDim,
    display: 'flex',
    gap: 12,
  },
  interfaceStats: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'flex-end',
    gap: 4,
    marginRight: 16,
  },
  statPackets: {
    fontSize: 12,
    color: COLORS.highlight,
  },
  statBytes: {
    fontSize: 11,
    color: COLORS.textDim,
  },
  toggleBtn: {
    padding: '8px 16px',
    borderRadius: 6,
    border: 'none',
    cursor: 'pointer',
    fontSize: 12,
    fontWeight: 600,
    transition: 'all 0.2s',
  },
  toggleBtnEnable: {
    background: COLORS.success,
    color: '#000',
  },
  toggleBtnDisable: {
    background: COLORS.error,
    color: '#fff',
  },
  statusBadge: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    padding: '4px 10px',
    borderRadius: 12,
    fontSize: 11,
    fontWeight: 600,
  },
  statusActive: {
    background: 'rgba(194, 239, 78, 0.15)',
    color: COLORS.success,
  },
  statusInactive: {
    background: 'rgba(139, 143, 163, 0.15)',
    color: COLORS.textDim,
  },
  chartContainer: {
    height: 300,
    marginTop: 16,
  },
  errorText: {
    color: COLORS.error,
    fontSize: 12,
    marginTop: 4,
  },
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const idx = Math.min(i, units.length - 1)
  return (bytes / Math.pow(1024, idx)).toFixed(2) + ' ' + units[idx]
}

function formatNumber(num: number): string {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toString()
}

export default function Interfaces() {
  const [interfaces, setInterfaces] = useState<InterfaceStatus[]>([])
  const [activeInterfaces, setActiveInterfaces] = useState<InterfaceStatus[]>([])
  const [aggregateStats, setAggregateStats] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [, setErrorState] = useState<string | null>(null)
  const intervalRef = useRef<NodeJS.Timeout | null>(null)

  const fetchData = async () => {
    try {
      const [allData, activeData, aggData] = await Promise.all([
        getAllInterfaces(),
        getActiveInterfaceList(),
        getInterfacesAggregateStats(),
      ])
      setInterfaces(allData.interfaces || [])
      setActiveInterfaces(activeData.interfaces || [])
      setAggregateStats(aggData)
      setErrorState(null)
    } catch (err) {
      setErrorState('Failed to fetch interface data')
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    intervalRef.current = setInterval(fetchData, 5000)
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
    }
  }, [])

  const handleToggleInterface = async (name: string, isEnabled: boolean) => {
    try {
      if (isEnabled) {
        await disableInterface(name)
      } else {
        await enableInterface(name)
      }
      fetchData()
    } catch (err) {
      console.error('Failed to toggle interface:', err)
    }
  }

  // Prepare chart data
  const chartData = {
    labels: interfaces.map((iface) => iface.name),
    datasets: [
      {
        label: 'Packets Captured',
        data: interfaces.map((iface) => iface.packets_captured),
        backgroundColor: COLORS.chartColors[0],
        borderRadius: 4,
      },
      {
        label: 'Bytes Captured',
        data: interfaces.map((iface) => iface.bytes_captured / 1024), // KB
        backgroundColor: COLORS.chartColors[1],
        borderRadius: 4,
      },
    ],
  }

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        position: 'top' as const,
        labels: {
          color: COLORS.text,
          font: { size: 12 },
        },
      },
    },
    scales: {
      x: {
        ticks: {
          color: COLORS.textDim,
          font: { size: 11 },
        },
        grid: {
          color: 'rgba(255, 255, 255, 0.05)',
        },
      },
      y: {
        ticks: {
          color: COLORS.textDim,
          font: { size: 11 },
        },
        grid: {
          color: 'rgba(255, 255, 255, 0.05)',
        },
      },
    },
  }

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
          <h1 style={styles.title}>网络接口管理</h1>
          <div style={styles.subtitle}>管理多个网络接口的抓包状态</div>
        </div>
        <button style={styles.refreshBtn} onClick={fetchData}>
          <span>↻</span> 刷新
        </button>
      </div>

      <div style={styles.statsRow}>
        <div style={styles.statCard}>
          <div style={styles.statLabel}>总接口数</div>
          <div style={styles.statValue}>{interfaces.length}</div>
        </div>
        <div style={styles.statCard}>
          <div style={styles.statLabel}>活跃接口</div>
          <div style={{ ...styles.statValue, color: COLORS.success }}>
            {activeInterfaces.length}
          </div>
        </div>
        <div style={styles.statCard}>
          <div style={styles.statLabel}>总包数</div>
          <div style={styles.statValue}>
            {formatNumber(aggregateStats?.total_packets || 0)}
          </div>
        </div>
        <div style={styles.statCard}>
          <div style={styles.statLabel}>总流量</div>
          <div style={styles.statValue}>
            {formatBytes(aggregateStats?.total_bytes || 0)}
          </div>
        </div>
      </div>

      <div style={styles.contentGrid}>
        <div style={styles.card}>
          <div style={styles.cardTitle}>接口列表</div>
          <div style={styles.interfaceList}>
            {interfaces.map((iface) => (
              <div
                key={iface.name}
                style={{
                  ...styles.interfaceItem,
                  ...(iface.is_enabled ? styles.interfaceItemActive : {}),
                }}
              >
                <div style={styles.interfaceInfo}>
                  <div style={styles.interfaceName}>
                    {iface.name}
                    <span
                      style={{
                        ...styles.statusBadge,
                        ...(iface.is_enabled ? styles.statusActive : styles.statusInactive),
                        marginLeft: 12,
                      }}
                    >
                      <span
                        style={{
                          width: 6,
                          height: 6,
                          borderRadius: '50%',
                          background: iface.is_enabled ? COLORS.success : COLORS.textDim,
                        }}
                      />
                      {iface.is_enabled ? '运行中' : '已停止'}
                    </span>
                  </div>
                  <div style={styles.interfaceDesc}>
                    {iface.description || 'No description'}
                  </div>
                  <div style={styles.interfaceMeta}>
                    <span>IP: {iface.ip_address || 'N/A'}</span>
                    <span>MAC: {iface.mac_address || 'N/A'}</span>
                    <span>状态: {iface.is_up ? 'Up' : 'Down'}</span>
                  </div>
                  {iface.error && (
                    <div style={styles.errorText}>Error: {iface.error}</div>
                  )}
                </div>

                <div style={styles.interfaceStats}>
                  <div style={styles.statPackets}>
                    {formatNumber(iface.packets_captured)} pkts
                  </div>
                  <div style={styles.statBytes}>
                    {formatBytes(iface.bytes_captured)}
                  </div>
                </div>

                <button
                  style={{
                    ...styles.toggleBtn,
                    ...(iface.is_enabled
                      ? styles.toggleBtnDisable
                      : styles.toggleBtnEnable),
                  }}
                  onClick={() => handleToggleInterface(iface.name, iface.is_enabled)}
                >
                  {iface.is_enabled ? '停止' : '启动'}
                </button>
              </div>
            ))}
          </div>
        </div>

        <div>
          <div style={styles.card}>
            <div style={styles.cardTitle}>流量对比</div>
            <div style={styles.chartContainer}>
              <Bar data={chartData} options={chartOptions} />
            </div>
          </div>

          <div style={{ ...styles.card, marginTop: 24 }}>
            <div style={styles.cardTitle}>活跃接口详情</div>
            <div style={styles.interfaceList}>
              {activeInterfaces.length === 0 ? (
                <div style={{ textAlign: 'center', color: COLORS.textDim, padding: 20 }}>
                  暂无活跃接口
                </div>
              ) : (
                activeInterfaces.map((iface) => (
                  <div key={iface.name} style={styles.interfaceItem}>
                    <div style={styles.interfaceInfo}>
                      <div style={styles.interfaceName}>{iface.name}</div>
                      <div style={styles.interfaceMeta}>
                        <span>IP: {iface.ip_address || 'N/A'}</span>
                      </div>
                    </div>
                    <div style={styles.interfaceStats}>
                      <div style={styles.statPackets}>
                        {formatNumber(iface.packets_captured)} pkts
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
