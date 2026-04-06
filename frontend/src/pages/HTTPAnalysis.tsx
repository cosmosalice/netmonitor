import { useState, useEffect, useCallback } from 'react'
import {
  Chart, CategoryScale, LinearScale, BarElement, ArcElement, Title, Tooltip, Legend,
} from 'chart.js'
import { Doughnut, Bar } from 'react-chartjs-2'
import {
  getHTTPSummary, getHTTPHosts, getHTTPUserAgents, getHTTPMethods, getHTTPStatusCodes,
  getTLSSummary, getTLSSNI, getTLSJA3, getTLSVersions,
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
  tabContainer: { display: 'flex', gap: 8, marginBottom: 24 },
  tab: { padding: '10px 20px', borderRadius: theme.radii.sm, cursor: 'pointer', fontWeight: 600, transition: 'all 0.2s', textTransform: 'uppercase', letterSpacing: '0.2px' },
  tabActive: { background: theme.colors.sentryPurple, color: '#fff' },
  tabInactive: { background: theme.colors.bgDeep, color: theme.colors.textDim },
  chartWrap: { maxWidth: 320, margin: '0 auto' },
}

// ── Types ──
interface HTTPSummaryData {
  total_requests: number
  total_responses: number
  unique_hosts: number
  top_methods: Record<string, number>
  top_status_codes: Record<number, number>
}

interface HTTPHostStats {
  host: string
  request_count: number
  bytes_in: number
  bytes_out: number
  last_seen: string
}

interface UserAgentEntry {
  user_agent: string
  count: number
}

interface TLSSummaryData {
  total_handshakes: number
  unique_sni: number
  unique_ja3: number
  tls_versions: Record<string, number>
}

interface SNIStats {
  domain: string
  count: number
  first_seen: string
  last_seen: string
}

interface JA3Stats {
  hash: string
  count: number
  user_agents?: string[]
}

function fmtNum(n: number) {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n ?? 0)
}

function fmtBytes(n: number) {
  if (n >= 1_073_741_824) return (n / 1_073_741_824).toFixed(1) + ' GB'
  if (n >= 1_048_576) return (n / 1_048_576).toFixed(1) + ' MB'
  if (n >= 1_024) return (n / 1_024).toFixed(1) + ' KB'
  return String(n ?? 0) + ' B'
}

