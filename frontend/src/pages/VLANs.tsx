import { useState, useEffect, useCallback } from 'react'
import {
  Chart, CategoryScale, LinearScale, BarElement, ArcElement, Title, Tooltip, Legend,
} from 'chart.js'
import { Doughnut, Bar } from 'react-chartjs-2'
import {
  getVLANs, getVLANHosts, getVLANFlows,
} from '../api/index'
import { theme } from '../theme'

Chart.register(CategoryScale, LinearScale, BarElement, ArcElement, Title, Tooltip, Legend)

// ── Theme colors ──
const C = {
  bg: theme.colors.bgPrimary, card: theme.colors.bgDeep, border: theme.colors.border, accent: theme.colors.sentryPurple,
  text: theme.colors.textPrimary, textW: '#ffffff', textDim: theme.colors.textDim,
}

const CHART_COLORS = [
  theme.colors.sentryPurple, '#e94560', theme.colors.warning, theme.colors.success,
  '#9c27b0', '#ff5722', '#2196f3', '#795548', '#607d8b', theme.colors.border,
]

// ── Styles ──
const S: Record<string, React.CSSProperties> = {
  page: { padding: 24, background: C.bg, minHeight: '100%', color: C.text, fontFamily: theme.typography.fontFamily },
  title: { fontSize: 22, fontWeight: 700, color: C.textW, marginBottom: 8 },
  subtitle: { fontSize: 14, color: C.textDim, marginBottom: 24 },
  row: { display: 'flex', gap: 16, marginBottom: 16, flexWrap: 'wrap' as const },
  card: { background: C.card, borderRadius: 12, padding: 20, border: `1px solid ${C.border}`, flex: 1, minWidth: 200 },
  cardTitle: { fontSize: 14, fontWeight: 600, color: C.textDim, marginBottom: 8 },
  cardValue: { fontSize: 28, fontWeight: 700, color: C.textW },
  section: { background: C.card, borderRadius: 12, padding: 20, border: `1px solid ${C.border}`, marginBottom: 16 },
  sectionTitle: { fontSize: 16, fontWeight: 700, color: C.textW, marginBottom: 16, display: 'flex', alignItems: 'center', gap: 8 },
  table: { width: '100%', borderCollapse: 'collapse' as const, fontSize: 13 },
  th: { textAlign: 'left' as const, padding: '10px 12px', borderBottom: `1px solid ${C.border}`, color: C.textDim, fontWeight: 600, cursor: 'pointer' },
  td: { padding: '10px 12px', borderBottom: `1px solid ${C.border}22` },
  tag: { display: 'inline-block', background: C.border, color: C.accent, borderRadius: 4, padding: '2px 6px', fontSize: 11, marginRight: 4, marginBottom: 2 },
  expandRow: { background: `${C.border}22` },
  expandCell: { padding: '16px 12px', borderBottom: `1px solid ${C.border}44` },
  chartWrap: { maxWidth: 320, margin: '0 auto' },
  expandIcon: { transition: 'transform 0.2s', fontSize: 12 },
  clickableRow: { cursor: 'pointer' },
  subTable: { width: '100%', borderCollapse: 'collapse' as const, fontSize: 12, background: C.bg, borderRadius: 8, overflow: 'hidden' },
  subTh: { textAlign: 'left' as const, padding: '8px 10px', background: theme.colors.borderLight, color: C.textDim, fontWeight: 600 },
  subTd: { padding: '8px 10px', borderBottom: `1px solid ${C.border}22` },
}

// ── Interfaces matching backend ──
interface VLANInfo {
  vlan_id: number
  host_count: number
  bytes: number
  flows: number
  hosts?: string[]
}

interface VLANHost {
  ip: string
  bytes_sent: number
  bytes_recv: number
  flows: number
}

interface VLANFlow {
  flow_id: string
  src_ip: string
  dst_ip: string
  src_port: number
  dst_port: number
  protocol: string
  bytes_sent: number
  bytes_recv: number
  l7_protocol: string
}

// ── Helpers ──
function fmtBytes(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

function fmtNum(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n ?? 0)
}

