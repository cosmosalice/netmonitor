import { useState, useEffect, useRef, useCallback } from 'react'
import {
  Chart,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  ArcElement,
  BarElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { Line, Doughnut, Bar } from 'react-chartjs-2'
import { MapContainer, TileLayer, CircleMarker, Tooltip as LeafletTooltip } from 'react-leaflet'
import 'leaflet/dist/leaflet.css'
import { getSummaryStats, getCaptureStatus, getProtocolStats, getHostStats, getGeoHosts, getCountryStats, getASNStats } from '../api/index'
import WebSocketClient from '../api/websocket'
import { theme } from '../theme'

Chart.register(CategoryScale, LinearScale, PointElement, LineElement, ArcElement, BarElement, Title, Tooltip, Legend, Filler)

// ── 工具函数 ──
function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const idx = Math.min(i, units.length - 1)
  return (bytes / Math.pow(1024, idx)).toFixed(2) + ' ' + units[idx]
}

function formatBandwidth(bytesPerSec: number): string {
  if (bytesPerSec <= 0) return '0 B/s'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  const i = Math.floor(Math.log(bytesPerSec) / Math.log(1024))
  const idx = Math.min(i, units.length - 1)
  return (bytesPerSec / Math.pow(1024, idx)).toFixed(2) + ' ' + units[idx]
}

function formatUptime(seconds: number): string {
  if (!seconds || seconds <= 0) return '0秒'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  const parts: string[] = []
  if (h > 0) parts.push(`${h}小时`)
  if (m > 0) parts.push(`${m}分`)
  parts.push(`${s}秒`)
  return parts.join(' ')
}

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
  chartColors: theme.colors.chartColors,
}

function dotStyle(active: boolean): React.CSSProperties {
  return {
    width: 10,
    height: 10,
    borderRadius: '50%',
    background: active ? COLORS.success : COLORS.error,
    boxShadow: active ? `0 0 8px ${COLORS.success}` : 'none',
  }
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    padding: 24,
    background: COLORS.bg,
    minHeight: '100%',
    color: COLORS.text,
    fontFamily: theme.typography.fontFamily,
  },
  cardsRow: {
    display: 'grid',
    gridTemplateColumns: 'repeat(4, 1fr)',
    gap: 16,
    marginBottom: 24,
  },
  card: {
    background: COLORS.card,
    borderRadius: 12,
    padding: '20px 24px',
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
    boxShadow: theme.shadows.elevated,
  },
  cardLabel: {
    fontSize: 13,
    color: COLORS.textDim,
    letterSpacing: 0.5,
  },
  cardValue: {
    fontSize: 28,
    fontWeight: 700,
    color: COLORS.highlight,
  },
  chartsRow: {
    display: 'grid',
    gridTemplateColumns: '2fr 1fr',
    gap: 16,
    marginBottom: 24,
  },
  chartBox: {
    background: COLORS.card,
    borderRadius: 12,
    padding: 20,
    boxShadow: theme.shadows.elevated,
  },
  chartTitle: {
    fontSize: 15,
    fontWeight: 600,
    marginBottom: 12,
    color: COLORS.text,
  },
  bottomRow: {
    display: 'grid',
    gridTemplateColumns: '2fr 1fr',
    gap: 16,
  },
  table: {
    width: '100%',
    borderCollapse: 'collapse' as const,
    fontSize: 13,
  },
  th: {
    textAlign: 'left' as const,
    padding: '10px 12px',
    borderBottom: `1px solid ${theme.colors.border}`,
    color: COLORS.textDim,
    fontWeight: 600,
  },
  td: {
    padding: '10px 12px',
    borderBottom: `1px solid rgba(255,255,255,0.05)`,
  },
  statusRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    marginBottom: 10,
  },

  placeholder: {
    color: COLORS.textDim,
    textAlign: 'center' as const,
    padding: 40,
  },
}

