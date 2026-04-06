import { useState, useEffect } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { getFlowDetail } from '../api'
import { theme } from '../theme'

// ── 工具函数 ──
function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const idx = Math.min(i, units.length - 1)
  return (bytes / Math.pow(1024, idx)).toFixed(2) + ' ' + units[idx]
}

function formatTime(timeStr: string): string {
  if (!timeStr) return '-'
  return new Date(timeStr).toLocaleString('zh-CN')
}

function formatDuration(start: string, end: string): string {
  if (!start) return '-'
  const s = new Date(start).getTime()
  const e = end ? new Date(end).getTime() : Date.now()
  const diffSec = Math.floor((e - s) / 1000)
  if (diffSec < 60) return `${diffSec} 秒`
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分 ${diffSec % 60} 秒`
  const h = Math.floor(diffSec / 3600)
  const m = Math.floor((diffSec % 3600) / 60)
  return `${h} 小时 ${m} 分`
}

function rttColor(ms: number): string {
  if (ms < 50) return theme.colors.success
  if (ms <= 200) return theme.colors.warning
  return theme.colors.error
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
  breadcrumb: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    marginBottom: 20,
    fontSize: 14,
    color: theme.colors.textDim,
  },
  breadcrumbLink: {
    color: theme.colors.sentryPurple,
    textDecoration: 'none',
    cursor: 'pointer',
  },
  card: {
    background: theme.colors.bgCard,
    borderRadius: theme.radii.sm,
    padding: 20,
    border: `1px solid ${theme.colors.border}`,
    marginBottom: 16,
    boxShadow: theme.shadows.elevated,
  },
  cardTitle: {
    fontSize: 15,
    fontWeight: 600,
    color: theme.colors.textPrimary,
    marginBottom: 16,
    paddingBottom: 10,
    borderBottom: `1px solid ${theme.colors.border}`,
  },
  grid2: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: 16,
  },
  infoRow: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '8px 0',
    borderBottom: '1px solid rgba(255,255,255,0.05)',
  },
  infoLabel: {
    color: theme.colors.textDim,
    fontSize: 13,
  },
  infoValue: {
    color: theme.colors.textPrimary,
    fontSize: 13,
    fontFamily: theme.typography.fontFamilyMono,
  },
  metricCard: {
    background: theme.colors.bgDeep,
    borderRadius: theme.radii.sm,
    padding: '14px 16px',
    textAlign: 'center' as const,
  },
  metricLabel: {
    fontSize: 12,
    color: theme.colors.textDim,
    marginBottom: 6,
  },
  metricValue: {
    fontSize: 22,
    fontWeight: 700,
  },
  statusBadge: {
    display: 'inline-block',
    padding: '3px 10px',
    borderRadius: theme.radii.lg,
    fontSize: 12,
    fontWeight: 600,
  },
  loading: {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    height: '60vh',
    color: theme.colors.textDim,
    fontSize: 16,
  },
  error: {
    display: 'flex',
    flexDirection: 'column' as const,
    justifyContent: 'center',
    alignItems: 'center',
    height: '60vh',
    gap: 16,
  },
  hostLink: {
    color: theme.colors.sentryPurple,
    textDecoration: 'none',
    fontFamily: theme.typography.fontFamilyMono,
    fontSize: 13,
  },
}

// ── 类型 ──
interface TcpMetrics {
  rtt_ms: number
  min_rtt_ms: number
  max_rtt_ms: number
  avg_rtt_ms: number
  retransmissions: number
  out_of_order: number
  packet_loss: number
  avg_window_size: number
  max_window_size: number
}

interface FlowData {
  flow_id: string
  src_ip: string
  dst_ip: string
  src_port: number
  dst_port: number
  protocol: string
  l7_protocol: string
  l7_category: string
  bytes_sent: number
  bytes_recv: number
  packets_sent: number
  packets_recv: number
  start_time: string
  end_time: string
  is_active: boolean
  tcp_metrics?: TcpMetrics
}

// ── 组件 ──
const FlowDetail = () => {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [flow, setFlow] = useState<FlowData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    let active = true
    const fetchFlow = async () => {
      try {
        setLoading(true)
        setError(null)
        const data = await getFlowDetail(decodeURIComponent(id))
        if (active) setFlow(data)
      } catch (err: any) {
        if (active) {
          setError(err?.response?.data?.error || err?.message || '获取 Flow 详情失败')
        }
      } finally {
        if (active) setLoading(false)
      }
    }
    fetchFlow()
    return () => { active = false }
  }, [id])

  if (loading) {
    return (
      <div style={styles.page}>
        <div style={styles.loading}>
          <span>⏳ 加载 Flow 详情中...</span>
        </div>
      </div>
    )
  }

  if (error || !flow) {
    return (
      <div style={styles.page}>
        <div style={styles.error}>
          <span style={{ fontSize: 48 }}>⚠️</span>
          <span style={{ color: theme.colors.error, fontSize: 16 }}>{error || 'Flow 数据不存在'}</span>
          <button
            onClick={() => navigate(-1)}
            style={{
              background: theme.colors.sentryPurple,
              color: '#fff',
              border: 'none',
              borderRadius: theme.radii.xs,
              padding: '8px 20px',
              cursor: 'pointer',
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.2px',
            }}
          >
            返回上一页
          </button>
        </div>
      </div>
    )
  }

  const isTcp = flow.protocol?.toUpperCase() === 'TCP'
  const tcp = flow.tcp_metrics

  return (
    <div style={styles.page}>
      {/* 面包屑导航 */}
      <div style={styles.breadcrumb}>
        <Link to="/realtime" style={styles.breadcrumbLink}>实时监控</Link>
        <span>{'>'}</span>
        <span style={{ color: theme.colors.textPrimary }}>Flow 详情</span>
      </div>

      {/* 基本信息卡片 */}
      <div style={styles.card}>
        <div style={styles.cardTitle}>基本信息</div>

        {/* 五元组 */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 16,
          padding: '12px 0 20px',
          fontSize: 16,
          fontFamily: 'monospace',
        }}>
          <span style={{ color: theme.colors.sentryPurple }}>{flow.src_ip}:{flow.src_port}</span>
          <span style={{ color: theme.colors.textDim, fontSize: 20 }}>→</span>
          <span style={{ color: theme.colors.sentryPurple }}>{flow.dst_ip}:{flow.dst_port}</span>
        </div>

        <div style={styles.grid2}>
          <div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>L4 协议</span>
              <span style={styles.infoValue}>{flow.protocol || '-'}</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>L7 协议</span>
              <span style={styles.infoValue}>{flow.l7_protocol || '-'}</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>L7 分类</span>
              <span style={styles.infoValue}>{flow.l7_category || '-'}</span>
            </div>
          </div>
          <div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>状态</span>
              <span style={{
                ...styles.statusBadge,
                background: flow.is_active ? 'rgba(76,175,80,0.15)' : 'rgba(160,160,160,0.15)',
                color: flow.is_active ? theme.colors.success : theme.colors.textDim,
              }}>
                {flow.is_active ? '● Active' : '● Closed'}
              </span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>开始时间</span>
              <span style={styles.infoValue}>{formatTime(flow.start_time)}</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>结束时间</span>
              <span style={styles.infoValue}>{flow.end_time ? formatTime(flow.end_time) : '进行中'}</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>持续时长</span>
              <span style={styles.infoValue}>{formatDuration(flow.start_time, flow.end_time)}</span>
            </div>
          </div>
        </div>
      </div>

      {/* 流量统计 */}
      <div style={styles.card}>
        <div style={styles.cardTitle}>流量统计</div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16 }}>
          <div style={styles.metricCard}>
            <div style={styles.metricLabel}>发送</div>
            <div style={{ ...styles.metricValue, color: '#4fc3f7' }}>
              {formatBytes(flow.bytes_sent)}
            </div>
            <div style={{ fontSize: 12, color: theme.colors.textDim, marginTop: 4 }}>
              {flow.packets_sent.toLocaleString()} 包
            </div>
          </div>
          <div style={styles.metricCard}>
            <div style={styles.metricLabel}>接收</div>
            <div style={{ ...styles.metricValue, color: '#66bb6a' }}>
              {formatBytes(flow.bytes_recv)}
            </div>
            <div style={{ fontSize: 12, color: theme.colors.textDim, marginTop: 4 }}>
              {flow.packets_recv.toLocaleString()} 包
            </div>
          </div>
          <div style={styles.metricCard}>
            <div style={styles.metricLabel}>总流量</div>
            <div style={{ ...styles.metricValue, color: theme.colors.sentryPurple }}>
              {formatBytes(flow.bytes_sent + flow.bytes_recv)}
            </div>
            <div style={{ fontSize: 12, color: theme.colors.textDim, marginTop: 4 }}>
              {(flow.packets_sent + flow.packets_recv).toLocaleString()} 包
            </div>
          </div>
        </div>
      </div>

      {/* TCP 性能指标 */}
      <div style={styles.card}>
        <div style={styles.cardTitle}>TCP 性能指标</div>
        {!isTcp ? (
          <div style={{ textAlign: 'center', padding: 24, color: theme.colors.textDim }}>
            非 TCP 协议，无 TCP 指标
          </div>
        ) : !tcp ? (
          <div style={{ textAlign: 'center', padding: 24, color: theme.colors.textDim }}>
            暂无 TCP 指标数据
          </div>
        ) : (
          <>
            {/* RTT 指标 */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 16 }}>
              <div style={styles.metricCard}>
                <div style={styles.metricLabel}>当前 RTT</div>
                <div style={{ ...styles.metricValue, color: rttColor(tcp.rtt_ms), fontSize: 20 }}>
                  {tcp.rtt_ms.toFixed(1)} ms
                </div>
              </div>
              <div style={styles.metricCard}>
                <div style={styles.metricLabel}>最小 RTT</div>
                <div style={{ ...styles.metricValue, color: rttColor(tcp.min_rtt_ms), fontSize: 20 }}>
                  {tcp.min_rtt_ms.toFixed(1)} ms
                </div>
              </div>
              <div style={styles.metricCard}>
                <div style={styles.metricLabel}>最大 RTT</div>
                <div style={{ ...styles.metricValue, color: rttColor(tcp.max_rtt_ms), fontSize: 20 }}>
                  {tcp.max_rtt_ms.toFixed(1)} ms
                </div>
              </div>
              <div style={styles.metricCard}>
                <div style={styles.metricLabel}>平均 RTT</div>
                <div style={{ ...styles.metricValue, color: rttColor(tcp.avg_rtt_ms), fontSize: 20 }}>
                  {tcp.avg_rtt_ms.toFixed(1)} ms
                </div>
              </div>
            </div>

            {/* 其他 TCP 指标 */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 12 }}>
              <div style={styles.metricCard}>
                <div style={styles.metricLabel}>重传次数</div>
                <div style={{
                  ...styles.metricValue,
                  fontSize: 20,
                  color: tcp.retransmissions > 0 ? theme.colors.error : theme.colors.success,
                }}>
                  {tcp.retransmissions}
                </div>
              </div>
              <div style={styles.metricCard}>
                <div style={styles.metricLabel}>乱序包数</div>
                <div style={{
                  ...styles.metricValue,
                  fontSize: 20,
                  color: tcp.out_of_order > 0 ? theme.colors.warning : theme.colors.success,
                }}>
                  {tcp.out_of_order}
                </div>
              </div>
              <div style={styles.metricCard}>
                <div style={styles.metricLabel}>丢包数</div>
                <div style={{
                  ...styles.metricValue,
                  fontSize: 20,
                  color: tcp.packet_loss > 0 ? theme.colors.error : theme.colors.success,
                }}>
                  {tcp.packet_loss}
                </div>
              </div>
              <div style={styles.metricCard}>
                <div style={styles.metricLabel}>平均窗口</div>
                <div style={{ ...styles.metricValue, fontSize: 20, color: theme.colors.textPrimary }}>
                  {formatBytes(tcp.avg_window_size)}
                </div>
              </div>
              <div style={styles.metricCard}>
                <div style={styles.metricLabel}>最大窗口</div>
                <div style={{ ...styles.metricValue, fontSize: 20, color: theme.colors.textPrimary }}>
                  {formatBytes(tcp.max_window_size)}
                </div>
              </div>
            </div>
          </>
        )}
      </div>

      {/* 关联信息 */}
      <div style={styles.card}>
        <div style={styles.cardTitle}>关联信息</div>
        <div style={styles.grid2}>
          <div style={styles.infoRow}>
            <span style={styles.infoLabel}>源主机</span>
            <Link to={`/hosts?ip=${encodeURIComponent(flow.src_ip)}`} style={styles.hostLink}>
              {flow.src_ip} ↗
            </Link>
          </div>
          <div style={styles.infoRow}>
            <span style={styles.infoLabel}>目标主机</span>
            <Link to={`/hosts?ip=${encodeURIComponent(flow.dst_ip)}`} style={styles.hostLink}>
              {flow.dst_ip} ↗
            </Link>
          </div>
        </div>
      </div>
    </div>
  )
}

export default FlowDetail
