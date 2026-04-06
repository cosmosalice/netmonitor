import { useState, useEffect, useCallback } from 'react'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js'
import { Bar } from 'react-chartjs-2'
import { getProtocolStats, exportHosts, downloadBlob } from '../api'
import { theme } from '../theme'

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend)

const CATEGORY_COLORS: Record<string, string> = {
  Web: '#6a5fc1',
  Streaming: '#fa7faa',
  Social: '#ffb287',
  Network: '#c2ef4e',
  Other: '#79628c',
}

const CATEGORIES = ['全部', 'Web', 'Streaming', 'Social', 'Network', 'Other']

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const k = 1024
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), units.length - 1)
  return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + units[i]
}

type SortKey = 'name' | 'category' | 'total_bytes' | 'percentage' | 'flow_count' | 'packet_count'
type SortDir = 'asc' | 'desc'

interface ProtocolItem {
  name: string
  category: string
  total_bytes: number
  percentage: number
  flow_count: number
  packet_count: number
}

const cardStyle: React.CSSProperties = {
  background: theme.colors.bgCard,
  borderRadius: theme.radii.lg,
  padding: 16,
  border: `1px solid ${theme.colors.border}`,
  boxShadow: theme.shadows.elevated,
}

const inputStyle: React.CSSProperties = {
  background: theme.colors.bgDeep,
  border: `1px solid ${theme.colors.border}`,
  borderRadius: theme.radii.xs,
  color: theme.colors.textPrimary,
  padding: '8px 12px',
  fontSize: 14,
  outline: 'none',
  fontFamily: theme.typography.fontFamily,
}