// ── StatsCards ──
function StatsCards({ stats, alertCount, countryCount, asnCount }: { stats: any; alertCount: number; countryCount: number; asnCount: number }) {
  const items = [
    { label: '总带宽', value: stats ? formatBandwidth(stats.total_bandwidth || stats.totalBandwidth || 0) : '加载中...', color: COLORS.highlight },
    { label: '活跃主机数', value: stats ? String(stats.active_hosts ?? stats.activeHosts ?? 0) : '加载中...', color: COLORS.highlight },
    { label: '活跃流数', value: stats ? String(stats.active_flows ?? stats.activeFlows ?? 0) : '加载中...', color: COLORS.highlight },
    { label: '检测协议数', value: stats ? String(stats.detected_protocols ?? stats.detectedProtocols ?? 0) : '加载中...', color: COLORS.highlight },
    { label: '告警数', value: String(alertCount), color: COLORS.error },
    { label: '国家数', value: String(countryCount), color: '#ffc107' },
    { label: 'ASN 数', value: String(asnCount), color: '#ab47bc' },
  ]
  return (
    <div style={{ ...styles.cardsRow, gridTemplateColumns: 'repeat(7, 1fr)' }}>
      {items.map((it) => (
        <div key={it.label} style={styles.card}>
          <span style={styles.cardLabel}>{it.label}</span>
          <span style={{ ...styles.cardValue, color: it.color, fontSize: 24 }}>{it.value}</span>
        </div>
      ))}
    </div>
  )
}

// ── BandwidthChart ──
function BandwidthChart({ dataPoints }: { dataPoints: { time: string; value: number }[] }) {
  const data = {
    labels: dataPoints.map((p) => p.time),
    datasets: [
      {
        label: '带宽',
        data: dataPoints.map((p) => p.value),
        borderColor: COLORS.highlight,
        backgroundColor: 'rgba(106,95,193,0.1)',
        fill: true,
        tension: 0.4,
        pointRadius: 0,
        borderWidth: 2,
      },
    ],
  }
  const options: any = {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 0 },
    scales: {
      x: {
        ticks: { color: COLORS.textDim, maxTicksLimit: 10, font: { size: 11 } },
        grid: { color: 'rgba(255,255,255,0.05)' },
      },
      y: {
        ticks: {
          color: COLORS.textDim,
          font: { size: 11 },
          callback: (v: number) => formatBandwidth(v),
        },
        grid: { color: 'rgba(255,255,255,0.05)' },
      },
    },
    plugins: {
      legend: { display: false },
      tooltip: {
        callbacks: {
          label: (ctx: any) => formatBandwidth(ctx.parsed.y),
        },
      },
    },
  }
  return (
    <div style={styles.chartBox}>
      <div style={styles.chartTitle}>实时带宽趋势</div>
      <div style={{ height: 260 }}>
        {dataPoints.length === 0 ? (
          <div style={styles.placeholder}>暂无数据</div>
        ) : (
          <Line data={data} options={options} />
        )}
      </div>
    </div>
  )
}

// ── ProtocolPieChart ──
function ProtocolPieChart({ protocols }: { protocols: any[] | null }) {
  if (protocols === null) {
    return (
      <div style={styles.chartBox}>
        <div style={styles.chartTitle}>Top 协议分布</div>
        <div style={styles.placeholder}>加载中...</div>
      </div>
    )
  }
  if (protocols.length === 0) {
    return (
      <div style={styles.chartBox}>
        <div style={styles.chartTitle}>Top 协议分布</div>
        <div style={styles.placeholder}>暂无数据</div>
      </div>
    )
  }
  const top5 = protocols.slice(0, 5)
  const data = {
    labels: top5.map((p: any) => p.protocol || p.name || 'Unknown'),
    datasets: [
      {
        data: top5.map((p: any) => p.bytes || p.traffic || p.total_bytes || 0),
        backgroundColor: COLORS.chartColors.slice(0, top5.length),
        borderColor: theme.colors.border,
        borderWidth: 1,
      },
    ],
  }
  const options: any = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { position: 'bottom', labels: { color: COLORS.textDim, font: { size: 12 }, padding: 16 } },
      tooltip: {
        callbacks: {
          label: (ctx: any) => `${ctx.label}: ${formatBytes(ctx.parsed)}`,
        },
      },
    },
  }
  return (
    <div style={styles.chartBox}>
      <div style={styles.chartTitle}>Top 协议分布</div>
      <div style={{ height: 260 }}>
        <Doughnut data={data} options={options} />
      </div>
    </div>
  )
}

