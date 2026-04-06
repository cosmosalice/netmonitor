import { useState, useEffect, useCallback } from 'react'
import { getHostStats, getHostRisks, exportHosts, downloadBlob } from '../api/index'
import { theme } from '../theme'

interface HostProtocols {
  [protocol: string]: number
}

interface RiskScore {
  ip: string
  score: number
  level: 'low' | 'medium' | 'high' | 'critical'
  factors: { name: string; score: number; description: string }[]
  last_updated: string
}

interface HostData {
  ip: string
  mac: string
  hostname: string
  bytes_sent: number
  bytes_recv: number
  packets_sent: number
  packets_recv: number
  active_flows: number
  protocols: HostProtocols
  first_seen: string
  last_seen: string
}

type SortField = 'total' | 'connections' | 'last_seen' | 'risk'

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const idx = Math.min(i, units.length - 1)
  return `${(bytes / Math.pow(k, idx)).toFixed(2)} ${units[idx]}`
}

const formatTime = (timeStr: string): string => {
  if (!timeStr) return '-'
  const d = new Date(timeStr)
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString('zh-CN', {
    month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

// ── Styles ──────────────────────────────────────────────────────────────────
const colors = {
  bg: theme.colors.bgPrimary,
  card: theme.colors.bgDeep,
  cardHover: '#1a2845',
  text: theme.colors.textPrimary,
  textDim: theme.colors.textDim,
  accent: theme.colors.sentryPurple,
  accentDark: theme.colors.border,
  border: theme.colors.borderLight,
  success: theme.colors.success,
  danger: theme.colors.error,
  rowSelected: theme.colors.border,
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
    marginBottom: '20px',
    color: colors.accent,
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
  },
  toolbar: {
    display: 'flex',
    gap: '12px',
    marginBottom: '16px',
    flexWrap: 'wrap' as const,
    alignItems: 'center',
  },
  searchInput: {
    flex: '1 1 260px',
    padding: '10px 14px',
    borderRadius: '8px',
    border: `1px solid ${colors.border}`,
    backgroundColor: colors.card,
    color: colors.text,
    fontSize: '14px',
    outline: 'none',
    minWidth: '200px',
  },
  select: {
    padding: '10px 14px',
    borderRadius: '8px',
    border: `1px solid ${colors.border}`,
    backgroundColor: colors.card,
    color: colors.text,
    fontSize: '14px',
    outline: 'none',
    cursor: 'pointer',
  },
  tableWrap: {
    overflowX: 'auto' as const,
    borderRadius: '10px',
    border: `1px solid ${colors.border}`,
    backgroundColor: colors.card,
  },
  table: {
    width: '100%',
    borderCollapse: 'collapse' as const,
    fontSize: '14px',
  },
  th: {
    padding: '12px 16px',
    textAlign: 'left' as const,
    borderBottom: `2px solid ${colors.border}`,
    color: colors.accent,
    fontWeight: 600,
    whiteSpace: 'nowrap' as const,
    userSelect: 'none' as const,
  },
  td: {
    padding: '10px 16px',
    borderBottom: `1px solid ${colors.border}`,
    whiteSpace: 'nowrap' as const,
  },
  detailPanel: {
    marginTop: '16px',
    borderRadius: '10px',
    border: `1px solid ${colors.border}`,
    backgroundColor: colors.card,
    padding: '20px',
  },
  detailTitle: {
    fontSize: '16px',
    fontWeight: 600,
    color: colors.accent,
    marginBottom: '16px',
  },
  detailGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
    gap: '12px',
    marginBottom: '20px',
  },
  statCard: {
    padding: '12px 16px',
    borderRadius: '8px',
    backgroundColor: colors.accentDark,
    textAlign: 'center' as const,
  },
  statLabel: {
    fontSize: '12px',
    color: colors.textDim,
    marginBottom: '4px',
  },
  statValue: {
    fontSize: '18px',
    fontWeight: 700,
    color: colors.accent,
  },
  protoTable: {
    width: '100%',
    borderCollapse: 'collapse' as const,
    fontSize: '13px',
  },
  protoBar: {
    height: '6px',
    borderRadius: '3px',
    backgroundColor: colors.accent,
    transition: 'width 0.3s',
  },
  loading: {
    textAlign: 'center' as const,
    padding: '60px 0',
    color: colors.textDim,
    fontSize: '15px',
  },
  emptyRow: {
    textAlign: 'center' as const,
    padding: '40px',
    color: colors.textDim,
  },
  badge: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: '10px',
    fontSize: '12px',
    backgroundColor: colors.accentDark,
    color: colors.accent,
  },
}

const riskColors: Record<string, string> = {
  low: theme.colors.success,
  medium: theme.colors.warning,
  high: '#ff9100',
  critical: theme.colors.error,
}

const riskLabels: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '严重',
}

// ── Component ───────────────────────────────────────────────────────────────

