import { useState, useEffect, useRef, useCallback } from 'react'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { Line, Doughnut } from 'react-chartjs-2'
import { useNavigate } from 'react-router-dom'
import { getHostStats, getActiveFlows } from '../api'
import WebSocketClient from '../api/websocket'
import { theme } from '../theme'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const MAX_POINTS = 120

const COLORS = {
  bg: theme.colors.bgPrimary,
  card: theme.colors.bgDeep,
  text: theme.colors.textPrimary,
  accent: theme.colors.border,
  highlight: theme.colors.sentryPurple,
  upload: theme.colors.sentryPurple,
  download: theme.colors.lime,
  border: theme.colors.border,
}

const PROTOCOL_COLORS = theme.colors.protocolColors

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const idx = Math.min(i, units.length - 1)
  return (bytes / Math.pow(k, idx)).toFixed(1) + ' ' + units[idx]
}

function formatBps(bps: number): string {
  return formatBytes(bps) + '/s'
}

const cardStyle: React.CSSProperties = {
  background: COLORS.card,
  borderRadius: 12,
  padding: 16,
  border: `1px solid ${COLORS.border}`,
  boxShadow: theme.shadows.elevated,
}

const Realtime = () => {
  // Bandwidth chart data
  const [uploadData, setUploadData] = useState<number[]>([])
  const [downloadData, setDownloadData] = useState<number[]>([])
  const [timeLabels, setTimeLabels] = useState<string[]>([])

  // Protocol distribution
  const [protocolLabels, setProtocolLabels] = useState<string[]>([])
  const [protocolValues, setProtocolValues] = useState<number[]>([])

  // Host ranking
  const [hosts, setHosts] = useState<any[]>([])
  const [hostsLoading, setHostsLoading] = useState(true)

  // Active flows
  const [flows, setFlows] = useState<any[]>([])
  const [flowsLoading, setFlowsLoading] = useState(true)

  const navigate = useNavigate()

  // Flow count trend
  const [flowCounts, setFlowCounts] = useState<number[]>([])
  const [flowLabels, setFlowLabels] = useState<string[]>([])

  const wsRef = useRef<WebSocketClient | null>(null)

  const getNowLabel = useCallback(() => {
    const d = new Date()
    return d.getHours().toString().padStart(2, '0') + ':' +
      d.getMinutes().toString().padStart(2, '0') + ':' +
      d.getSeconds().toString().padStart(2, '0')
  }, [])

  // WebSocket setup
  useEffect(() => {
    const ws = new WebSocketClient()
    wsRef.current = ws

    const handleStatsUpdate = (payload: any) => {
      const now = getNowLabel()

      // Bandwidth
      const up = payload.bytes_sent_rate || payload.upload_bps || 0
      const down = payload.bytes_recv_rate || payload.download_bps || 0
      setUploadData(prev => [...prev, up].slice(-MAX_POINTS))
      setDownloadData(prev => [...prev, down].slice(-MAX_POINTS))
      setTimeLabels(prev => [...prev, now].slice(-MAX_POINTS))

      // Protocol distribution
      if (payload.protocol_stats && typeof payload.protocol_stats === 'object') {
        const entries = Object.entries(payload.protocol_stats) as [string, number][]
        entries.sort((a, b) => b[1] - a[1])
        const top = entries.slice(0, 10)
        setProtocolLabels(top.map(e => e[0]))
        setProtocolValues(top.map(e => e[1]))
      }

      // Flow count
      const flowCount = payload.active_flows || payload.flow_count || 0
      setFlowCounts(prev => [...prev, flowCount].slice(-MAX_POINTS))
      setFlowLabels(prev => [...prev, now].slice(-MAX_POINTS))
    }

    ws.on('stats_update', handleStatsUpdate)
    ws.connect()

    return () => {
      ws.off('stats_update', handleStatsUpdate)
      ws.disconnect()
    }
  }, [getNowLabel])

  // Host ranking polling
  useEffect(() => {
    let active = true
    const fetchHosts = async () => {
      try {
        const data = await getHostStats(10)
        if (active) {
          const list = Array.isArray(data) ? data : (data?.hosts || data?.data || [])
          setHosts(list.slice(0, 10))
          setHostsLoading(false)
        }
      } catch {
        if (active) setHostsLoading(false)
      }
    }
    fetchHosts()
    const timer = setInterval(fetchHosts, 3000)
    return () => { active = false; clearInterval(timer) }
  }, [])

  // Active flows polling
  useEffect(() => {
    let active = true
    const fetchFlows = async () => {
      try {
        const data = await getActiveFlows(20)
        if (active) {
          const list = Array.isArray(data) ? data : (data?.flows || data?.data || [])
          setFlows(list)
          setFlowsLoading(false)
        }
      } catch {
        if (active) setFlowsLoading(false)
      }
    }
    fetchFlows()
    const timer = setInterval(fetchFlows, 3000)
    return () => { active = false; clearInterval(timer) }
  }, [])

  const bandwidthChartData = {
    labels: timeLabels,
    datasets: [
      {
        label: '上行带宽',
        data: uploadData,
        borderColor: COLORS.upload,
        backgroundColor: 'rgba(106,95,193,0.1)',
        fill: true,
        tension: 0.3,
        pointRadius: 0,
        borderWidth: 2,
      },
      {
        label: '下行带宽',
        data: downloadData,
        borderColor: COLORS.download,
        backgroundColor: 'rgba(194,239,78,0.1)',
        fill: true,
        tension: 0.3,
        pointRadius: 0,
        borderWidth: 2,
      },
    ],
  }

  const bandwidthOptions: any = {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 0 },
    plugins: {
      legend: { labels: { color: COLORS.text } },
      title: { display: true, text: '实时带宽', color: COLORS.text },
    },
    scales: {
      x: {
        ticks: { color: COLORS.text, maxTicksLimit: 10 },
        grid: { color: 'rgba(255,255,255,0.05)' },
      },
      y: {
        ticks: {
          color: COLORS.text,
          callback: (v: number) => formatBps(v),
        },
        grid: { color: 'rgba(255,255,255,0.05)' },
      },
    },
  }

  const doughnutData = {
    labels: protocolLabels.length > 0 ? protocolLabels : ['暂无数据'],
    datasets: [{
      data: protocolValues.length > 0 ? protocolValues : [1],
      backgroundColor: protocolValues.length > 0 ? PROTOCOL_COLORS : ['#333'],
      borderColor: COLORS.card,
      borderWidth: 2,
    }],
  }

  const doughnutOptions: any = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        position: 'bottom' as const,
        labels: { color: COLORS.text, boxWidth: 12, padding: 8, font: { size: 11 } },
      },
      title: { display: true, text: '协议分布', color: COLORS.text },
      tooltip: {
        callbacks: {
          label: (ctx: any) => `${ctx.label}: ${formatBytes(ctx.parsed)}`,
        },
      },
    },
  }

  const flowChartData = {
    labels: flowLabels,
    datasets: [{
      label: '活跃 Flow 数',
      data: flowCounts,
      borderColor: COLORS.highlight,
      backgroundColor: 'rgba(106,95,193,0.1)',
      fill: true,
      tension: 0.3,
      pointRadius: 0,
      borderWidth: 2,
    }],
  }

  const flowOptions: any = {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 0 },
    plugins: {
      legend: { labels: { color: COLORS.text } },
      title: { display: true, text: '活跃 Flow 趋势', color: COLORS.text },
    },
    scales: {
      x: {
        ticks: { color: COLORS.text, maxTicksLimit: 10 },
        grid: { color: 'rgba(255,255,255,0.05)' },
      },
      y: {
        ticks: { color: COLORS.text },
        grid: { color: 'rgba(255,255,255,0.05)' },
        beginAtZero: true,
      },
    },
  }

  return (
    <div style={{ padding: 20, background: COLORS.bg, minHeight: '100vh', color: COLORS.text, fontFamily: theme.typography.fontFamily }}>
      <h1 style={{ margin: '0 0 16px', fontSize: 22 }}>实时监控</h1>

      <div style={{
        display: 'grid',
        gridTemplateColumns: '2fr 1fr',
        gridTemplateRows: '1fr 1fr auto',
        gap: 16,
      }}>
        {/* 带宽图 - 左上 */}
        <div style={cardStyle}>
          {timeLabels.length === 0
            ? <p style={{ textAlign: 'center', paddingTop: 60, opacity: 0.5 }}>等待数据中...</p>
            : <Line data={bandwidthChartData} options={bandwidthOptions} />}
        </div>

        {/* 协议分布 - 右上 */}
        <div style={cardStyle}>
          <Doughnut data={doughnutData} options={doughnutOptions} />
        </div>

        {/* 主机排行 - 左下 */}
        <div style={{ ...cardStyle, overflow: 'auto' }}>
          <h3 style={{ margin: '0 0 10px', fontSize: 14, color: COLORS.highlight }}>
            主机流量排行 Top 10
          </h3>
          {hostsLoading ? (
            <p style={{ opacity: 0.5 }}>加载中...</p>
          ) : hosts.length === 0 ? (
            <p style={{ opacity: 0.5 }}>暂无数据</p>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: `1px solid ${COLORS.border}` }}>
                  {['排名', 'IP 地址', '带宽', '趋势'].map(h => (
                    <th key={h} style={{ padding: '6px 8px', textAlign: 'left', color: COLORS.highlight, fontWeight: 600 }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {hosts.map((host: any, i: number) => {
                  const ip = host.ip || host.host || host.address || '-'
                  const bw = host.bytes_total || host.bandwidth || host.traffic || 0
                  const trend = host.trend || (bw > 0 ? '▲' : '—')
                  return (
                    <tr key={ip + i} style={{ borderBottom: `1px solid rgba(255,255,255,0.05)` }}>
                      <td style={{ padding: '5px 8px' }}>{i + 1}</td>
                      <td style={{ padding: '5px 8px', fontFamily: 'monospace' }}>{ip}</td>
                      <td style={{ padding: '5px 8px' }}>{formatBytes(bw)}</td>
                      <td style={{ padding: '5px 8px', color: bw > 0 ? COLORS.download : '#888' }}>{trend}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>

        {/* Flow 趋势 - 右下 */}
        <div style={cardStyle}>
          {flowLabels.length === 0
            ? <p style={{ textAlign: 'center', paddingTop: 60, opacity: 0.5 }}>等待数据中...</p>
            : <Line data={flowChartData} options={flowOptions} />}
        </div>

        {/* 活跃 Flow 列表 - 底部跨两列 */}
        <div style={{ ...cardStyle, gridColumn: '1 / -1', maxHeight: 320, overflow: 'auto' }}>
          <h3 style={{ margin: '0 0 10px', fontSize: 14, color: COLORS.highlight }}>
            活跃 Flow 列表
          </h3>
          {flowsLoading ? (
            <p style={{ opacity: 0.5 }}>加载中...</p>
          ) : flows.length === 0 ? (
            <p style={{ opacity: 0.5 }}>暂无活跃 Flow</p>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: `1px solid ${COLORS.border}` }}>
                  {['源地址', '目标地址', '协议', 'L7 协议', '发送', '接收', '状态'].map(h => (
                    <th key={h} style={{ padding: '6px 8px', textAlign: 'left', color: COLORS.highlight, fontWeight: 600 }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {flows.map((f: any, i: number) => {
                  const flowId = f.flow_id || f.id || ''
                  const srcIp = f.src_ip || '-'
                  const dstIp = f.dst_ip || '-'
                  const srcPort = f.src_port ?? ''
                  const dstPort = f.dst_port ?? ''
                  const proto = f.protocol || '-'
                  const l7 = f.l7_protocol || '-'
                  const sent = f.bytes_sent || 0
                  const recv = f.bytes_recv || 0
                  const active = f.is_active !== false
                  return (
                    <tr
                      key={flowId || i}
                      onClick={() => flowId && navigate(`/flow/${encodeURIComponent(flowId)}`)}
                      style={{
                        borderBottom: '1px solid rgba(255,255,255,0.05)',
                        cursor: flowId ? 'pointer' : 'default',
                      }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLElement).style.background = 'rgba(106,95,193,0.06)' }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = 'transparent' }}
                    >
                      <td style={{ padding: '5px 8px', fontFamily: 'monospace' }}>{srcIp}{srcPort ? `:${srcPort}` : ''}</td>
                      <td style={{ padding: '5px 8px', fontFamily: 'monospace' }}>{dstIp}{dstPort ? `:${dstPort}` : ''}</td>
                      <td style={{ padding: '5px 8px' }}>{proto}</td>
                      <td style={{ padding: '5px 8px' }}>{l7}</td>
                      <td style={{ padding: '5px 8px' }}>{formatBytes(sent)}</td>
                      <td style={{ padding: '5px 8px' }}>{formatBytes(recv)}</td>
                      <td style={{ padding: '5px 8px' }}>
                        <span style={{
                          color: active ? '#4caf50' : '#a0a0a0',
                          fontSize: 12,
                        }}>
                          {active ? '● Active' : '● Closed'}
                        </span>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}

export default Realtime
