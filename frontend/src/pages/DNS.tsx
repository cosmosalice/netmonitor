import { useState, useEffect, useCallback } from 'react'
import {
  Chart, CategoryScale, LinearScale, BarElement, ArcElement, Title, Tooltip, Legend,
} from 'chart.js'
import { Doughnut, Bar } from 'react-chartjs-2'
import {
  getDNSSummary, getDNSDomains, getDNSServers, getDNSResponseCodes, getDNSQueryTypes,
} from '../api/index'
import { theme } from '../theme'

Chart.register(CategoryScale, LinearScale, BarElement, ArcElement, Title, Tooltip, Legend)

// ── Styles ──
const S: Record<string, React.CSSProperties> = {
  page: { padding: 24, background: theme.colors.bgPrimary, minHeight: '100%', color: theme.colors.textPrimary, fontFamily: theme.typography.fontFamily },
  title: { fontSize: 22, fontWeight: 700, color: theme.colors.textPrimary, marginBottom: 24 },
  row: { display: 'flex', gap: 16, marginBottom: 16, flexWrap: 'wrap' as const },
  card: { background: theme.colors.bgCard, borderRadius: theme.radii.lg, padding: 20, border: `1px solid ${theme.colors.border}`, flex: 1, minWidth: 200, boxShadow: theme.shadows.elevated },
  cardTitle: { fontSize: 14, fontWeight: 600, color: theme.colors.textDim, marginBottom: 8 },
  cardValue: { fontSize: 28, fontWeight: 700, color: theme.colors.textPrimary },
  cardSub: { fontSize: 12, color: theme.colors.textDim, marginTop: 4 },
  section: { background: theme.colors.bgCard, borderRadius: theme.radii.lg, padding: 20, border: `1px solid ${theme.colors.border}`, marginBottom: 16, boxShadow: theme.shadows.elevated },
  sectionTitle: { fontSize: 16, fontWeight: 700, color: theme.colors.textPrimary, marginBottom: 16 },
  table: { width: '100%', borderCollapse: 'collapse' as const, fontSize: 13 },
  th: { textAlign: 'left' as const, padding: '10px 12px', borderBottom: `1px solid ${theme.colors.border}`, color: theme.colors.textDim, fontWeight: 600 },
  td: { padding: '10px 12px', borderBottom: `1px solid ${theme.colors.border}22` },
  tag: { display: 'inline-block', background: theme.colors.bgDeep, color: theme.colors.sentryPurple, borderRadius: theme.radii.xs, padding: '2px 6px', fontSize: 11, marginRight: 4, marginBottom: 2 },
  chartWrap: { maxWidth: 320, margin: '0 auto' },
}

interface DNSSummaryData {
  total_queries: number
  total_responses: number
  unique_domains: number
  unique_servers: number
  nxdomain_count: number
  nxdomain_pct: number
}

interface DomainStat {
  domain: string
  query_count: number
  first_seen: string
  last_seen: string
  query_types: string[]
  response_ips: string[]
}

interface ServerStat {
  ip: string
  query_count: number
  avg_latency_ms: number
}

function fmtTime(ts: string) {
  if (!ts) return '-'
  const d = new Date(ts)
  return d.toLocaleString()
}

function fmtNum(n: number) {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n ?? 0)
}