const Hosts = () => {
  const [hosts, setHosts] = useState<HostData[]>([])
  const [riskMap, setRiskMap] = useState<Record<string, RiskScore>>({})
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [sortField, setSortField] = useState<SortField>('total')
  const [selectedIp, setSelectedIp] = useState<string | null>(null)

  const fetchHosts = useCallback(async () => {
    try {
      const [hostData, riskData] = await Promise.all([
        getHostStats(200),
        getHostRisks({ limit: 200 }).catch(() => ({ hosts: [] })),
      ])
      setHosts(hostData.hosts || [])
      const rm: Record<string, RiskScore> = {}
      if (riskData.hosts) {
        for (const r of riskData.hosts) {
          rm[r.ip] = r
        }
      }
      setRiskMap(rm)
    } catch (err) {
      console.error('Failed to fetch hosts:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchHosts()
    const timer = setInterval(fetchHosts, 5000)
    return () => clearInterval(timer)
  }, [fetchHosts])

  // Filter & sort
  const filtered = hosts
    .filter((h) => h.ip.includes(search))
    .sort((a, b) => {
      switch (sortField) {
        case 'total':
          return (b.bytes_sent + b.bytes_recv) - (a.bytes_sent + a.bytes_recv)
        case 'connections':
          return (b.packets_sent + b.packets_recv) - (a.packets_sent + a.packets_recv)
        case 'last_seen':
          return new Date(b.last_seen).getTime() - new Date(a.last_seen).getTime()
        case 'risk':
          return (riskMap[b.ip]?.score ?? 0) - (riskMap[a.ip]?.score ?? 0)
        default:
          return 0
      }
    })

  const selectedHost = selectedIp ? hosts.find((h) => h.ip === selectedIp) : null

  // Protocol distribution for selected host
  const protoEntries = selectedHost?.protocols
    ? Object.entries(selectedHost.protocols).sort((a, b) => b[1] - a[1])
    : []
  const protoTotal = protoEntries.reduce((s, [, v]) => s + v, 0)

  return (
    <div style={styles.container}>
      {/* Header */}
      <div style={styles.header}>
        <span>🖥</span> 主机管理
      </div>

      {/* Toolbar */}
      <div style={styles.toolbar}>
        <input
          style={styles.searchInput}
          placeholder="搜索 IP 地址..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          style={styles.select}
          value={sortField}
          onChange={(e) => setSortField(e.target.value as SortField)}
        >
          <option value="total">按总流量排序</option>
          <option value="connections">按连接数排序</option>
          <option value="last_seen">按最后活跃排序</option>
          <option value="risk">按风险评分排序</option>
        </select>
        <button
          style={{ background: 'transparent', border: `1px solid ${theme.colors.sentryPurple}`, color: theme.colors.sentryPurple, padding: '6px 12px', borderRadius: '4px', cursor: 'pointer', fontSize: '13px', textTransform: 'uppercase', letterSpacing: '0.2px' }}
          onMouseEnter={e => (e.currentTarget.style.background = 'rgba(106, 95, 193, 0.1)')}
          onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
          onClick={async () => {
            try {
              const res = await exportHosts({ format: 'csv', sort: sortField })
              downloadBlob(res.data, 'hosts_export.csv')
            } catch (err) { console.error('Export failed:', err) }
          }}
        >
          📥 导出 CSV
        </button>
        <button
          style={{ background: 'transparent', border: `1px solid ${theme.colors.sentryPurple}`, color: theme.colors.sentryPurple, padding: '6px 12px', borderRadius: '4px', cursor: 'pointer', fontSize: '13px', textTransform: 'uppercase', letterSpacing: '0.2px' }}
          onMouseEnter={e => (e.currentTarget.style.background = 'rgba(106, 95, 193, 0.1)')}
          onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
          onClick={async () => {
            try {
              const res = await exportHosts({ format: 'json', sort: sortField })
              downloadBlob(res.data, 'hosts_export.json')
            } catch (err) { console.error('Export failed:', err) }
          }}
        >
          📥 导出 JSON
        </button>
      </div>

      {/* Table */}
      {loading ? (
        <div style={styles.loading}>加载中...</div>
      ) : (
        <div style={styles.tableWrap}>
          <table style={styles.table}>
            <thead>
              <tr>
                <th style={styles.th}>IP 地址</th>
                <th style={styles.th}>风险评分</th>
                <th style={styles.th}>上行流量</th>
                <th style={styles.th}>下行流量</th>
                <th style={styles.th}>总流量</th>
                <th style={styles.th}>连接数</th>
                <th style={styles.th}>最后活跃</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={7} style={styles.emptyRow}>暂无数据</td>
                </tr>
              ) : (
                filtered.map((h) => {
                  const isSelected = selectedIp === h.ip
                  const total = h.bytes_sent + h.bytes_recv
                  const conns = h.packets_sent + h.packets_recv
                  const risk = riskMap[h.ip]
                  return (
                    <tr
                      key={h.ip}
                      onClick={() => setSelectedIp(isSelected ? null : h.ip)}
                      style={{
                        cursor: 'pointer',
                        backgroundColor: isSelected ? colors.rowSelected : 'transparent',
                        transition: 'background-color 0.15s',
                      }}
                      onMouseEnter={(e) => {
                        if (!isSelected) e.currentTarget.style.backgroundColor = colors.cardHover
                      }}
                      onMouseLeave={(e) => {
                        if (!isSelected) e.currentTarget.style.backgroundColor = 'transparent'
                      }}
                    >
                      <td style={{ ...styles.td, fontWeight: 600, color: colors.accent }}>{h.ip}</td>
                      <td style={styles.td}>
                        {risk ? (
                          <span style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: '6px',
                          }}>
                            <span style={{ fontWeight: 700, color: riskColors[risk.level] || colors.textDim }}>
                              {risk.score}
                            </span>
                            <span style={{
                              display: 'inline-block',
                              padding: '1px 8px',
                              borderRadius: '10px',
                              fontSize: '11px',
                              fontWeight: 600,
                              color: '#fff',
                              backgroundColor: riskColors[risk.level] || colors.textDim,
                            }}>
                              {riskLabels[risk.level] || risk.level}
                            </span>
                          </span>
                        ) : (
                          <span style={{ color: colors.textDim }}>-</span>
                        )}
                      </td>
                      <td style={styles.td}>{formatBytes(h.bytes_sent)}</td>
                      <td style={styles.td}>{formatBytes(h.bytes_recv)}</td>
                      <td style={{ ...styles.td, fontWeight: 600 }}>{formatBytes(total)}</td>
                      <td style={styles.td}>
                        <span style={styles.badge}>{conns.toLocaleString()}</span>
                      </td>
                      <td style={{ ...styles.td, color: colors.textDim }}>{formatTime(h.last_seen)}</td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Detail Panel */}
      {selectedHost && (
        <div style={styles.detailPanel}>
          <div style={styles.detailTitle}>主机详情 — {selectedHost.ip}</div>

          <div style={styles.detailGrid}>
            <div style={styles.statCard}>
              <div style={styles.statLabel}>上行流量</div>
              <div style={styles.statValue}>{formatBytes(selectedHost.bytes_sent)}</div>
            </div>
            <div style={styles.statCard}>
              <div style={styles.statLabel}>下行流量</div>
              <div style={styles.statValue}>{formatBytes(selectedHost.bytes_recv)}</div>
            </div>
            <div style={styles.statCard}>
              <div style={styles.statLabel}>总流量</div>
              <div style={styles.statValue}>{formatBytes(selectedHost.bytes_sent + selectedHost.bytes_recv)}</div>
            </div>
            <div style={styles.statCard}>
              <div style={styles.statLabel}>发送包数</div>
              <div style={styles.statValue}>{selectedHost.packets_sent.toLocaleString()}</div>
            </div>
            <div style={styles.statCard}>
              <div style={styles.statLabel}>接收包数</div>
              <div style={styles.statValue}>{selectedHost.packets_recv.toLocaleString()}</div>
            </div>
            <div style={styles.statCard}>
              <div style={styles.statLabel}>首次发现</div>
              <div style={{ ...styles.statValue, fontSize: '14px' }}>{formatTime(selectedHost.first_seen)}</div>
            </div>
          </div>

          {/* Protocol distribution */}
          {protoEntries.length > 0 ? (
            <>
              <div style={{ fontSize: '14px', fontWeight: 600, marginBottom: '10px', color: colors.text }}>
                协议分布
              </div>
              <table style={styles.protoTable}>
                <thead>
                  <tr>
                    <th style={{ ...styles.th, fontSize: '12px', padding: '8px 12px' }}>协议</th>
                    <th style={{ ...styles.th, fontSize: '12px', padding: '8px 12px' }}>流量</th>
                    <th style={{ ...styles.th, fontSize: '12px', padding: '8px 12px', width: '40%' }}>占比</th>
                  </tr>
                </thead>
                <tbody>
                  {protoEntries.map(([proto, bytes]) => {
                    const pct = protoTotal > 0 ? (bytes / protoTotal) * 100 : 0
                    return (
                      <tr key={proto}>
                        <td style={{ ...styles.td, padding: '6px 12px', color: colors.accent }}>{proto}</td>
                        <td style={{ ...styles.td, padding: '6px 12px' }}>{formatBytes(bytes)}</td>
                        <td style={{ ...styles.td, padding: '6px 12px' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                            <div style={{ flex: 1, height: '6px', borderRadius: '3px', backgroundColor: colors.border }}>
                              <div style={{ ...styles.protoBar, width: `${pct}%` }} />
                            </div>
                            <span style={{ fontSize: '12px', color: colors.textDim, minWidth: '45px', textAlign: 'right' }}>
                              {pct.toFixed(1)}%
                            </span>
                          </div>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </>
          ) : (
            <div style={{ color: colors.textDim, fontSize: '13px' }}>暂无协议数据</div>
          )}
        </div>
      )}
    </div>
  )
}

export default Hosts