const Protocols = () => {
  const [protocols, setProtocols] = useState<ProtocolItem[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState('全部')
  const [sortKey, setSortKey] = useState<SortKey>('total_bytes')
  const [sortDir, setSortDir] = useState<SortDir>('desc')

  const fetchData = useCallback(async () => {
    try {
      const data = await getProtocolStats(200)
      const list: any[] = Array.isArray(data) ? data : (data?.protocols || data?.data || [])
      const mapped: ProtocolItem[] = list.map((p: any) => ({
        name: p.name || p.protocol || '-',
        category: p.category || p.type || 'Other',
        total_bytes: p.total_bytes || p.bytes || p.traffic || 0,
        percentage: p.percentage || p.percent || 0,
        flow_count: p.flow_count || p.flows || 0,
        packet_count: p.packet_count || p.packets || 0,
      }))
      setProtocols(mapped)
      setLoading(false)
    } catch {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
    const timer = setInterval(fetchData, 5000)
    return () => clearInterval(timer)
  }, [fetchData])

  const filtered = protocols
    .filter(p => {
      if (category !== '全部' && p.category !== category) return false
      if (search && !p.name.toLowerCase().includes(search.toLowerCase())) return false
      return true
    })
    .sort((a, b) => {
      const av = a[sortKey]
      const bv = b[sortKey]
      if (typeof av === 'string' && typeof bv === 'string') {
        return sortDir === 'asc' ? av.localeCompare(bv) : bv.localeCompare(av)
      }
      return sortDir === 'asc' ? (av as number) - (bv as number) : (bv as number) - (av as number)
    })

  const maxBytes = Math.max(...filtered.map(p => p.total_bytes), 1)

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    } else {
      setSortKey(key)
      setSortDir('desc')
    }
  }

  const sortIndicator = (key: SortKey) =>
    sortKey === key ? (sortDir === 'asc' ? ' ▲' : ' ▼') : ''

  // Category chart data
  const categoryMap: Record<string, number> = {}
  protocols.forEach(p => {
    categoryMap[p.category] = (categoryMap[p.category] || 0) + p.total_bytes
  })
  const catLabels = Object.keys(categoryMap)
  const catValues = Object.values(categoryMap)

  const barData = {
    labels: catLabels.length > 0 ? catLabels : ['暂无数据'],
    datasets: [{
      label: '流量',
      data: catValues.length > 0 ? catValues : [0],
      backgroundColor: catLabels.map(c => CATEGORY_COLORS[c] || '#9966ff'),
      borderRadius: 4,
    }],
  }

  const barOptions: any = {
    indexAxis: 'y' as const,
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { display: false },
      title: { display: true, text: '类别流量分布', color: theme.colors.textPrimary, font: { family: theme.typography.fontFamily } },
      tooltip: {
        callbacks: {
          label: (ctx: any) => formatBytes(ctx.parsed.x),
        },
      },
    },
    scales: {
      x: {
        ticks: { color: theme.colors.textDim, callback: (v: number) => formatBytes(v), font: { family: theme.typography.fontFamily } },
        grid: { color: 'rgba(255,255,255,0.05)' },
      },
      y: {
        ticks: { color: theme.colors.textDim, font: { family: theme.typography.fontFamily } },
        grid: { color: 'rgba(255,255,255,0.05)' },
      },
    },
  }

  const thStyle: React.CSSProperties = {
    padding: '8px 10px',
    textAlign: 'left',
    color: theme.colors.sentryPurple,
    fontWeight: 600,
    cursor: 'pointer',
    userSelect: 'none',
    whiteSpace: 'nowrap',
    borderBottom: `1px solid ${theme.colors.border}`,
    fontSize: 13,
    fontFamily: theme.typography.fontFamily,
  }

  const tdStyle: React.CSSProperties = {
    padding: '6px 10px',
    borderBottom: '1px solid rgba(255,255,255,0.05)',
    fontSize: 13,
  }

  return (
    <div style={{ padding: 20, background: theme.colors.bgPrimary, minHeight: '100vh', color: theme.colors.textPrimary, fontFamily: theme.typography.fontFamily }}>
      <h1 style={{ margin: '0 0 16px', fontSize: 22 }}>协议分析</h1>

      {/* 搜索/过滤栏 */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, alignItems: 'center' }}>
        <input
          type="text"
          placeholder="搜索协议名称..."
          value={search}
          onChange={e => setSearch(e.target.value)}
          style={{ ...inputStyle, width: 240 }}
        />
        <select
          value={category}
          onChange={e => setCategory(e.target.value)}
          style={{ ...inputStyle, cursor: 'pointer' }}
        >
          {CATEGORIES.map(c => (
            <option key={c} value={c}>{c}</option>
          ))}
        </select>
        <span style={{ opacity: 0.5, fontSize: 13, marginLeft: 'auto' }}>
          共 {filtered.length} 条 · 每 5 秒刷新
        </span>
        <button
          style={{ background: 'transparent', border: `1px solid ${theme.colors.sentryPurple}`, color: theme.colors.sentryPurple, padding: '6px 12px', borderRadius: theme.radii.xs, cursor: 'pointer', fontSize: '13px', textTransform: 'uppercase', letterSpacing: '0.2px', fontFamily: theme.typography.fontFamily }}
          onMouseEnter={e => (e.currentTarget.style.background = theme.colors.glassDeep)}
          onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
          onClick={async () => {
            try {
              const res = await exportHosts({ format: 'csv', sort: 'total' })
              downloadBlob(res.data, 'protocols_export.csv')
            } catch (err) { console.error('Export failed:', err) }
          }}
        >
          📥 导出协议数据 CSV
        </button>
      </div>

      {/* 主体区域 */}
      <div style={{ display: 'grid', gridTemplateColumns: '7fr 3fr', gap: 16, height: 'calc(100vh - 160px)' }}>
        {/* 表格 */}
        <div style={{ ...cardStyle, overflow: 'auto' }}>
          {loading ? (
            <p style={{ opacity: 0.5, textAlign: 'center', paddingTop: 40 }}>加载中...</p>
          ) : filtered.length === 0 ? (
            <p style={{ opacity: 0.5, textAlign: 'center', paddingTop: 40 }}>暂无数据</p>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr>
                  <th style={thStyle} onClick={() => handleSort('name')}>协议名称{sortIndicator('name')}</th>
                  <th style={thStyle} onClick={() => handleSort('category')}>类别{sortIndicator('category')}</th>
                  <th style={thStyle} onClick={() => handleSort('total_bytes')}>总流量{sortIndicator('total_bytes')}</th>
                  <th style={{ ...thStyle, minWidth: 120 }} onClick={() => handleSort('percentage')}>占比{sortIndicator('percentage')}</th>
                  <th style={thStyle} onClick={() => handleSort('flow_count')}>流数{sortIndicator('flow_count')}</th>
                  <th style={thStyle} onClick={() => handleSort('packet_count')}>包数{sortIndicator('packet_count')}</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((p, i) => (
                  <tr key={p.name + i} style={{ transition: 'background 0.2s' }}
                    onMouseEnter={e => (e.currentTarget.style.background = 'rgba(255,255,255,0.03)')}
                    onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                  >
                    <td style={{ ...tdStyle, fontWeight: 500 }}>{p.name}</td>
                    <td style={tdStyle}>
                      <span style={{
                        display: 'inline-block',
                        padding: '2px 8px',
                        borderRadius: 4,
                        fontSize: 11,
                        background: CATEGORY_COLORS[p.category] ? `${CATEGORY_COLORS[p.category]}22` : 'rgba(153,102,255,0.13)',
                        color: CATEGORY_COLORS[p.category] || '#9966ff',
                      }}>
                        {p.category}
                      </span>
                    </td>
                    <td style={tdStyle}>{formatBytes(p.total_bytes)}</td>
                    <td style={tdStyle}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <div style={{ flex: 1, height: 6, background: 'rgba(255,255,255,0.08)', borderRadius: 3, overflow: 'hidden' }}>
                          <div style={{
                            width: `${Math.min((p.total_bytes / maxBytes) * 100, 100)}%`,
                            height: '100%',
                            background: theme.colors.sentryPurple,
                            borderRadius: 3,
                            transition: 'width 0.3s',
                          }} />
                        </div>
                        <span style={{ fontSize: 11, opacity: 0.7, minWidth: 40, textAlign: 'right' }}>
                          {p.percentage.toFixed(1)}%
                        </span>
                      </div>
                    </td>
                    <td style={tdStyle}>{p.flow_count.toLocaleString()}</td>
                    <td style={tdStyle}>{p.packet_count.toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* 类别图表 */}
        <div style={cardStyle}>
          <Bar data={barData} options={barOptions} />
        </div>
      </div>
    </div>
  )
}

export default Protocols
