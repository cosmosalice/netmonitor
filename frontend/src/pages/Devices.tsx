import { useState, useEffect, useCallback } from 'react'
import { theme } from '../theme'

interface Device {
  mac: string
  name: string
  vendor: string
  ips: string[]
  first_seen: string
  last_seen: string
  bytes_sent: number
  bytes_recv: number
  flow_count: number
  device_type: string
  is_online: boolean
  hostname?: string
  os?: string
}

interface DeviceStats {
  online_count: number
  offline_count: number
  total_count: number
  vendor_dist: Record<string, number>
  type_dist: Record<string, number>
}

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

const deviceTypeIcons: Record<string, string> = {
  pc: '💻',
  server: '🖥',
  router: '📡',
  phone: '📱',
  iot: '🔌',
  unknown: '❓',
}

const deviceTypeLabels: Record<string, string> = {
  pc: '电脑',
  server: '服务器',
  router: '路由器',
  phone: '手机',
  iot: 'IoT设备',
  unknown: '未知',
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
  warning: theme.colors.warning,
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
  statsRow: {
    display: 'flex',
    gap: '16px',
    marginBottom: '20px',
    flexWrap: 'wrap' as const,
  },
  statCard: {
    padding: '16px 24px',
    borderRadius: '10px',
    backgroundColor: colors.card,
    border: `1px solid ${colors.border}`,
    minWidth: '140px',
  },
  statLabel: {
    fontSize: '12px',
    color: colors.textDim,
    marginBottom: '4px',
  },
  statValue: {
    fontSize: '24px',
    fontWeight: 700,
    color: colors.accent,
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
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
  },
  detailGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
    gap: '12px',
    marginBottom: '20px',
  },
  detailStatCard: {
    padding: '12px 16px',
    borderRadius: '8px',
    backgroundColor: colors.accentDark,
    textAlign: 'center' as const,
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
  onlineBadge: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: '10px',
    fontSize: '11px',
    fontWeight: 600,
  },
  ipList: {
    display: 'flex',
    flexWrap: 'wrap' as const,
    gap: '4px',
    maxWidth: '200px',
  },
  ipTag: {
    padding: '2px 6px',
    borderRadius: '4px',
    fontSize: '11px',
    backgroundColor: colors.accentDark,
    color: colors.accent,
  },
}