const VLANs: React.FC = () => {
  const [vlans, setVLANs] = useState<VLANInfo[]>([])
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const [hosts, setHosts] = useState<VLANHost[]>([])
  const [flows, setFlows] = useState<VLANFlow[]>([])
  const [loading, setLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)

  const fetchVLANs = useCallback(async () => {
    try {
      const res = await getVLANs()
      setVLANs(res?.vlans || [])
    } catch (e) {
      console.error('VLAN fetch error', e)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchVLANs()
    const timer = setInterval(fetchVLANs, 5000)
    return () => clearInterval(timer)
  }, [fetchVLANs])

  const fetchDetails = async (vlanId: number) => {
    setDetailLoading(true)
    try {
      const [hostsRes, flowsRes] = await Promise.all([
        getVLANHosts(vlanId),
        getVLANFlows(vlanId, 50),
      ])
      setHosts(hostsRes?.hosts || [])
      setFlows(flowsRes?.flows || [])
    } catch (e) {
      console.error('VLAN detail fetch error', e)
    } finally {
      setDetailLoading(false)
    }
  }

  const toggleExpand = (vlanId: number) => {
    if (expandedId === vlanId) {
      setExpandedId(null)
    } else {
      setExpandedId(vlanId)
      fetchDetails(vlanId)
    }
  }

  // ── Summary Stats ──
  const totalVLANs = vlans.length
  const totalHosts = vlans.reduce((sum, v) => sum + v.host_count, 0)
  const totalBytes = vlans.reduce((sum, v) => sum + v.bytes, 0)
  const totalFlows = vlans.reduce((sum, v) => sum + v.flows, 0)

  // ── Chart data ──
  const chartLabels = vlans.map(v => `VLAN ${v.vlan_id}`)
  const chartValues = vlans.map(v => v.bytes)
  const doughnutData = {
    labels: chartLabels,
    datasets: [{
      data: chartValues,
      backgroundColor: chartLabels.map((_, i) => CHART_COLORS[i % CHART_COLORS.length]),
      borderWidth: 0,
    }],
  }

  const barData = {
    labels: chartLabels,
    datasets: [{
      label: '流量',
      data: chartValues,
      backgroundColor: C.accent,
      borderRadius: 4,
    }],
  }

  const doughnutOpts: any = {
    responsive: true,
    plugins: {
      legend: { position: 'bottom' as const, labels: { color: C.textDim, boxWidth: 12, padding: 12, font: { size: 11 } } },
      tooltip: {
        backgroundColor: '#000',
        titleColor: '#fff',
        bodyColor: '#fff',
        callbacks: {
          label: (ctx: any) => `${ctx.label}: ${fmtBytes(ctx.raw)}`,
        },
      },
    },
    cutout: '60%',
  }

  const barOpts: any = {
    responsive: true,
    plugins: {
      legend: { display: false },
      tooltip: {
        backgroundColor: '#000',
        titleColor: '#fff',
        bodyColor: '#fff',
        callbacks: {
          label: (ctx: any) => fmtBytes(ctx.raw),
        },
      },
    },
    scales: {
      x: { ticks: { color: C.textDim }, grid: { color: `${C.border}44` } },
      y: { ticks: { color: C.textDim }, grid: { display: false } },
    },
  }

  return (
    <div style={S.page}>
      <div style={S.title}>VLAN 分析</div>
      <div style={S.subtitle}>虚拟局域网流量统计与分析（每 5 秒自动刷新）</div>

      {/* ── Summary Cards ── */}
      <div style={S.row}>
        <div style={S.card}>
          <div style={S.cardTitle}>活跃 VLAN 数</div>
          <div style={S.cardValue}>{totalVLANs}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardTitle}>总主机数</div>
          <div style={S.cardValue}>{fmtNum(totalHosts)}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardTitle}>总流量</div>
          <div style={S.cardValue}>{fmtBytes(totalBytes)}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardTitle}>活跃流数</div>
          <div style={S.cardValue}>{fmtNum(totalFlows)}</div>
        </div>
      </div>

      {/* ── Charts ── */}
      {vlans.length > 0 && (
        <div style={{ ...S.row, alignItems: 'stretch' }}>
          <div style={{ ...S.section, flex: 1 }}>
            <div style={S.sectionTitle}>VLAN 流量分布</div>
            <div style={S.chartWrap}>
              <Doughnut data={doughnutData} options={doughnutOpts} />
            </div>
          </div>
          <div style={{ ...S.section, flex: 2 }}>
            <div style={S.sectionTitle}>VLAN 流量柱状图</div>
            <Bar data={barData} options={barOpts} height={Math.max(150, vlans.length * 30 + 60)} />
          </div>
        </div>
      )}

      {/* ── VLAN Table ── */}
      <div style={S.section}>
        <div style={S.sectionTitle}>VLAN 列表</div>
        <div style={{ overflowX: 'auto' }}>
          <table style={S.table}>
            <thead>
              <tr>
                <th style={{ ...S.th, width: 40 }}></th>
                <th style={S.th}>VLAN ID</th>
                <th style={S.th}>主机数</th>
                <th style={S.th}>流量</th>
                <th style={S.th}>活跃流数</th>
              </tr>
            </thead>
            <tbody>
              {vlans.map((vlan, i) => (
                <>
                  <tr
                    key={vlan.vlan_id}
                    style={{
                      ...S.clickableRow,
                      background: i % 2 === 0 ? 'transparent' : `${C.border}22`,
                    }}
                    onClick={() => toggleExpand(vlan.vlan_id)}
                  >
                    <td style={S.td}>
                      <span style={{
                        ...S.expandIcon,
                        display: 'inline-block',
                        transform: expandedId === vlan.vlan_id ? 'rotate(90deg)' : 'rotate(0deg)',
                      }}>▶</span>
                    </td>
                    <td style={{ ...S.td, color: C.accent, fontWeight: 700 }}>VLAN {vlan.vlan_id}</td>
                    <td style={{ ...S.td, fontWeight: 600 }}>{vlan.host_count}</td>
                    <td style={{ ...S.td, fontWeight: 600 }}>{fmtBytes(vlan.bytes)}</td>
                    <td style={S.td}>{vlan.flows}</td>
                  </tr>
                  {/* ── Expanded Detail Row ── */}
                  {expandedId === vlan.vlan_id && (
                    <tr key={`${vlan.vlan_id}-detail`} style={S.expandRow}>
                      <td colSpan={5} style={S.expandCell}>
                        {detailLoading ? (
                          <div style={{ textAlign: 'center', padding: 20, color: C.textDim }}>加载中...</div>
                        ) : (
                          <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap' }}>
                            {/* ── Hosts Table ── */}
                            <div style={{ flex: 1, minWidth: 300 }}>
                              <div style={{ fontSize: 14, fontWeight: 600, color: C.textW, marginBottom: 12 }}>
                                主机列表 ({hosts.length})
                              </div>
                              <table style={S.subTable}>
                                <thead>
                                  <tr>
                                    <th style={S.subTh}>IP 地址</th>
                                    <th style={S.subTh}>发送</th>
                                    <th style={S.subTh}>接收</th>
                                    <th style={S.subTh}>流数</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {hosts.map((h, idx) => (
                                    <tr key={h.ip} style={{ background: idx % 2 === 0 ? 'transparent' : `${C.border}11` }}>
                                      <td style={{ ...S.subTd, color: C.accent, fontWeight: 600 }}>{h.ip}</td>
                                      <td style={S.subTd}>{fmtBytes(h.bytes_sent)}</td>
                                      <td style={S.subTd}>{fmtBytes(h.bytes_recv)}</td>
                                      <td style={S.subTd}>{h.flows}</td>
                                    </tr>
                                  ))}
                                  {hosts.length === 0 && (
                                    <tr><td colSpan={4} style={{ ...S.subTd, textAlign: 'center', color: C.textDim }}>暂无主机数据</td></tr>
                                  )}
                                </tbody>
                              </table>
                            </div>
                            {/* ── Flows Table ── */}
                            <div style={{ flex: 2, minWidth: 400 }}>
                              <div style={{ fontSize: 14, fontWeight: 600, color: C.textW, marginBottom: 12 }}>
                                流量列表 ({flows.length})
                              </div>
                              <table style={S.subTable}>
                                <thead>
                                  <tr>
                                    <th style={S.subTh}>源 IP:端口</th>
                                    <th style={S.subTh}>目标 IP:端口</th>
                                    <th style={S.subTh}>协议</th>
                                    <th style={S.subTh}>L7</th>
                                    <th style={S.subTh}>发送</th>
                                    <th style={S.subTh}>接收</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {flows.map((f, idx) => (
                                    <tr key={f.flow_id} style={{ background: idx % 2 === 0 ? 'transparent' : `${C.border}11` }}>
                                      <td style={{ ...S.subTd, fontSize: 11 }}>
                                        <span style={{ color: C.accent }}>{f.src_ip}</span>:{f.src_port}
                                      </td>
                                      <td style={{ ...S.subTd, fontSize: 11 }}>
                                        <span style={{ color: C.accent }}>{f.dst_ip}</span>:{f.dst_port}
                                      </td>
                                      <td style={S.subTd}><span style={S.tag}>{f.protocol}</span></td>
                                      <td style={S.subTd}><span style={{ ...S.tag, background: theme.colors.border }}>{f.l7_protocol || '-'}</span></td>
                                      <td style={S.subTd}>{fmtBytes(f.bytes_sent)}</td>
                                      <td style={S.subTd}>{fmtBytes(f.bytes_recv)}</td>
                                    </tr>
                                  ))}
                                  {flows.length === 0 && (
                                    <tr><td colSpan={6} style={{ ...S.subTd, textAlign: 'center', color: C.textDim }}>暂无流量数据</td></tr>
                                  )}
                                </tbody>
                              </table>
                            </div>
                          </div>
                        )}
                      </td>
                    </tr>
                  )}
                </>
              ))}
              {loading && (
                <tr><td colSpan={5} style={{ ...S.td, textAlign: 'center', color: C.textDim, padding: 40 }}>加载中...</td></tr>
              )}
              {!loading && vlans.length === 0 && (
                <tr><td colSpan={5} style={{ ...S.td, textAlign: 'center', color: C.textDim, padding: 40 }}>暂无 VLAN 数据，请先开始抓包</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

export default VLANs
