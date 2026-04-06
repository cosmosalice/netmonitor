import { useState, useEffect, useCallback } from 'react'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { Line, Doughnut, Bar } from 'react-chartjs-2'
import { useNavigate } from 'react-router-dom'
import {
  getHistoricalTraffic,
  getHistoricalHosts,
  getHistoricalProtocols,
  getHistoricalCompare,
  getHistoricalFlows,
  exportFlows,
  exportTimeseries,
  downloadBlob,
} from '../api'
import { theme } from '../theme'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, BarElement, ArcElement, Title, Tooltip, Legend, Filler)

// ── 工具函数 ──
function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const idx = Math.min(i, units.length - 1)
  return (bytes / Math.pow(1024, idx)).toFixed(2) + ' ' + units[idx]
}

function formatBps(bytes: number): string {
  if (bytes <= 0) return '0 B/s'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const idx = Math.min(i, units.length - 1)
  return (bytes / Math.pow(1024, idx)).toFixed(2) + ' ' + units[idx]
}

function formatTime(ts: string): string {
  if (!ts) return '-'
  const d = new Date(ts)
  return d.toLocaleString('zh-CN')
}

function formatShortTime(ts: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  return `${String(d.getMonth() + 1).padStart(2, '0')}/${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function nowUnix(): number {
  return Math.floor(Date.now() / 1000)
}

// ── 样式 ──
const styles: Record<string, React.CSSProperties> = {
  page: {
    padding: 24,
    background: theme.colors.bgPrimary,
    minHeight: '100%',
    color: theme.colors.textPrimary,
    fontFamily: theme.typography.fontFamily,
  },
  title: {
    fontSize: 22,
    fontWeight: 700,
    color: theme.colors.textPrimary,
    margin: '0 0 20px',
  },
  card: {
    background: theme.colors.bgCard,
    borderRadius: theme.radii.lg,
    padding: 20,
    border: `1px solid ${theme.colors.border}`,
    marginBottom: 16,
    boxShadow: theme.shadows.elevated,
  },
  cardTitle: {
    fontSize: 15,
    fontWeight: 600,
    marginBottom: 12,
    color: theme.colors.textPrimary,
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  badge: {
    fontSize: 11,
    padding: '2px 8px',
    borderRadius: theme.radii.xs,
    background: theme.colors.bgDeep,
    color: theme.colors.sentryPurple,
    fontWeight: 500,
  },
  tabBar: {
    display: 'flex',
    gap: 0,
    marginBottom: 20,
    borderBottom: `1px solid ${theme.colors.border}`,
  },
  tab: {
    padding: '10px 20px',
    cursor: 'pointer',
    fontSize: 14,
    fontWeight: 500,
    color: theme.colors.textDim,
    borderBottom: '2px solid transparent',
    transition: 'all 0.2s',
  },
  tabActive: {
    padding: '10px 20px',
    cursor: 'pointer',
    fontSize: 14,
    fontWeight: 600,
    color: theme.colors.sentryPurple,
    borderBottom: `2px solid ${theme.colors.sentryPurple}`,
  },
  presetBar: {
    display: 'flex',
    gap: 8,
    alignItems: 'center',
    flexWrap: 'wrap' as const,
    marginBottom: 16,
  },
  presetBtn: {
    padding: '6px 14px',
    borderRadius: theme.radii.xs,
    border: `1px solid ${theme.colors.border}`,
    background: 'transparent',
    color: theme.colors.textSecondary,
    cursor: 'pointer',
    fontSize: 13,
    transition: 'all 0.2s',
  },
  presetBtnActive: {
    padding: '6px 14px',
    borderRadius: theme.radii.xs,
    border: `1px solid ${theme.colors.sentryPurple}`,
    background: theme.colors.glassDeep,
    color: theme.colors.sentryPurple,
    cursor: 'pointer',
    fontSize: 13,
    fontWeight: 600,
  },
  inputRow: {
    display: 'flex',
    gap: 12,
    alignItems: 'center',
    flexWrap: 'wrap' as const,
  },
  input: {
    background: theme.colors.bgDeep,
    border: `1px solid ${theme.colors.border}`,
    borderRadius: theme.radii.xs,
    padding: '6px 10px',
    color: theme.colors.textPrimary,
    fontSize: 13,
    outline: 'none',
    fontFamily: theme.typography.fontFamily,
  },
  selectInput: {
    background: theme.colors.bgDeep,
    border: `1px solid ${theme.colors.border}`,
    borderRadius: theme.radii.xs,
    padding: '6px 10px',
    color: theme.colors.textPrimary,
    fontSize: 13,
    outline: 'none',
    minWidth: 100,
    fontFamily: theme.typography.fontFamily,
  },
  applyBtn: {
    padding: '6px 16px',
    borderRadius: theme.radii.xs,
    border: 'none',
    background: theme.colors.sentryPurple,
    color: '#fff',
    cursor: 'pointer',
    fontSize: 13,
    fontWeight: 600,
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
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
    color: theme.colors.sentryPurple,
    fontWeight: 600,
    cursor: 'pointer',
  },
  td: {
    padding: '10px 12px',
    borderBottom: '1px solid rgba(255,255,255,0.05)',
  },
  placeholder: {
    color: theme.colors.textDim,
    textAlign: 'center' as const,
    padding: 40,
  },
  gridRow: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: 16,
    marginBottom: 16,
  },
  pager: {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    gap: 12,
    marginTop: 12,
  },
  pageBtn: {
    padding: '5px 14px',
    borderRadius: theme.radii.xs,
    border: `1px solid ${theme.colors.border}`,
    background: 'transparent',
    color: theme.colors.textSecondary,
    cursor: 'pointer',
    fontSize: 13,
  },
  pageBtnDisabled: {
    padding: '5px 14px',
    borderRadius: theme.radii.xs,
    border: `1px solid ${theme.colors.border}`,
    background: 'transparent',
    color: theme.colors.textDim,
    cursor: 'not-allowed',
    fontSize: 13,
    opacity: 0.5,
  },
  filterRow: {
    display: 'flex',
    gap: 10,
    alignItems: 'center',
    flexWrap: 'wrap' as const,
    marginBottom: 12,
  },
}

// ── Presets ──
const TIME_PRESETS = [
  { label: '最近 1 小时', seconds: 3600 },
  { label: '最近 6 小时', seconds: 6 * 3600 },
  { label: '最近 24 小时', seconds: 24 * 3600 },
  { label: '最近 7 天', seconds: 7 * 86400 },
  { label: '最近 30 天', seconds: 30 * 86400 },
]

// ── 主组件 ──
const History = () => {
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<'overview' | 'compare' | 'flows'>('overview')

  // Time range
  const [activePreset, setActivePreset] = useState(2) // default 24h
  const [startTime, setStartTime] = useState(() => nowUnix() - 24 * 3600)
  const [endTime, setEndTime] = useState(() => nowUnix())
  const [customStart, setCustomStart] = useState('')
  const [customEnd, setCustomEnd] = useState('')

  // Data states
  const [trafficData, setTrafficData] = useState<any>(null)
  const [trafficLoading, setTrafficLoading] = useState(false)
  const [hostsData, setHostsData] = useState<any[]>([])
  const [hostsLoading, setHostsLoading] = useState(false)
  const [hostSort, setHostSort] = useState<'total' | 'sent' | 'recv'>('total')
  const [protocolsData, setProtocolsData] = useState<any[]>([])
  const [protocolsLoading, setProtocolsLoading] = useState(false)

  // Compare states
  const [p1Start, setP1Start] = useState('')
  const [p1End, setP1End] = useState('')
  const [p2Start, setP2Start] = useState('')
  const [p2End, setP2End] = useState('')
  const [compareData, setCompareData] = useState<any>(null)
  const [compareLoading, setCompareLoading] = useState(false)

  // Flow query states
  const [flowSrcIp, setFlowSrcIp] = useState('')
  const [flowDstIp, setFlowDstIp] = useState('')
  const [flowProtocol, setFlowProtocol] = useState('')
  const [flowsData, setFlowsData] = useState<any[]>([])
  const [flowsTotal, setFlowsTotal] = useState(0)
  const [flowsLoading, setFlowsLoading] = useState(false)
  const [flowPage, setFlowPage] = useState(0)
  const FLOW_PAGE_SIZE = 50

  // ── 时间范围切换 ──
  const selectPreset = useCallback((idx: number) => {
    const end = nowUnix()
    const start = end - TIME_PRESETS[idx].seconds
    setActivePreset(idx)
    setStartTime(start)
    setEndTime(end)
    setCustomStart('')
    setCustomEnd('')
  }, [])

  const applyCustomRange = useCallback(() => {
    if (!customStart || !customEnd) return
    const s = Math.floor(new Date(customStart).getTime() / 1000)
    const e = Math.floor(new Date(customEnd).getTime() / 1000)
    if (s >= e) return
    setActivePreset(-1)
    setStartTime(s)
    setEndTime(e)
  }, [customStart, customEnd])

  // ── 数据获取 ──
  const fetchTraffic = useCallback(async () => {
    setTrafficLoading(true)
    try {
      const data = await getHistoricalTraffic(startTime, endTime)
      setTrafficData(data)
    } catch { setTrafficData(null) }
    setTrafficLoading(false)
  }, [startTime, endTime])

  const fetchHosts = useCallback(async () => {
    setHostsLoading(true)
    try {
      const data = await getHistoricalHosts(startTime, endTime, 20, hostSort)
      setHostsData(Array.isArray(data) ? data : data?.hosts || [])
    } catch { setHostsData([]) }
    setHostsLoading(false)
  }, [startTime, endTime, hostSort])

  const fetchProtocols = useCallback(async () => {
    setProtocolsLoading(true)
    try {
      const data = await getHistoricalProtocols(startTime, endTime)
      setProtocolsData(Array.isArray(data) ? data : data?.protocols || [])
    } catch { setProtocolsData([]) }
    setProtocolsLoading(false)
  }, [startTime, endTime])

  const fetchFlows = useCallback(async (page: number) => {
    setFlowsLoading(true)
    try {
      const params: any = { start: startTime, end: endTime, limit: FLOW_PAGE_SIZE, offset: page * FLOW_PAGE_SIZE }
      if (flowSrcIp) params.src_ip = flowSrcIp
      if (flowDstIp) params.dst_ip = flowDstIp
      if (flowProtocol) params.l7_protocol = flowProtocol
      const data = await getHistoricalFlows(params)
      setFlowsData(Array.isArray(data) ? data : data?.flows || [])
      setFlowsTotal(data?.total || 0)
    } catch { setFlowsData([]); setFlowsTotal(0) }
    setFlowsLoading(false)
  }, [startTime, endTime, flowSrcIp, flowDstIp, flowProtocol])

  const fetchCompare = useCallback(async () => {
    if (!p1Start || !p1End || !p2Start || !p2End) return
    setCompareLoading(true)
    try {
      const s1 = Math.floor(new Date(p1Start).getTime() / 1000)
      const e1 = Math.floor(new Date(p1End).getTime() / 1000)
      const s2 = Math.floor(new Date(p2Start).getTime() / 1000)
      const e2 = Math.floor(new Date(p2End).getTime() / 1000)
      const data = await getHistoricalCompare(s1, e1, s2, e2)
      setCompareData(data)
    } catch { setCompareData(null) }
    setCompareLoading(false)
  }, [p1Start, p1End, p2Start, p2End])

  // Auto-fetch on time range change for overview tab
  useEffect(() => {
    if (activeTab === 'overview') {
      fetchTraffic()
      fetchHosts()
      fetchProtocols()
    }
  }, [startTime, endTime, activeTab, fetchTraffic, fetchHosts, fetchProtocols])

  // Re-fetch hosts when sort changes
  useEffect(() => {
    if (activeTab === 'overview') fetchHosts()
  }, [hostSort])

  // Fetch flows when switching to flows tab or page changes
  useEffect(() => {
    if (activeTab === 'flows') fetchFlows(flowPage)
  }, [activeTab, flowPage])

  // ── Chart configs ──
  const chartGridColor = 'rgba(255,255,255,0.05)'

  const trafficChartData = () => {
    const points = trafficData?.data || []
    return {
      labels: points.map((p: any) => formatShortTime(p.timestamp)),
      datasets: [
        {
          label: '平均值',
          data: points.map((p: any) => p.avg_value || 0),
          borderColor: theme.colors.sentryPurple,
          backgroundColor: 'rgba(106,95,193,0.08)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 2,
        },
        {
          label: '最大值',
          data: points.map((p: any) => p.max_value || 0),
          borderColor: theme.colors.pink,
          backgroundColor: 'transparent',
          fill: false,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 1.5,
          borderDash: [4, 3],
        },
        {
          label: '最小值',
          data: points.map((p: any) => p.min_value || 0),
          borderColor: theme.colors.lime,
          backgroundColor: 'transparent',
          fill: false,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 1.5,
          borderDash: [4, 3],
        },
      ],
    }
  }

  const trafficChartOptions: any = {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 300 },
    scales: {
      x: {
        ticks: { color: theme.colors.textDim, maxTicksLimit: 12, font: { size: 11, family: theme.typography.fontFamily } },
        grid: { color: chartGridColor },
      },
      y: {
        ticks: {
          color: theme.colors.textDim,
          font: { size: 11, family: theme.typography.fontFamily },
          callback: (v: number) => formatBps(v),
        },
        grid: { color: chartGridColor },
      },
    },
    plugins: {
      legend: { labels: { color: theme.colors.textPrimary, font: { size: 12, family: theme.typography.fontFamily }, padding: 16 } },
      tooltip: {
        callbacks: {
          label: (ctx: any) => `${ctx.dataset.label}: ${formatBps(ctx.parsed.y)}`,
        },
      },
    },
  }

  // ── Top Hosts bar chart (top 10) ──
  const hostsBarData = () => {
    const top10 = hostsData.slice(0, 10).reverse()
    return {
      labels: top10.map((h: any) => h.ip || h.hostname || '-'),
      datasets: [{
        label: '总流量',
        data: top10.map((h: any) => h.total_bytes || 0),
        backgroundColor: 'rgba(106,95,193,0.7)',
        borderColor: theme.colors.sentryPurple,
        borderWidth: 1,
        borderRadius: 4,
      }],
    }
  }

  const hostsBarOptions: any = {
    responsive: true,
    maintainAspectRatio: false,
    indexAxis: 'y' as const,
    animation: { duration: 300 },
    scales: {
      x: {
        ticks: {
          color: theme.colors.textDim,
          font: { size: 11, family: theme.typography.fontFamily },
          callback: (v: number) => formatBytes(v),
        },
        grid: { color: chartGridColor },
      },
      y: {
        ticks: { color: theme.colors.textPrimary, font: { size: 11, family: theme.typography.fontFamily } },
        grid: { display: false },
      },
    },
    plugins: {
      legend: { display: false },
      tooltip: {
        callbacks: {
          label: (ctx: any) => formatBytes(ctx.parsed.x),
        },
      },
    },
  }

  // ── Protocol doughnut ──
  const protocolDoughnutData = () => {
    const top8 = protocolsData.slice(0, 8)
    return {
      labels: top8.map((p: any) => p.protocol || 'Unknown'),
      datasets: [{
        data: top8.map((p: any) => p.total_bytes || 0),
        backgroundColor: theme.colors.chartColors.slice(0, top8.length),
        borderWidth: 0,
      }],
    }
  }

  const protocolDoughnutOptions: any = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { position: 'bottom', labels: { color: theme.colors.textDim, font: { size: 12, family: theme.typography.fontFamily }, padding: 12 } },
      tooltip: {
        callbacks: {
          label: (ctx: any) => `${ctx.label}: ${formatBytes(ctx.parsed)}`,
        },
      },
    },
  }

  // ── Compare chart ──
  const compareChartData = () => {
    const p1 = compareData?.period1?.data || []
    const p2 = compareData?.period2?.data || []
    const maxLen = Math.max(p1.length, p2.length)
    const labels = Array.from({ length: maxLen }, (_, i) => `#${i + 1}`)
    return {
      labels,
      datasets: [
        {
          label: '时间段 1',
          data: p1.map((d: any) => d.avg_value || d.value || 0),
          borderColor: theme.colors.sentryPurple,
          backgroundColor: 'rgba(106,95,193,0.08)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 2,
        },
        {
          label: '时间段 2',
          data: p2.map((d: any) => d.avg_value || d.value || 0),
          borderColor: theme.colors.pink,
          backgroundColor: 'rgba(250,127,170,0.08)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 2,
        },
      ],
    }
  }

  const compareChartOptions: any = {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 300 },
    scales: {
      x: {
        ticks: { color: theme.colors.textDim, maxTicksLimit: 12, font: { size: 11, family: theme.typography.fontFamily } },
        grid: { color: chartGridColor },
      },
      y: {
        ticks: {
          color: theme.colors.textDim,
          font: { size: 11, family: theme.typography.fontFamily },
          callback: (v: number) => formatBps(v),
        },
        grid: { color: chartGridColor },
      },
    },
    plugins: {
      legend: { labels: { color: theme.colors.textPrimary, font: { size: 12, family: theme.typography.fontFamily }, padding: 16 } },
      tooltip: {
        callbacks: {
          label: (ctx: any) => `${ctx.dataset.label}: ${formatBps(ctx.parsed.y)}`,
        },
      },
    },
  }

  const totalPages = Math.ceil(flowsTotal / FLOW_PAGE_SIZE)

  // ── Render ──
  return (
    <div style={styles.page}>
      <h1 style={styles.title}>历史分析</h1>

      {/* Tab Bar */}
      <div style={styles.tabBar}>
        {([
          ['overview', '总览'],
          ['compare', '时间段对比'],
          ['flows', '历史 Flow'],
        ] as const).map(([key, label]) => (
          <div
            key={key}
            style={activeTab === key ? styles.tabActive : styles.tab}
            onClick={() => setActiveTab(key)}
          >
            {label}
          </div>
        ))}
      </div>

      {/* ============ OVERVIEW TAB ============ */}
      {activeTab === 'overview' && (
        <>
          {/* Time Selector */}
          <div style={styles.card}>
            <div style={styles.presetBar}>
              {TIME_PRESETS.map((p, i) => (
                <button
                  key={i}
                  style={activePreset === i ? styles.presetBtnActive : styles.presetBtn}
                  onClick={() => selectPreset(i)}
                >
                  {p.label}
                </button>
              ))}
              <span style={{ color: theme.colors.textDim, margin: '0 6px' }}>|</span>
              <input
                type="datetime-local"
                style={styles.input}
                value={customStart}
                onChange={(e) => setCustomStart(e.target.value)}
                placeholder="开始时间"
              />
              <span style={{ color: theme.colors.textDim }}>至</span>
              <input
                type="datetime-local"
                style={styles.input}
                value={customEnd}
                onChange={(e) => setCustomEnd(e.target.value)}
                placeholder="结束时间"
              />
              <button style={styles.applyBtn} onClick={applyCustomRange}>应用</button>
            </div>
          </div>

          {/* Traffic Trend */}
          <div style={styles.card}>
            <div style={{ ...styles.cardTitle, justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span>历史流量趋势</span>
                {trafficData?.granularity && (
                  <span style={styles.badge}>粒度: {trafficData.granularity}</span>
                )}
              </div>
              <button
                style={{ background: 'transparent', border: `1px solid ${theme.colors.sentryPurple}`, color: theme.colors.sentryPurple, padding: '6px 12px', borderRadius: theme.radii.xs, cursor: 'pointer', fontSize: '13px', textTransform: 'uppercase', letterSpacing: '0.2px' }}
                onMouseEnter={e => (e.currentTarget.style.background = theme.colors.glassDeep)}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                onClick={async () => {
                  try {
                    const res = await exportTimeseries({ format: 'csv', type: 'bandwidth', start: startTime, end: endTime })
                    downloadBlob(res.data, 'timeseries_export.csv')
                  } catch (err) { console.error('Export failed:', err) }
                }}
              >
                📥 导出时序数据 CSV
              </button>
            </div>
            <div style={{ height: 300 }}>
              {trafficLoading ? (
                <div style={styles.placeholder}>加载中...</div>
              ) : !trafficData?.data?.length ? (
                <div style={styles.placeholder}>暂无数据</div>
              ) : (
                <Line data={trafficChartData()} options={trafficChartOptions} />
              )}
            </div>
          </div>

          {/* Top Hosts */}
          <div style={styles.gridRow}>
            <div style={styles.card}>
              <div style={styles.cardTitle}>Top 10 主机流量</div>
              <div style={{ height: 340 }}>
                {hostsLoading ? (
                  <div style={styles.placeholder}>加载中...</div>
                ) : hostsData.length === 0 ? (
                  <div style={styles.placeholder}>暂无数据</div>
                ) : (
                  <Bar data={hostsBarData()} options={hostsBarOptions} />
                )}
              </div>
            </div>
            <div style={styles.card}>
              <div style={styles.cardTitle}>
                <span>主机排行</span>
                <div style={{ marginLeft: 'auto', display: 'flex', gap: 4 }}>
                  {(['total', 'sent', 'recv'] as const).map((s) => (
                    <button
                      key={s}
                      style={hostSort === s ? styles.presetBtnActive : styles.presetBtn}
                      onClick={() => setHostSort(s)}
                    >
                      {s === 'total' ? '总流量' : s === 'sent' ? '发送' : '接收'}
                    </button>
                  ))}
                </div>
              </div>
              <div style={{ maxHeight: 340, overflowY: 'auto' }}>
                {hostsLoading ? (
                  <div style={styles.placeholder}>加载中...</div>
                ) : hostsData.length === 0 ? (
                  <div style={styles.placeholder}>暂无数据</div>
                ) : (
                  <table style={styles.table}>
                    <thead>
                      <tr>
                        <th style={styles.th}>IP</th>
                        <th style={styles.th}>主机名</th>
                        <th style={styles.th}>发送</th>
                        <th style={styles.th}>接收</th>
                        <th style={styles.th}>总流量</th>
                        <th style={styles.th}>流数</th>
                      </tr>
                    </thead>
                    <tbody>
                      {hostsData.map((h: any, i: number) => (
                        <tr key={h.ip || i}>
                          <td style={{ ...styles.td, fontFamily: 'monospace' }}>{h.ip || '-'}</td>
                          <td style={styles.td}>{h.hostname || '-'}</td>
                          <td style={styles.td}>{formatBytes(h.bytes_sent || 0)}</td>
                          <td style={styles.td}>{formatBytes(h.bytes_recv || 0)}</td>
                          <td style={{ ...styles.td, color: theme.colors.sentryPurple, fontWeight: 600 }}>{formatBytes(h.total_bytes || 0)}</td>
                          <td style={styles.td}>{h.flow_count || 0}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          </div>

          {/* Protocol Distribution */}
          <div style={styles.gridRow}>
            <div style={styles.card}>
              <div style={styles.cardTitle}>协议分布</div>
              <div style={{ height: 300 }}>
                {protocolsLoading ? (
                  <div style={styles.placeholder}>加载中...</div>
                ) : protocolsData.length === 0 ? (
                  <div style={styles.placeholder}>暂无数据</div>
                ) : (
                  <Doughnut data={protocolDoughnutData()} options={protocolDoughnutOptions} />
                )}
              </div>
            </div>
            <div style={styles.card}>
              <div style={styles.cardTitle}>协议详情</div>
              <div style={{ maxHeight: 300, overflowY: 'auto' }}>
                {protocolsLoading ? (
                  <div style={styles.placeholder}>加载中...</div>
                ) : protocolsData.length === 0 ? (
                  <div style={styles.placeholder}>暂无数据</div>
                ) : (
                  <table style={styles.table}>
                    <thead>
                      <tr>
                        <th style={styles.th}>协议</th>
                        <th style={styles.th}>分类</th>
                        <th style={styles.th}>流量</th>
                        <th style={styles.th}>流数</th>
                        <th style={styles.th}>占比</th>
                      </tr>
                    </thead>
                    <tbody>
                      {protocolsData.map((p: any, i: number) => (
                        <tr key={p.protocol || i}>
                          <td style={{ ...styles.td, fontWeight: 600 }}>{p.protocol || '-'}</td>
                          <td style={styles.td}>{p.category || '-'}</td>
                          <td style={styles.td}>{formatBytes(p.total_bytes || 0)}</td>
                          <td style={styles.td}>{p.flow_count || 0}</td>
                          <td style={{ ...styles.td, color: theme.colors.sentryPurple }}>{(p.percentage || 0).toFixed(1)}%</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          </div>
        </>
      )}

      {/* ============ COMPARE TAB ============ */}
      {activeTab === 'compare' && (
        <div style={styles.card}>
          <div style={styles.cardTitle}>时间段对比</div>
          <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap', marginBottom: 16 }}>
            <div>
              <div style={{ color: theme.colors.textDim, fontSize: 13, marginBottom: 6 }}>时间段 1</div>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <input type="datetime-local" style={styles.input} value={p1Start} onChange={e => setP1Start(e.target.value)} />
                <span style={{ color: theme.colors.textDim }}>至</span>
                <input type="datetime-local" style={styles.input} value={p1End} onChange={e => setP1End(e.target.value)} />
              </div>
            </div>
            <div>
              <div style={{ color: theme.colors.textDim, fontSize: 13, marginBottom: 6 }}>时间段 2</div>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <input type="datetime-local" style={styles.input} value={p2Start} onChange={e => setP2Start(e.target.value)} />
                <span style={{ color: theme.colors.textDim }}>至</span>
                <input type="datetime-local" style={styles.input} value={p2End} onChange={e => setP2End(e.target.value)} />
              </div>
            </div>
            <div style={{ display: 'flex', alignItems: 'flex-end' }}>
              <button style={styles.applyBtn} onClick={fetchCompare}>开始对比</button>
            </div>
          </div>
          <div style={{ height: 350 }}>
            {compareLoading ? (
              <div style={styles.placeholder}>加载中...</div>
            ) : !compareData ? (
              <div style={styles.placeholder}>选择两个时间段后点击"开始对比"</div>
            ) : (
              <Line data={compareChartData()} options={compareChartOptions} />
            )}
          </div>
        </div>
      )}

      {/* ============ FLOWS TAB ============ */}
      {activeTab === 'flows' && (
        <div style={styles.card}>
          <div style={styles.cardTitle}>
            <span>历史 Flow 查询</span>
            <button
              style={{ background: 'transparent', border: `1px solid ${theme.colors.sentryPurple}`, color: theme.colors.sentryPurple, padding: '6px 12px', borderRadius: theme.radii.xs, cursor: 'pointer', fontSize: '13px', marginLeft: 'auto', textTransform: 'uppercase', letterSpacing: '0.2px' }}
              onMouseEnter={e => (e.currentTarget.style.background = theme.colors.glassDeep)}
              onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
              onClick={async () => {
                try {
                  const params: any = { format: 'csv', start: startTime, end: endTime }
                  if (flowSrcIp) params.src_ip = flowSrcIp
                  if (flowDstIp) params.dst_ip = flowDstIp
                  if (flowProtocol) params.protocol = flowProtocol
                  const res = await exportFlows(params)
                  downloadBlob(res.data, 'flows_export.csv')
                } catch (err) { console.error('Export failed:', err) }
              }}
            >
              📥 导出 Flow CSV
            </button>
          </div>
          {/* Filters */}
          <div style={styles.filterRow}>
            <input
              style={styles.input}
              placeholder="源 IP"
              value={flowSrcIp}
              onChange={e => setFlowSrcIp(e.target.value)}
            />
            <input
              style={styles.input}
              placeholder="目标 IP"
              value={flowDstIp}
              onChange={e => setFlowDstIp(e.target.value)}
            />
            <input
              style={styles.input}
              placeholder="L7 协议"
              value={flowProtocol}
              onChange={e => setFlowProtocol(e.target.value)}
            />
            <button style={styles.applyBtn} onClick={() => { setFlowPage(0); fetchFlows(0) }}>查询</button>
            <span style={{ color: theme.colors.textDim, fontSize: 12, marginLeft: 8 }}>
              共 {flowsTotal} 条记录
            </span>
          </div>
          {/* Table */}
          <div style={{ overflowX: 'auto' }}>
            {flowsLoading ? (
              <div style={styles.placeholder}>加载中...</div>
            ) : flowsData.length === 0 ? (
              <div style={styles.placeholder}>暂无数据，请选择时间范围后查询</div>
            ) : (
              <table style={styles.table}>
                <thead>
                  <tr>
                    <th style={styles.th}>Flow ID</th>
                    <th style={styles.th}>源地址</th>
                    <th style={styles.th}>目标地址</th>
                    <th style={styles.th}>协议</th>
                    <th style={styles.th}>L7 协议</th>
                    <th style={styles.th}>发送</th>
                    <th style={styles.th}>接收</th>
                    <th style={styles.th}>开始时间</th>
                    <th style={styles.th}>结束时间</th>
                  </tr>
                </thead>
                <tbody>
                  {flowsData.map((f: any, i: number) => {
                    const fid = f.flow_id || f.id || ''
                    return (
                      <tr
                        key={fid || i}
                        onClick={() => fid && navigate(`/flow/${encodeURIComponent(fid)}`)}
                        style={{ cursor: fid ? 'pointer' : 'default' }}
                        onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = 'rgba(106,95,193,0.06)' }}
                        onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = 'transparent' }}
                      >
                        <td style={{ ...styles.td, fontFamily: 'monospace', fontSize: 11, maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{fid || '-'}</td>
                        <td style={{ ...styles.td, fontFamily: 'monospace' }}>{(f.src_ip || '-')}{f.src_port ? `:${f.src_port}` : ''}</td>
                        <td style={{ ...styles.td, fontFamily: 'monospace' }}>{(f.dst_ip || '-')}{f.dst_port ? `:${f.dst_port}` : ''}</td>
                        <td style={styles.td}>{f.protocol || '-'}</td>
                        <td style={styles.td}>{f.l7_protocol || '-'}</td>
                        <td style={styles.td}>{formatBytes(f.bytes_sent || 0)}</td>
                        <td style={styles.td}>{formatBytes(f.bytes_recv || 0)}</td>
                        <td style={{ ...styles.td, fontSize: 12 }}>{formatTime(f.start_time || f.started_at || '')}</td>
                        <td style={{ ...styles.td, fontSize: 12 }}>{formatTime(f.end_time || f.ended_at || '')}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )}
          </div>
          {/* Pagination */}
          {totalPages > 1 && (
            <div style={styles.pager}>
              <button
                style={flowPage <= 0 ? styles.pageBtnDisabled : styles.pageBtn}
                disabled={flowPage <= 0}
                onClick={() => setFlowPage(p => Math.max(0, p - 1))}
              >
                上一页
              </button>
              <span style={{ color: theme.colors.textDim, fontSize: 13 }}>
                第 {flowPage + 1} / {totalPages} 页
              </span>
              <button
                style={flowPage >= totalPages - 1 ? styles.pageBtnDisabled : styles.pageBtn}
                disabled={flowPage >= totalPages - 1}
                onClick={() => setFlowPage(p => p + 1)}
              >
                下一页
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export default History