const Devices = () => {
  const [devices, setDevices] = useState<Device[]>([])
  const [stats, setStats] = useState<DeviceStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [sortField, setSortField] = useState('traffic')
  const [selectedMac, setSelectedMac] = useState<string | null>(null)
  const [detailFlows, setDetailFlows] = useState<any[]>([])

  const fetchData = useCallback(async () => {
    try {
      const [devicesRes, statsRes] = await Promise.all([
        fetch(`http://localhost:8080/api/v1/devices?sort=${sortField}&limit=200`).then(r => r.json()),
        fetch('http://localhost:8080/api/v1/devices/stats').then(r => r.json()),
      ])
      setDevices(devicesRes.devices || [])
      setStats(statsRes)
    } catch (err) {
      console.error('Failed to fetch devices:', err)
    } finally {
      setLoading(false)
    }
  }, [sortField])

  useEffect(() => {
    fetchData()
    const timer = setInterval(fetchData, 5000)
    return () => clearInterval(timer)
  }, [fetchData])

  // Fetch device flows when a device is selected
  useEffect(() => {
    if (selectedMac) {
      fetch(`http://localhost:8080/api/v1/devices/${encodeURIComponent(selectedMac)}/flows?limit=20`)
        .then(r => r.json())
        .then(data => setDetailFlows(data.flows || []))
        .catch(() => setDetailFlows([]))
    } else {
      setDetailFlows([])
    }
  }, [selectedMac])

  // Filter devices by search
  const filtered = devices.filter(d => {
    if (!search) return true
    const q = search.toLowerCase()
    return d.mac.toLowerCase().includes(q) ||
           d.vendor.toLowerCase().includes(q) ||
           d.ips.some(ip => ip.includes(q)) ||
           (d.name && d.name.toLowerCase().includes(q))
  })

  const selectedDevice = selectedMac ? devices.find(d => d.mac === selectedMac) : null

  return (
    <div style={styles.container}>
      {/* Header */}
      <div style={styles.header}>
        <span>🔍</span> 设备发现
      </div>

      {/* Stats Row */}
      {stats && (
        <div style={styles.statsRow}>
          <div style={styles.statCard}>
            <div style={styles.statLabel}>在线设备</div>
            <div style={{ ...styles.statValue, color: colors.success }}>{stats.online_count}</div>
          </div>
          <div style={styles.statCard}>
            <div style={styles.statLabel}>离线设备</div>
            <div style={{ ...styles.statValue, color: colors.textDim }}>{stats.offline_count}</div>
          </div>
          <div style={styles.statCard}>
            <div style={styles.statLabel}>总设备数</div>
            <div style={styles.statValue}>{stats.total_count}</div>
          </div>
        </div>
      )}

      {/* Toolbar */}
      <div style={styles.toolbar}>
        <input
          style={styles.searchInput}
          placeholder="搜索 MAC、IP、厂商..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select
          style={styles.select}
          value={sortField}
          onChange={(e) => setSortField(e.target.value)}
        >
          <option value="traffic">按流量排序</option>
          <option value="last_seen">按最近活跃排序</option>
          <option value="flow_count">按流数排序</option>
          <option value="mac">按 MAC 排序</option>
        </select>
      </div>

      {/* Table */}
      {loading ? (
        <div style={styles.loading}>加载中...</div>
      ) : (
        <div style={styles.tableWrap}>
          <table style={styles.table}>
            <thead>
              <tr>
                <th style={styles.th}>状态</th>
                <th style={styles.th}>MAC 地址</th>
                <th style={styles.th}>厂商</th>
                <th style={styles.th}>IP 地址</th>
                <th style={styles.th}>设备类型</th>
                <th style={styles.th}>上行流量</th>
                <th style={styles.th}>下行流量</th>
                <th style={styles.th}>流数</th>
                <th style={styles.th}>最后活跃</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={9} style={styles.emptyRow}>暂无设备数据</td>
                </tr>
              ) : (
                filtered.map((d) => {
                  const isSelected = selectedMac === d.mac
                  return (
                    <tr
                      key={d.mac}
                      onClick={() => setSelectedMac(isSelected ? null : d.mac)}
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
                      <td style={styles.td}>
                        <span style={{
                          ...styles.onlineBadge,
                          backgroundColor: d.is_online ? colors.success : colors.textDim,
                          color: '#fff',
                        }}>
                          {d.is_online ? '在线' : '离线'}
                        </span>
                      </td>
                      <td style={{ ...styles.td, fontWeight: 600, color: colors.accent }}>{d.mac}</td>
                      <td style={styles.td}>{d.vendor || '-'}</td>
                      <td style={styles.td}>
                        <div style={styles.ipList}>
                          {d.ips && d.ips.length > 0 ? (
                            d.ips.slice(0, 3).map((ip, i) => (
                              <span key={i} style={styles.ipTag}>{ip}</span>
                            ))
                          ) : (
                            <span style={{ color: colors.textDim }}>-</span>
                          )}
                          {d.ips && d.ips.length > 3 && (
                            <span style={styles.ipTag}>+{d.ips.length - 3}</span>
                          )}
                        </div>
                      </td>
                      <td style={styles.td}>
                        <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                          <span>{deviceTypeIcons[d.device_type] || deviceTypeIcons.unknown}</span>
                          <span style={{ color: colors.textDim, fontSize: '12px' }}>
                            {deviceTypeLabels[d.device_type] || '未知'}
                          </span>
                        </span>
                      </td>
                      <td style={styles.td}>{formatBytes(d.bytes_sent)}</td>
                      <td style={styles.td}>{formatBytes(d.bytes_recv)}</td>
                      <td style={styles.td}>
                        <span style={styles.badge}>{d.flow_count.toLocaleString()}</span>
                      </td>
                      <td style={{ ...styles.td, color: colors.textDim }}>{formatTime(d.last_seen)}</td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Detail Panel */}
      {selectedDevice && (
        <div style={styles.detailPanel}>
          <div style={styles.detailTitle}>
            <span>{deviceTypeIcons[selectedDevice.device_type] || deviceTypeIcons.unknown}</span>
            设备详情 — {selectedDevice.mac}
          </div>

          <div style={styles.detailGrid}>
            <div style={styles.detailStatCard}>
              <div style={styles.statLabel}>厂商</div>
              <div style={styles.statValue}>{selectedDevice.vendor || '未知'}</div>
            </div>
            <div style={styles.detailStatCard}>
              <div style={styles.statLabel}>设备类型</div>
              <div style={styles.statValue}>{deviceTypeLabels[selectedDevice.device_type] || '未知'}</div>
            </div>
            <div style={styles.detailStatCard}>
              <div style={styles.statLabel}>上行流量</div>
              <div style={styles.statValue}>{formatBytes(selectedDevice.bytes_sent)}</div>
            </div>
            <div style={styles.detailStatCard}>
              <div style={styles.statLabel}>下行流量</div>
              <div style={styles.statValue}>{formatBytes(selectedDevice.bytes_recv)}</div>
            </div>
            <div style={styles.detailStatCard}>
              <div style={styles.statLabel}>总流量</div>
              <div style={styles.statValue}>{formatBytes(selectedDevice.bytes_sent + selectedDevice.bytes_recv)}</div>
            </div>
            <div style={styles.detailStatCard}>
              <div style={styles.statLabel}>流数量</div>
              <div style={styles.statValue}>{selectedDevice.flow_count.toLocaleString()}</div>
            </div>
            <div style={styles.detailStatCard}>
              <div style={styles.statLabel}>首次发现</div>
              <div style={{ ...styles.statValue, fontSize: '14px' }}>{formatTime(selectedDevice.first_seen)}</div>
            </div>
            <div style={styles.detailStatCard}>
              <div style={styles.statLabel}>最后活跃</div>
              <div style={{ ...styles.statValue, fontSize: '14px' }}>{formatTime(selectedDevice.last_seen)}</div>
            </div>
          </div>

          {/* IP Addresses */}
          {selectedDevice.ips && selectedDevice.ips.length > 0 && (
            <>
              <div style={{ fontSize: '14px', fontWeight: 600, marginBottom: '10px', color: colors.text }}>
                IP 地址
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginBottom: '20px' }}>
                {selectedDevice.ips.map((ip, i) => (
                  <span key={i} style={{ ...styles.ipTag, padding: '6px 12px', fontSize: '13px' }}>{ip}</span>
                ))}
              </div>
            </>
          )}

          {/* Recent Flows */}
          {detailFlows.length > 0 && (
            <>
              <div style={{ fontSize: '14px', fontWeight: 600, marginBottom: '10px', color: colors.text }}>
                最近关联流
              </div>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ ...styles.table, fontSize: '13px' }}>
                  <thead>
                    <tr>
                      <th style={{ ...styles.th, fontSize: '12px', padding: '8px 12px' }}>源 IP:端口</th>
                      <th style={{ ...styles.th, fontSize: '12px', padding: '8px 12px' }}>目标 IP:端口</th>
                      <th style={{ ...styles.th, fontSize: '12px', padding: '8px 12px' }}>协议</th>
                      <th style={{ ...styles.th, fontSize: '12px', padding: '8px 12px' }}>L7 协议</th>
                      <th style={{ ...styles.th, fontSize: '12px', padding: '8px 12px' }}>流量</th>
                      <th style={{ ...styles.th, fontSize: '12px', padding: '8px 12px' }}>状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    {detailFlows.slice(0, 10).map((f, i) => (
                      <tr key={i}>
                        <td style={{ ...styles.td, padding: '6px 12px' }}>{f.src_ip}:{f.src_port}</td>
                        <td style={{ ...styles.td, padding: '6px 12px' }}>{f.dst_ip}:{f.dst_port}</td>
                        <td style={{ ...styles.td, padding: '6px 12px' }}>{f.protocol}</td>
                        <td style={{ ...styles.td, padding: '6px 12px' }}>{f.l7_protocol || '-'}</td>
                        <td style={{ ...styles.td, padding: '6px 12px' }}>{formatBytes((f.bytes_sent || 0) + (f.bytes_recv || 0))}</td>
                        <td style={{ ...styles.td, padding: '6px 12px' }}>
                          <span style={{
                            ...styles.onlineBadge,
                            backgroundColor: f.is_active ? colors.success : colors.textDim,
                            color: '#fff',
                          }}>
                            {f.is_active ? '活跃' : '结束'}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  )
}

export default Devices