const DNS: React.FC = () => {
  const [summary, setSummary] = useState<DNSSummaryData | null>(null)
  const [domains, setDomains] = useState<DomainStat[]>([])
  const [servers, setServers] = useState<ServerStat[]>([])
  const [responseCodes, setResponseCodes] = useState<Record<string, number>>({})
  const [queryTypes, setQueryTypes] = useState<Record<string, number>>({})

  const fetchAll = useCallback(async () => {
    try {
      const [sumRes, domRes, srvRes, rcRes, qtRes] = await Promise.all([
        getDNSSummary(),
        getDNSDomains(50),
        getDNSServers(20),
        getDNSResponseCodes(),
        getDNSQueryTypes(),
      ])
      setSummary(sumRes)
      setDomains(domRes?.domains || [])
      setServers(srvRes?.servers || [])
      setResponseCodes(rcRes?.response_codes || {})
      setQueryTypes(qtRes?.query_types || {})
    } catch (e) {
      console.error('DNS fetch error', e)
    }
  }, [])

  useEffect(() => {
    fetchAll()
    const timer = setInterval(fetchAll, 5000)
    return () => clearInterval(timer)
  }, [fetchAll])

  // ── Chart data builders ──
  const rcLabels = Object.keys(responseCodes)
  const rcValues = Object.values(responseCodes)
  const rcChartData = {
    labels: rcLabels,
    datasets: [{
      data: rcValues,
      backgroundColor: rcLabels.map((_, i) => theme.colors.chartColors[i % theme.colors.chartColors.length]),
      borderWidth: 0,
    }],
  }

  const qtLabels = Object.keys(queryTypes)
  const qtValues = Object.values(queryTypes)
  const qtChartData = {
    labels: qtLabels,
    datasets: [{
      data: qtValues,
      backgroundColor: qtLabels.map((_, i) => theme.colors.chartColors[i % theme.colors.chartColors.length]),
      borderWidth: 0,
    }],
  }

  const top10 = domains.slice(0, 10)
  const barData = {
    labels: top10.map(d => d.domain.length > 28 ? d.domain.slice(0, 26) + '…' : d.domain),
    datasets: [{
      label: '查询次数',
      data: top10.map(d => d.query_count),
      backgroundColor: theme.colors.sentryPurple,
      borderRadius: 4,
    }],
  }

  const doughnutOpts: any = {
    responsive: true,
    plugins: {
      legend: { position: 'bottom' as const, labels: { color: theme.colors.textDim, boxWidth: 12, padding: 12, font: { size: 11, family: theme.typography.fontFamily } } },
      tooltip: { backgroundColor: '#000', titleColor: '#fff', bodyColor: '#fff' },
    },
    cutout: '60%',
  }

  const barOpts: any = {
    responsive: true,
    indexAxis: 'y' as const,
    plugins: {
      legend: { display: false },
      tooltip: { backgroundColor: '#000', titleColor: '#fff', bodyColor: '#fff' },
    },
    scales: {
      x: { ticks: { color: theme.colors.textDim, font: { family: theme.typography.fontFamily } }, grid: { color: `${theme.colors.border}44` } },
      y: { ticks: { color: theme.colors.textDim, font: { size: 11, family: theme.typography.fontFamily } }, grid: { display: false } },
    },
  }

  return (
    <div style={S.page}>
      <div style={S.title}>DNS 分析</div>

      {/* ── Summary Cards ── */}
      <div style={S.row}>
        <div style={S.card}>
          <div style={S.cardTitle}>总查询数</div>
          <div style={S.cardValue}>{fmtNum(summary?.total_queries ?? 0)}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardTitle}>总响应数</div>
          <div style={S.cardValue}>{fmtNum(summary?.total_responses ?? 0)}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardTitle}>NXDOMAIN 比例</div>
          <div style={S.cardValue}>{(summary?.nxdomain_pct ?? 0).toFixed(1)}%</div>
          <div style={S.cardSub}>{summary?.nxdomain_count ?? 0} 条 NXDOMAIN</div>
        </div>
        <div style={S.card}>
          <div style={S.cardTitle}>唯一域名数</div>
          <div style={S.cardValue}>{fmtNum(summary?.unique_domains ?? 0)}</div>
        </div>
        <div style={S.card}>
          <div style={S.cardTitle}>DNS 服务器数</div>
          <div style={S.cardValue}>{summary?.unique_servers ?? 0}</div>
        </div>
      </div>

      {/* ── Top Domains Bar Chart ── */}
      {top10.length > 0 && (
        <div style={S.section}>
          <div style={S.sectionTitle}>Top 10 查询域名</div>
          <Bar data={barData} options={barOpts} height={top10.length * 28 + 40} />
        </div>
      )}

      {/* ── Top Domains Table ── */}
      <div style={S.section}>
        <div style={S.sectionTitle}>Top 查询域名</div>
        <div style={{ overflowX: 'auto' }}>
          <table style={S.table}>
            <thead>
              <tr>
                <th style={S.th}>#</th>
                <th style={S.th}>域名</th>
                <th style={S.th}>查询次数</th>
                <th style={S.th}>查询类型</th>
                <th style={S.th}>解析 IP</th>
                <th style={S.th}>首次查询</th>
                <th style={S.th}>最后查询</th>
              </tr>
            </thead>
            <tbody>
              {domains.map((d, i) => (
                <tr key={d.domain} style={{ background: i % 2 === 0 ? 'transparent' : `${theme.colors.border}22` }}>
                  <td style={S.td}>{i + 1}</td>
                  <td style={{ ...S.td, color: theme.colors.sentryPurple, fontWeight: 600, maxWidth: 300, wordBreak: 'break-all' }}>{d.domain}</td>
                  <td style={{ ...S.td, fontWeight: 600 }}>{d.query_count}</td>
                  <td style={S.td}>
                    {(d.query_types || []).map(t => <span key={t} style={S.tag}>{t}</span>)}
                  </td>
                  <td style={{ ...S.td, maxWidth: 200, wordBreak: 'break-all', fontSize: 12 }}>
                    {(d.response_ips || []).slice(0, 3).join(', ')}
                    {(d.response_ips || []).length > 3 && ` +${d.response_ips.length - 3}`}
                  </td>
                  <td style={{ ...S.td, fontSize: 12 }}>{fmtTime(d.first_seen)}</td>
                  <td style={{ ...S.td, fontSize: 12 }}>{fmtTime(d.last_seen)}</td>
                </tr>
              ))}
              {domains.length === 0 && (
                <tr><td colSpan={7} style={{ ...S.td, textAlign: 'center', color: theme.colors.textDim }}>暂无 DNS 数据，请先开始抓包</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* ── Charts Row: Response Codes + Query Types ── */}
      <div style={{ ...S.row }}>
        <div style={{ ...S.section, flex: 1 }}>
          <div style={S.sectionTitle}>响应码分布</div>
          <div style={S.chartWrap}>
            {rcLabels.length > 0 ? <Doughnut data={rcChartData} options={doughnutOpts} /> : <div style={{ color: theme.colors.textDim, textAlign: 'center', padding: 40 }}>暂无数据</div>}
          </div>
        </div>
        <div style={{ ...S.section, flex: 1 }}>
          <div style={S.sectionTitle}>查询类型分布</div>
          <div style={S.chartWrap}>
            {qtLabels.length > 0 ? <Doughnut data={qtChartData} options={doughnutOpts} /> : <div style={{ color: theme.colors.textDim, textAlign: 'center', padding: 40 }}>暂无数据</div>}
          </div>
        </div>
      </div>

      {/* ── Top DNS Servers ── */}
      <div style={S.section}>
        <div style={S.sectionTitle}>Top DNS 服务器</div>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>#</th>
              <th style={S.th}>IP 地址</th>
              <th style={S.th}>查询次数</th>
            </tr>
          </thead>
          <tbody>
            {servers.map((s, i) => (
              <tr key={s.ip} style={{ background: i % 2 === 0 ? 'transparent' : `${theme.colors.border}22` }}>
                <td style={S.td}>{i + 1}</td>
                <td style={{ ...S.td, color: theme.colors.sentryPurple, fontWeight: 600 }}>{s.ip}</td>
                <td style={{ ...S.td, fontWeight: 600 }}>{s.query_count}</td>
              </tr>
            ))}
            {servers.length === 0 && (
              <tr><td colSpan={3} style={{ ...S.td, textAlign: 'center', color: theme.colors.textDim }}>暂无数据</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default DNS