const HTTPAnalysis: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'http' | 'tls'>('http')

  // HTTP state
  const [httpSummary, setHTTPSummary] = useState<HTTPSummaryData | null>(null)
  const [httpHosts, setHTTPHosts] = useState<HTTPHostStats[]>([])
  const [userAgents, setUserAgents] = useState<UserAgentEntry[]>([])
  const [httpMethods, setHTTPMethods] = useState<Record<string, number>>({})
  const [statusCodes, setStatusCodes] = useState<Record<number, number>>({})

  // TLS state
  const [tlsSummary, setTLSSummary] = useState<TLSSummaryData | null>(null)
  const [sniDomains, setSNIDomains] = useState<SNIStats[]>([])
  const [ja3Hashes, setJA3Hashes] = useState<JA3Stats[]>([])
  const [tlsVersions, setTLSVersions] = useState<Record<string, number>>({})

  // Fetch HTTP data
  const fetchHTTPData = useCallback(async () => {
    try {
      const [sumRes, hostsRes, uaRes, methodsRes, codesRes] = await Promise.all([
        getHTTPSummary(),
        getHTTPHosts(50),
        getHTTPUserAgents(30),
        getHTTPMethods(),
        getHTTPStatusCodes(),
      ])
      setHTTPSummary(sumRes)
      setHTTPHosts(hostsRes?.hosts || [])
      setUserAgents(uaRes?.user_agents || [])
      setHTTPMethods(methodsRes?.methods || {})
      setStatusCodes(codesRes?.status_codes || {})
    } catch (e) {
      console.error('HTTP fetch error', e)
    }
  }, [])

  // Fetch TLS data
  const fetchTLSData = useCallback(async () => {
    try {
      const [sumRes, sniRes, ja3Res, verRes] = await Promise.all([
        getTLSSummary(),
        getTLSSNI(50),
        getTLSJA3(30),
        getTLSVersions(),
      ])
      setTLSSummary(sumRes)
      setSNIDomains(sniRes?.sni_domains || [])
      setJA3Hashes(ja3Res?.ja3_hashes || [])
      setTLSVersions(verRes?.versions || {})
    } catch (e) {
      console.error('TLS fetch error', e)
    }
  }, [])

  useEffect(() => {
    fetchHTTPData()
    fetchTLSData()
    const timer = setInterval(() => {
      fetchHTTPData()
      fetchTLSData()
    }, 5000)
    return () => clearInterval(timer)
  }, [fetchHTTPData, fetchTLSData])

  // ── Chart data builders ──
  const methodLabels = Object.keys(httpMethods)
  const methodValues = Object.values(httpMethods)
  const methodChartData = {
    labels: methodLabels,
    datasets: [{
      data: methodValues,
      backgroundColor: methodLabels.map((_, i) => theme.colors.chartColors[i % theme.colors.chartColors.length]),
      borderWidth: 0,
    }],
  }

  const codeLabels = Object.keys(statusCodes).map(String)
  const codeValues = Object.values(statusCodes)
  const codeChartData = {
    labels: codeLabels,
    datasets: [{
      label: '请求数',
      data: codeValues,
      backgroundColor: codeLabels.map((code, i) => {
        const c = parseInt(code)
        if (c >= 200 && c < 300) return theme.colors.lime
        if (c >= 300 && c < 400) return theme.colors.warning
        if (c >= 400 && c < 500) return '#ff5722'
        if (c >= 500) return theme.colors.error
        return theme.colors.chartColors[i % theme.colors.chartColors.length]
      }),
      borderRadius: 4,
    }],
  }

  const versionLabels = Object.keys(tlsVersions)
  const versionValues = Object.values(tlsVersions)
  const versionChartData = {
    labels: versionLabels,
    datasets: [{
      data: versionValues,
      backgroundColor: versionLabels.map((_, i) => theme.colors.chartColors[i % theme.colors.chartColors.length]),
      borderWidth: 0,
    }],
  }

  // Top 10 hosts bar chart
  const top10Hosts = httpHosts.slice(0, 10)
  const hostsBarData = {
    labels: top10Hosts.map(h => h.host.length > 28 ? h.host.slice(0, 26) + '…' : h.host),
    datasets: [{
      label: '请求数',
      data: top10Hosts.map(h => h.request_count),
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

  const codeBarOpts: any = {
    responsive: true,
    plugins: {
      legend: { display: false },
      tooltip: { backgroundColor: '#000', titleColor: '#fff', bodyColor: '#fff' },
    },
    scales: {
      x: { ticks: { color: theme.colors.textDim, font: { family: theme.typography.fontFamily } }, grid: { color: `${theme.colors.border}44` } },
      y: { ticks: { color: theme.colors.textDim, font: { family: theme.typography.fontFamily } }, grid: { display: false } },
    },
  }

  return (
    <div style={S.page}>
      <div style={S.title}>HTTP/TLS 分析</div>

      {/* ── Tabs ── */}
      <div style={S.tabContainer}>
        <div
          style={{ ...S.tab, ...(activeTab === 'http' ? S.tabActive : S.tabInactive) }}
          onClick={() => setActiveTab('http')}
        >
          HTTP 分析
        </div>
        <div
          style={{ ...S.tab, ...(activeTab === 'tls' ? S.tabActive : S.tabInactive) }}
          onClick={() => setActiveTab('tls')}
        >
          TLS 分析
        </div>
      </div>

      {/* ── HTTP Tab ── */}
      {activeTab === 'http' && (
        <>
          {/* Summary Cards */}
          <div style={S.row}>
            <div style={S.card}>
              <div style={S.cardTitle}>总请求数</div>
              <div style={S.cardValue}>{fmtNum(httpSummary?.total_requests ?? 0)}</div>
            </div>
            <div style={S.card}>
              <div style={S.cardTitle}>总响应数</div>
              <div style={S.cardValue}>{fmtNum(httpSummary?.total_responses ?? 0)}</div>
            </div>
            <div style={S.card}>
              <div style={S.cardTitle}>唯一 Host 数</div>
              <div style={S.cardValue}>{httpSummary?.unique_hosts ?? 0}</div>
            </div>
          </div>

          {/* Top Hosts Bar Chart */}
          {top10Hosts.length > 0 && (
            <div style={S.section}>
              <div style={S.sectionTitle}>Top 10 HTTP Hosts</div>
              <Bar data={hostsBarData} options={barOpts} height={top10Hosts.length * 28 + 40} />
            </div>
          )}

          {/* Top HTTP Hosts Table */}
          <div style={S.section}>
            <div style={S.sectionTitle}>Top HTTP Hosts</div>
            <div style={{ overflowX: 'auto' }}>
              <table style={S.table}>
                <thead>
                  <tr>
                    <th style={S.th}>#</th>
                    <th style={S.th}>Host</th>
                    <th style={S.th}>请求数</th>
                    <th style={S.th}>入站流量</th>
                    <th style={S.th}>出站流量</th>
                  </tr>
                </thead>
                <tbody>
                  {httpHosts.map((h, i) => (
                    <tr key={h.host} style={{ background: i % 2 === 0 ? 'transparent' : `${theme.colors.border}22` }}>
                      <td style={S.td}>{i + 1}</td>
                      <td style={{ ...S.td, color: theme.colors.sentryPurple, fontWeight: 600, maxWidth: 300, wordBreak: 'break-all' }}>{h.host}</td>
                      <td style={{ ...S.td, fontWeight: 600 }}>{fmtNum(h.request_count)}</td>
                      <td style={S.td}>{fmtBytes(h.bytes_in)}</td>
                      <td style={S.td}>{fmtBytes(h.bytes_out)}</td>
                    </tr>
                  ))}
                  {httpHosts.length === 0 && (
                    <tr><td colSpan={5} style={{ ...S.td, textAlign: 'center', color: theme.colors.textDim }}>暂无 HTTP 数据，请先开始抓包</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>

          {/* Top User-Agents Table */}
          <div style={S.section}>
            <div style={S.sectionTitle}>Top User-Agents</div>
            <div style={{ overflowX: 'auto' }}>
              <table style={S.table}>
                <thead>
                  <tr>
                    <th style={S.th}>#</th>
                    <th style={S.th}>User-Agent</th>
                    <th style={S.th}>请求数</th>
                  </tr>
                </thead>
                <tbody>
                  {userAgents.map((ua, i) => (
                    <tr key={i} style={{ background: i % 2 === 0 ? 'transparent' : `${theme.colors.border}22` }}>
                      <td style={S.td}>{i + 1}</td>
                      <td style={{ ...S.td, maxWidth: 500, wordBreak: 'break-all', fontSize: 12 }}>{ua.user_agent}</td>
                      <td style={{ ...S.td, fontWeight: 600 }}>{fmtNum(ua.count)}</td>
                    </tr>
                  ))}
                  {userAgents.length === 0 && (
                    <tr><td colSpan={3} style={{ ...S.td, textAlign: 'center', color: theme.colors.textDim }}>暂无数据</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>

          {/* Charts Row: Methods + Status Codes */}
          <div style={{ ...S.row }}>
            <div style={{ ...S.section, flex: 1 }}>
              <div style={S.sectionTitle}>HTTP 方法分布</div>
              <div style={S.chartWrap}>
                {methodLabels.length > 0 ? <Doughnut data={methodChartData} options={doughnutOpts} /> : <div style={{ color: theme.colors.textDim, textAlign: 'center', padding: 40 }}>暂无数据</div>}
              </div>
            </div>
            <div style={{ ...S.section, flex: 1 }}>
              <div style={S.sectionTitle}>状态码分布</div>
              <div style={{ height: 200 }}>
                {codeLabels.length > 0 ? <Bar data={codeChartData} options={codeBarOpts} /> : <div style={{ color: theme.colors.textDim, textAlign: 'center', padding: 40 }}>暂无数据</div>}
              </div>
            </div>
          </div>
        </>
      )}

      {/* ── TLS Tab ── */}
      {activeTab === 'tls' && (
        <>
          {/* Summary Cards */}
          <div style={S.row}>
            <div style={S.card}>
              <div style={S.cardTitle}>总握手数</div>
              <div style={S.cardValue}>{fmtNum(tlsSummary?.total_handshakes ?? 0)}</div>
            </div>
            <div style={S.card}>
              <div style={S.cardTitle}>唯一 SNI 域名</div>
              <div style={S.cardValue}>{tlsSummary?.unique_sni ?? 0}</div>
            </div>
            <div style={S.card}>
              <div style={S.cardTitle}>唯一 JA3 指纹</div>
              <div style={S.cardValue}>{tlsSummary?.unique_ja3 ?? 0}</div>
            </div>
          </div>

          {/* Top SNI Domains Table */}
          <div style={S.section}>
            <div style={S.sectionTitle}>Top SNI 域名</div>
            <div style={{ overflowX: 'auto' }}>
              <table style={S.table}>
                <thead>
                  <tr>
                    <th style={S.th}>#</th>
                    <th style={S.th}>域名</th>
                    <th style={S.th}>握手次数</th>
                    <th style={S.th}>首次发现</th>
                    <th style={S.th}>最后发现</th>
                  </tr>
                </thead>
                <tbody>
                  {sniDomains.map((s, i) => (
                    <tr key={s.domain} style={{ background: i % 2 === 0 ? 'transparent' : `${theme.colors.border}22` }}>
                      <td style={S.td}>{i + 1}</td>
                      <td style={{ ...S.td, color: theme.colors.sentryPurple, fontWeight: 600, maxWidth: 300, wordBreak: 'break-all' }}>{s.domain}</td>
                      <td style={{ ...S.td, fontWeight: 600 }}>{fmtNum(s.count)}</td>
                      <td style={{ ...S.td, fontSize: 12 }}>{s.first_seen ? new Date(s.first_seen).toLocaleString() : '-'}</td>
                      <td style={{ ...S.td, fontSize: 12 }}>{s.last_seen ? new Date(s.last_seen).toLocaleString() : '-'}</td>
                    </tr>
                  ))}
                  {sniDomains.length === 0 && (
                    <tr><td colSpan={5} style={{ ...S.td, textAlign: 'center', color: theme.colors.textDim }}>暂无 TLS 数据，请先开始抓包</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>

          {/* Top JA3 Hashes Table */}
          <div style={S.section}>
            <div style={S.sectionTitle}>Top JA3 指纹</div>
            <div style={{ overflowX: 'auto' }}>
              <table style={S.table}>
                <thead>
                  <tr>
                    <th style={S.th}>#</th>
                    <th style={S.th}>JA3 Hash</th>
                    <th style={S.th}>出现次数</th>
                  </tr>
                </thead>
                <tbody>
                  {ja3Hashes.map((j, i) => (
                    <tr key={j.hash} style={{ background: i % 2 === 0 ? 'transparent' : `${theme.colors.border}22` }}>
                      <td style={S.td}>{i + 1}</td>
                      <td style={{ ...S.td, fontFamily: theme.typography.fontFamilyMono, color: theme.colors.sentryPurple }}>{j.hash}</td>
                      <td style={{ ...S.td, fontWeight: 600 }}>{fmtNum(j.count)}</td>
                    </tr>
                  ))}
                  {ja3Hashes.length === 0 && (
                    <tr><td colSpan={3} style={{ ...S.td, textAlign: 'center', color: theme.colors.textDim }}>暂无数据</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>

          {/* TLS Version Distribution */}
          <div style={S.section}>
            <div style={S.sectionTitle}>TLS 版本分布</div>
            <div style={S.chartWrap}>
              {versionLabels.length > 0 ? <Doughnut data={versionChartData} options={doughnutOpts} /> : <div style={{ color: theme.colors.textDim, textAlign: 'center', padding: 40 }}>暂无数据</div>}
            </div>
          </div>
        </>
      )}
    </div>
  )
}

export default HTTPAnalysis