// ── TopTalkersTable ──
function TopTalkersTable({ hosts }: { hosts: any[] | null }) {
  return (
    <div style={styles.chartBox}>
      <div style={styles.chartTitle}>Top 流量主机</div>
      {hosts === null ? (
        <div style={styles.placeholder}>加载中...</div>
      ) : hosts.length === 0 ? (
        <div style={styles.placeholder}>暂无数据</div>
      ) : (
        <div style={{ maxHeight: 320, overflowY: 'auto' }}>
          <table style={styles.table}>
            <thead>
              <tr>
                <th style={styles.th}>IP 地址</th>
                <th style={styles.th}>上行流量</th>
                <th style={styles.th}>下行流量</th>
                <th style={styles.th}>连接数</th>
              </tr>
            </thead>
            <tbody>
              {hosts.slice(0, 10).map((h: any, i: number) => (
                <tr key={h.ip || h.host || i}>
                  <td style={styles.td}>{h.ip || h.host || h.address || '-'}</td>
                  <td style={styles.td}>{formatBytes(h.upload || h.bytes_sent || h.tx_bytes || 0)}</td>
                  <td style={styles.td}>{formatBytes(h.download || h.bytes_recv || h.rx_bytes || 0)}</td>
                  <td style={styles.td}>{h.connections || h.flows || h.flow_count || 0}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ── SystemStatus ──
function SystemStatus({ status }: { status: any }) {
  if (!status) {
    return (
      <div style={styles.chartBox}>
        <div style={styles.chartTitle}>系统状态</div>
        <div style={styles.placeholder}>加载中...</div>
      </div>
    )
  }
  const isRunning = !!(status.running || status.is_running || status.capturing)
  const uptime = status.uptime || status.duration || 0
  const iface = status.interface || status.iface || '-'
  const packets = status.packets || status.total_packets || 0

  return (
    <div style={styles.chartBox}>
      <div style={styles.chartTitle}>系统状态</div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 14, padding: '8px 0' }}>
        <div style={styles.statusRow}>
          <div style={dotStyle(isRunning)} />
          <span style={{ fontWeight: 600 }}>{isRunning ? '运行中' : '已停止'}</span>
        </div>
        <div style={styles.statusRow}>
          <span style={{ color: COLORS.textDim }}>运行时间：</span>
          <span>{formatUptime(uptime)}</span>
        </div>
        <div style={styles.statusRow}>
          <span style={{ color: COLORS.textDim }}>网卡：</span>
          <span>{iface}</span>
        </div>
        <div style={styles.statusRow}>
          <span style={{ color: COLORS.textDim }}>已捕获包数：</span>
          <span>{packets.toLocaleString()}</span>
        </div>
      </div>
    </div>
  )
}

// ── WorldMap ──
function WorldMap({ geoHosts }: { geoHosts: any[] }) {
  const validHosts = geoHosts.filter((h: any) => h.latitude && h.longitude)
  const maxBytes = Math.max(...validHosts.map((h: any) => (h.bytes_sent || 0) + (h.bytes_recv || 0)), 1)

  function getRadius(h: any) {
    const total = (h.bytes_sent || 0) + (h.bytes_recv || 0)
    const ratio = total / maxBytes
    return Math.max(3, Math.min(15, 3 + ratio * 12))
  }

  return (
    <div style={{ ...styles.chartBox, padding: 0, overflow: 'hidden' }}>
      <div style={{ padding: '20px 20px 12px 20px' }}>
        <div style={styles.chartTitle}>全球流量分布</div>
      </div>
      <div style={{ height: 350 }}>
        <MapContainer
          center={[20, 0]}
          zoom={2}
          style={{ height: '100%', width: '100%', background: '#0d1117' }}
          scrollWheelZoom={true}
          attributionControl={false}
        >
          <TileLayer url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png" />
          {validHosts.map((h: any, i: number) => (
            <CircleMarker
              key={h.ip || i}
              center={[h.latitude, h.longitude]}
              radius={getRadius(h)}
              pathOptions={{ color: COLORS.highlight, fillColor: COLORS.highlight, fillOpacity: 0.7, weight: 1 }}
            >
              <LeafletTooltip direction="top" opacity={0.95}>
                <div style={{ fontSize: 12, lineHeight: 1.6 }}>
                  <strong>{h.ip}</strong><br />
                  {h.country && <span>{h.city ? `${h.city}, ` : ''}{h.country}<br /></span>}
                  {h.as_org && <span>{h.as_org}<br /></span>}
                  <span>↑ {formatBytes(h.bytes_sent || 0)} / ↓ {formatBytes(h.bytes_recv || 0)}</span>
                </div>
              </LeafletTooltip>
            </CircleMarker>
          ))}
        </MapContainer>
      </div>
    </div>
  )
}

// ── TopCountries ──
function TopCountries({ countries }: { countries: any[] }) {
  if (!countries || countries.length === 0) {
    return (
      <div style={styles.chartBox}>
        <div style={styles.chartTitle}>Top 国家排行</div>
        <div style={styles.placeholder}>暂无数据</div>
      </div>
    )
  }
  const top10 = countries.slice(0, 10)
  const data = {
    labels: top10.map((c: any) => c.country || 'Unknown'),
    datasets: [
      {
        label: '总流量',
        data: top10.map((c: any) => c.total_bytes || 0),
        backgroundColor: COLORS.highlight,
        borderColor: theme.colors.border,
        borderWidth: 1,
        borderRadius: 4,
        barThickness: 18,
      },
    ],
  }
  const options: any = {
    indexAxis: 'y' as const,
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 0 },
    scales: {
      x: {
        ticks: { color: COLORS.textDim, font: { size: 11 }, callback: (v: number) => formatBytes(v) },
        grid: { color: 'rgba(255,255,255,0.05)' },
      },
      y: {
        ticks: { color: COLORS.text, font: { size: 12 } },
        grid: { display: false },
      },
    },
    plugins: {
      legend: { display: false },
      tooltip: { callbacks: { label: (ctx: any) => `${formatBytes(ctx.parsed.x)} | ${top10[ctx.dataIndex]?.host_count || 0} 主机` } },
    },
  }
  return (
    <div style={styles.chartBox}>
      <div style={styles.chartTitle}>Top 国家排行</div>
      <div style={{ height: 300 }}>
        <Bar data={data} options={options} />
      </div>
    </div>
  )
}

// ── TopASN ──
function TopASN({ asnStats }: { asnStats: any[] }) {
  return (
    <div style={styles.chartBox}>
      <div style={styles.chartTitle}>Top ASN 排行</div>
      {!asnStats || asnStats.length === 0 ? (
        <div style={styles.placeholder}>暂无数据</div>
      ) : (
        <div style={{ maxHeight: 320, overflowY: 'auto' }}>
          <table style={styles.table}>
            <thead>
              <tr>
                <th style={styles.th}>ASN</th>
                <th style={styles.th}>组织</th>
                <th style={styles.th}>主机数</th>
                <th style={styles.th}>总流量</th>
              </tr>
            </thead>
            <tbody>
              {asnStats.slice(0, 10).map((a: any, i: number) => (
                <tr key={a.asn || i}>
                  <td style={styles.td}>AS{a.asn}</td>
                  <td style={{ ...styles.td, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{a.as_org || '-'}</td>
                  <td style={styles.td}>{a.host_count || 0}</td>
                  <td style={styles.td}>{formatBytes(a.total_bytes || 0)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ── Dashboard 主组件 ──
const Dashboard = () => {
  const [stats, setStats] = useState<any>(null)
  const [captureStatus, setCaptureStatus] = useState<any>(null)
  const [protocols, setProtocols] = useState<any[] | null>(null)
  const [hosts, setHosts] = useState<any[] | null>(null)
  const [bandwidthHistory, setBandwidthHistory] = useState<{ time: string; value: number }[]>([])
  const [geoHosts, setGeoHosts] = useState<any[]>([])
  const [countryStats, setCountryStats] = useState<any[]>([])
  const [asnStats, setAsnStats] = useState<any[]>([])
  const [alertCount, setAlertCount] = useState(0)

  const wsRef = useRef<WebSocketClient | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const geoPollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchStats = useCallback(async () => {
    try {
      const data = await getSummaryStats()
      setStats(data)
    } catch {
      /* 降级：保留上次数据 */
    }
  }, [])

  const fetchCaptureStatus = useCallback(async () => {
    try {
      const data = await getCaptureStatus()
      setCaptureStatus(data)
    } catch {
      /* 降级 */
    }
  }, [])

  const fetchProtocols = useCallback(async () => {
    try {
      const data = await getProtocolStats(10)
      setProtocols(Array.isArray(data) ? data : data?.protocols || data?.data || [])
    } catch {
      if (protocols === null) setProtocols([])
    }
  }, [])

  const fetchHosts = useCallback(async () => {
    try {
      const data = await getHostStats(10)
      setHosts(Array.isArray(data) ? data : data?.hosts || data?.data || [])
    } catch {
      if (hosts === null) setHosts([])
    }
  }, [])

  const fetchGeoData = useCallback(async () => {
    try {
      const data = await getGeoHosts(100)
      setGeoHosts(Array.isArray(data) ? data : data?.hosts || [])
    } catch { /* ignore */ }
    try {
      const data = await getCountryStats()
      setCountryStats(Array.isArray(data) ? data : data?.countries || [])
    } catch { /* ignore */ }
    try {
      const data = await getASNStats()
      setAsnStats(Array.isArray(data) ? data : data?.asn_stats || [])
    } catch { /* ignore */ }
  }, [])

  // WebSocket stats_update handler
  const handleStatsUpdate = useCallback((payload: any) => {
    if (payload) {
      setStats(payload)
      if (payload.alertCount !== undefined || payload.alert_count !== undefined) {
        setAlertCount(payload.alertCount ?? payload.alert_count ?? 0)
      }
      const bw = payload.total_bandwidth || payload.totalBandwidth || 0
      const now = new Date()
      const timeLabel = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`
      setBandwidthHistory((prev) => {
        const next = [...prev, { time: timeLabel, value: bw }]
        return next.length > 60 ? next.slice(-60) : next
      })
    }
  }, [])

  useEffect(() => {
    // 初始数据加载
    fetchStats()
    fetchCaptureStatus()
    fetchProtocols()
    fetchHosts()
    fetchGeoData()

    // WebSocket 连接
    const ws = new WebSocketClient()
    wsRef.current = ws
    ws.on('stats_update', handleStatsUpdate)
    ws.connect()

    // 轮询协议/主机/状态（每 5 秒）
    pollTimerRef.current = setInterval(() => {
      fetchProtocols()
      fetchHosts()
      fetchCaptureStatus()
    }, 5000)

    // 地理数据轮询（每 30 秒）
    geoPollRef.current = setInterval(() => {
      fetchGeoData()
    }, 30000)

    return () => {
      if (wsRef.current) {
        wsRef.current.off('stats_update', handleStatsUpdate)
        wsRef.current.disconnect()
        wsRef.current = null
      }
      if (pollTimerRef.current) {
        clearInterval(pollTimerRef.current)
        pollTimerRef.current = null
      }
      if (geoPollRef.current) {
        clearInterval(geoPollRef.current)
        geoPollRef.current = null
      }
    }
  }, [])

  return (
    <div style={styles.page}>
      <StatsCards stats={stats} alertCount={alertCount} countryCount={countryStats.length} asnCount={asnStats.length} />
      <div style={styles.chartsRow}>
        <BandwidthChart dataPoints={bandwidthHistory} />
        <ProtocolPieChart protocols={protocols} />
      </div>
      <div style={styles.bottomRow}>
        <TopTalkersTable hosts={hosts} />
        <SystemStatus status={captureStatus} />
      </div>

      {/* ── GeoIP 区域 ── */}
      <div style={{ marginTop: 24 }}>
        <WorldMap geoHosts={geoHosts} />
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginTop: 24 }}>
        <TopCountries countries={countryStats} />
        <TopASN asnStats={asnStats} />
      </div>
    </div>
  )
}

export default Dashboard
