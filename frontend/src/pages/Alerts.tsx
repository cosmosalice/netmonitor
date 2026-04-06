import { useState, useEffect, useCallback } from 'react'
import {
  Chart, CategoryScale, LinearScale, BarElement, ArcElement, Title, Tooltip, Legend,
} from 'chart.js'
import { Doughnut, Bar } from 'react-chartjs-2'
import {
  getAlerts, acknowledgeAlert, resolveAlert, getAlertStats,
  getAlertRules, saveAlertRule, deleteAlertRule,
  getNotificationEndpoints, saveNotificationEndpoint, deleteNotificationEndpoint,
} from '../api/index'
import { theme } from '../theme'

Chart.register(CategoryScale, LinearScale, BarElement, ArcElement, Title, Tooltip, Legend)

// ── Colors ──
const C = {
  bg: theme.colors.bgPrimary, card: theme.colors.bgDeep, border: theme.colors.border, accent: theme.colors.sentryPurple,
  text: theme.colors.textPrimary, textW: '#ffffff', textDim: theme.colors.textDim,
  info: '#2196f3', warning: theme.colors.warning, error: theme.colors.error, critical: '#9c27b0',
}

const severityColor: Record<string, string> = {
  info: C.info, warning: C.warning, error: C.error, critical: C.critical,
}

function timeAgo(ts: string) {
  if (!ts) return '-'
  const d = new Date(ts)
  const s = Math.floor((Date.now() - d.getTime()) / 1000)
  if (s < 60) return `${s}秒前`
  if (s < 3600) return `${Math.floor(s / 60)}分钟前`
  if (s < 86400) return `${Math.floor(s / 3600)}小时前`
  return `${Math.floor(s / 86400)}天前`
}

// ── Styles ──
const S: Record<string, React.CSSProperties> = {
  page: { padding: 24, background: C.bg, minHeight: '100%', color: C.text, fontFamily: theme.typography.fontFamily },
  tabs: { display: 'flex', gap: 0, borderBottom: `2px solid ${C.border}`, marginBottom: 24 },
  tab: { padding: '10px 24px', cursor: 'pointer', fontSize: 14, fontWeight: 600, color: C.textDim, borderBottom: '2px solid transparent', marginBottom: -2, transition: 'all .2s' },
  tabActive: { color: C.accent, borderBottomColor: C.accent },
  card: { background: C.card, borderRadius: 12, padding: 20, border: `1px solid ${C.border}` },
  row: { display: 'flex', gap: 16, marginBottom: 16 },
  select: { background: C.card, color: C.text, border: `1px solid ${C.border}`, borderRadius: 6, padding: '6px 12px', fontSize: 13 },
  btn: { padding: '6px 14px', borderRadius: 6, border: 'none', cursor: 'pointer', fontSize: 12, fontWeight: 600 },
  btnPrimary: { background: theme.colors.sentryPurple, color: '#fff', textTransform: 'uppercase' as const, letterSpacing: '0.2px' },
  btnDanger: { background: theme.colors.error, color: '#fff', textTransform: 'uppercase' as const, letterSpacing: '0.2px' },
  btnSuccess: { background: theme.colors.success, color: '#000', textTransform: 'uppercase' as const, letterSpacing: '0.2px' },
  btnGhost: { background: 'transparent', color: C.textDim, border: `1px solid ${C.border}` },
  table: { width: '100%', borderCollapse: 'collapse' as const, fontSize: 13 },
  th: { textAlign: 'left' as const, padding: '10px 12px', borderBottom: `1px solid ${C.border}`, color: C.textDim, fontWeight: 600 },
  td: { padding: '10px 12px', borderBottom: '1px solid rgba(255,255,255,0.05)' },
  badge: { display: 'inline-block', padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 700 },
  input: { background: '#0d1b2a', color: C.text, border: `1px solid ${C.border}`, borderRadius: 6, padding: '8px 12px', fontSize: 13, width: '100%', boxSizing: 'border-box' as const },
  modalOverlay: { position: 'fixed' as const, top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000 },
  modalCard: { background: C.card, borderRadius: 12, padding: 24, width: 520, maxHeight: '80vh', overflowY: 'auto' as const, border: `1px solid ${C.border}` },
  formGroup: { marginBottom: 14 },
  label: { display: 'block', fontSize: 12, color: C.textDim, marginBottom: 4, fontWeight: 600 },
}

// ══════════════════════════════════════════
// Main Component
// ══════════════════════════════════════════

export default function Alerts() {
  const [tab, setTab] = useState(0)
  const tabs = ['告警列表', '告警统计', '规则管理', '通知配置']

  return (
    <div style={S.page}>
      <h2 style={{ margin: '0 0 20px', fontSize: 22, color: C.textW }}>告警管理</h2>
      <div style={S.tabs}>
        {tabs.map((t, i) => (
          <div key={i} style={{ ...S.tab, ...(tab === i ? S.tabActive : {}) }} onClick={() => setTab(i)}>{t}</div>
        ))}
      </div>
      {tab === 0 && <AlertList />}
      {tab === 1 && <AlertStatsTab />}
      {tab === 2 && <RuleManager />}
      {tab === 3 && <NotificationConfig />}
    </div>
  )
}

// ══════════════════════════════════════════
// Tab 1: Alert List
// ══════════════════════════════════════════

function AlertList() {
  const [alerts, setAlerts] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [filter, setFilter] = useState({ type: '', severity: '', status: '', limit: 50, offset: 0 })
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params: any = { limit: filter.limit, offset: filter.offset }
      if (filter.type) params.type = filter.type
      if (filter.severity) params.severity = filter.severity
      if (filter.status) params.status = filter.status
      const data = await getAlerts(params)
      setAlerts(data.alerts || [])
      setTotal(data.total || 0)
    } catch { /* ignore */ }
    setLoading(false)
  }, [filter])

  useEffect(() => { load() }, [load])

  const handleAck = async (id: number) => { await acknowledgeAlert(id); load() }
  const handleResolve = async (id: number) => { await resolveAlert(id); load() }

  const pageCount = Math.max(1, Math.ceil(total / filter.limit))
  const currentPage = Math.floor(filter.offset / filter.limit) + 1

  return (
    <div>
      {/* Filters */}
      <div style={S.row}>
        <select style={S.select} value={filter.type} onChange={e => setFilter(f => ({ ...f, type: e.target.value, offset: 0 }))}>
          <option value="">所有类型</option>
          <option value="flow">Flow</option>
          <option value="host">Host</option>
          <option value="interface">Interface</option>
          <option value="system">System</option>
        </select>
        <select style={S.select} value={filter.severity} onChange={e => setFilter(f => ({ ...f, severity: e.target.value, offset: 0 }))}>
          <option value="">所有级别</option>
          <option value="info">Info</option>
          <option value="warning">Warning</option>
          <option value="error">Error</option>
          <option value="critical">Critical</option>
        </select>
        <select style={S.select} value={filter.status} onChange={e => setFilter(f => ({ ...f, status: e.target.value, offset: 0 }))}>
          <option value="">所有状态</option>
          <option value="triggered">Triggered</option>
          <option value="acknowledged">Acknowledged</option>
          <option value="resolved">Resolved</option>
        </select>
        <button style={{ ...S.btn, ...S.btnGhost }} onClick={load}>{loading ? '加载中...' : '刷新'}</button>
      </div>

      {/* Table */}
      <div style={S.card}>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>ID</th><th style={S.th}>级别</th><th style={S.th}>类型</th>
              <th style={S.th}>标题</th><th style={S.th}>实体</th><th style={S.th}>触发时间</th>
              <th style={S.th}>状态</th><th style={S.th}>操作</th>
            </tr>
          </thead>
          <tbody>
            {alerts.length === 0 && (
              <tr><td colSpan={8} style={{ ...S.td, textAlign: 'center', color: C.textDim }}>暂无告警数据</td></tr>
            )}
            {alerts.map((a: any) => (
              <>
                <tr key={a.id} style={{ cursor: 'pointer' }} onClick={() => setExpandedId(expandedId === a.id ? null : a.id)}>
                  <td style={S.td}>{a.id}</td>
                  <td style={S.td}>
                    <span style={{ ...S.badge, background: severityColor[a.severity] || C.info, color: '#fff' }}>
                      {a.severity}
                    </span>
                  </td>
                  <td style={S.td}>{a.type}</td>
                  <td style={{ ...S.td, color: C.textW, fontWeight: 500 }}>{a.title}</td>
                  <td style={S.td}>{a.entity_id || '-'}</td>
                  <td style={S.td}>{timeAgo(a.triggered_at)}</td>
                  <td style={S.td}>
                    <span style={{
                      ...S.badge,
                      background: a.status === 'triggered' ? 'rgba(244,67,54,0.15)' : a.status === 'acknowledged' ? 'rgba(255,152,0,0.15)' : 'rgba(0,230,118,0.15)',
                      color: a.status === 'triggered' ? C.error : a.status === 'acknowledged' ? C.warning : theme.colors.success,
                    }}>{a.status}</span>
                  </td>
                  <td style={S.td} onClick={e => e.stopPropagation()}>
                    <div style={{ display: 'flex', gap: 6 }}>
                      {a.status === 'triggered' && (
                        <button style={{ ...S.btn, ...S.btnPrimary }} onClick={() => handleAck(a.id)}>确认</button>
                      )}
                      {(a.status === 'triggered' || a.status === 'acknowledged') && (
                        <button style={{ ...S.btn, ...S.btnSuccess }} onClick={() => handleResolve(a.id)}>解决</button>
                      )}
                    </div>
                  </td>
                </tr>
                {expandedId === a.id && (
                  <tr key={`${a.id}-detail`}>
                    <td colSpan={8} style={{ ...S.td, background: 'rgba(0,0,0,0.2)', padding: 16 }}>
                      <div style={{ fontSize: 12 }}>
                        <p style={{ margin: '0 0 6px', color: C.textDim }}>描述</p>
                        <p style={{ margin: '0 0 12px' }}>{a.description || '无'}</p>
                        <p style={{ margin: '0 0 6px', color: C.textDim }}>Metadata</p>
                        <pre style={{ margin: 0, fontSize: 11, color: C.textDim, whiteSpace: 'pre-wrap' }}>{a.metadata || '无'}</pre>
                      </div>
                    </td>
                  </tr>
                )}
              </>
            ))}
          </tbody>
        </table>

        {/* Pagination */}
        {total > filter.limit && (
          <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 12, marginTop: 16 }}>
            <button style={{ ...S.btn, ...S.btnGhost }} disabled={currentPage <= 1}
              onClick={() => setFilter(f => ({ ...f, offset: f.offset - f.limit }))}>上一页</button>
            <span style={{ fontSize: 13, color: C.textDim }}>{currentPage} / {pageCount}</span>
            <button style={{ ...S.btn, ...S.btnGhost }} disabled={currentPage >= pageCount}
              onClick={() => setFilter(f => ({ ...f, offset: f.offset + f.limit }))}>下一页</button>
          </div>
        )}
      </div>
    </div>
  )
}

// ══════════════════════════════════════════
// Tab 2: Alert Stats
// ══════════════════════════════════════════

function AlertStatsTab() {
  const [stats, setStats] = useState<any>(null)

  useEffect(() => {
    getAlertStats().then(setStats).catch(() => {})
  }, [])

  if (!stats) return <div style={{ color: C.textDim }}>加载中...</div>

  const severityData = {
    labels: Object.keys(stats.by_severity || {}),
    datasets: [{
      data: Object.values(stats.by_severity || {}) as number[],
      backgroundColor: Object.keys(stats.by_severity || {}).map(k => severityColor[k] || C.info),
      borderWidth: 0,
    }],
  }

  const typeData = {
    labels: Object.keys(stats.by_type || {}),
    datasets: [{
      label: '告警数',
      data: Object.values(stats.by_type || {}) as number[],
      backgroundColor: C.accent,
      borderRadius: 4,
    }],
  }

  const statusData = {
    labels: Object.keys(stats.by_status || {}),
    datasets: [{
      data: Object.values(stats.by_status || {}) as number[],
      backgroundColor: [theme.colors.error, theme.colors.warning, theme.colors.success],
      borderWidth: 0,
    }],
  }

  const chartOpts: any = {
    responsive: true, maintainAspectRatio: false,
    plugins: { legend: { labels: { color: C.textDim, font: { size: 11 } } } },
  }
  const barOpts: any = {
    ...chartOpts,
    scales: {
      x: { ticks: { color: C.textDim }, grid: { color: 'rgba(255,255,255,0.05)' } },
      y: { ticks: { color: C.textDim }, grid: { color: 'rgba(255,255,255,0.05)' } },
    },
  }

  const unresolved = (stats.by_status?.triggered || 0) + (stats.by_status?.acknowledged || 0)

  return (
    <div>
      {/* Summary cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 24 }}>
        <div style={S.card}>
          <div style={{ fontSize: 13, color: C.textDim }}>总告警数</div>
          <div style={{ fontSize: 32, fontWeight: 700, color: C.accent }}>{stats.total || 0}</div>
        </div>
        <div style={S.card}>
          <div style={{ fontSize: 13, color: C.textDim }}>最近 24 小时</div>
          <div style={{ fontSize: 32, fontWeight: 700, color: C.warning }}>{stats.last_24_hours || 0}</div>
        </div>
        <div style={S.card}>
          <div style={{ fontSize: 13, color: C.textDim }}>未处理</div>
          <div style={{ fontSize: 32, fontWeight: 700, color: C.error }}>{unresolved}</div>
        </div>
      </div>

      {/* Charts */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 16 }}>
        <div style={S.card}>
          <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 12 }}>按严重级别</div>
          <div style={{ height: 220 }}><Doughnut data={severityData} options={chartOpts} /></div>
        </div>
        <div style={S.card}>
          <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 12 }}>按类型</div>
          <div style={{ height: 220 }}><Bar data={typeData} options={barOpts} /></div>
        </div>
        <div style={S.card}>
          <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 12 }}>按状态</div>
          <div style={{ height: 220 }}><Doughnut data={statusData} options={chartOpts} /></div>
        </div>
      </div>
    </div>
  )
}

// ══════════════════════════════════════════
// Tab 3: Rule Manager
// ══════════════════════════════════════════

function RuleManager() {
  const [rules, setRules] = useState<any[]>([])
  const [showModal, setShowModal] = useState(false)
  const [editRule, setEditRule] = useState<any>(null)

  const load = useCallback(async () => {
    try {
      const data = await getAlertRules()
      setRules(data.rules || [])
    } catch { /* ignore */ }
  }, [])

  useEffect(() => { load() }, [load])

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除此规则？')) return
    await deleteAlertRule(id)
    load()
  }

  const handleToggle = async (rule: any) => {
    await saveAlertRule({ ...rule, enabled: !rule.enabled })
    load()
  }

  const handleSave = async (rule: any) => {
    await saveAlertRule(rule)
    setShowModal(false)
    setEditRule(null)
    load()
  }

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <button style={{ ...S.btn, ...S.btnPrimary }} onClick={() => { setEditRule(null); setShowModal(true) }}>+ 新建规则</button>
      </div>

      <div style={S.card}>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>名称</th><th style={S.th}>类型</th><th style={S.th}>级别</th>
              <th style={S.th}>条件</th><th style={S.th}>冷却(秒)</th><th style={S.th}>状态</th><th style={S.th}>操作</th>
            </tr>
          </thead>
          <tbody>
            {rules.length === 0 && (
              <tr><td colSpan={7} style={{ ...S.td, textAlign: 'center', color: C.textDim }}>暂无规则</td></tr>
            )}
            {rules.map((r: any) => (
              <tr key={r.id}>
                <td style={{ ...S.td, color: C.textW, fontWeight: 500 }}>{r.name}</td>
                <td style={S.td}>{r.type}</td>
                <td style={S.td}>
                  <span style={{ ...S.badge, background: severityColor[r.severity] || C.info, color: '#fff' }}>{r.severity}</span>
                </td>
                <td style={{ ...S.td, fontSize: 12, color: C.textDim }}>
                  {r.condition?.metric} {r.condition?.operator} {r.condition?.threshold}
                </td>
                <td style={S.td}>{r.cooldown_sec}</td>
                <td style={S.td}>
                  <div onClick={() => handleToggle(r)} style={{
                    width: 40, height: 20, borderRadius: 10, cursor: 'pointer',
                    background: r.enabled ? C.accent : C.border, position: 'relative', transition: 'background .2s',
                  }}>
                    <div style={{
                      width: 16, height: 16, borderRadius: '50%', background: '#fff', position: 'absolute', top: 2,
                      left: r.enabled ? 22 : 2, transition: 'left .2s',
                    }} />
                  </div>
                </td>
                <td style={S.td}>
                  <div style={{ display: 'flex', gap: 6 }}>
                    <button style={{ ...S.btn, ...S.btnGhost }} onClick={() => { setEditRule(r); setShowModal(true) }}>编辑</button>
                    <button style={{ ...S.btn, ...S.btnDanger }} onClick={() => handleDelete(r.id)}>删除</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showModal && <RuleModal rule={editRule} onSave={handleSave} onClose={() => { setShowModal(false); setEditRule(null) }} />}
    </div>
  )
}

function RuleModal({ rule, onSave, onClose }: { rule: any; onSave: (r: any) => void; onClose: () => void }) {
  const [form, setForm] = useState(() => rule ? { ...rule } : {
    id: 'rule_' + Date.now(), name: '', description: '', type: 'system', severity: 'warning', enabled: true,
    condition: { metric: 'bandwidth', operator: 'gt', threshold: 0, window_sec: 60 }, cooldown_sec: 300,
  })

  const update = (key: string, val: any) => setForm((f: any) => ({ ...f, [key]: val }))
  const updateCond = (key: string, val: any) => setForm((f: any) => ({ ...f, condition: { ...f.condition, [key]: val } }))

  return (
    <div style={S.modalOverlay} onClick={onClose}>
      <div style={S.modalCard} onClick={e => e.stopPropagation()}>
        <h3 style={{ margin: '0 0 20px', color: C.textW }}>{rule ? '编辑规则' : '新建规则'}</h3>
        <div style={S.formGroup}>
          <label style={S.label}>名称</label>
          <input style={S.input} value={form.name} onChange={e => update('name', e.target.value)} />
        </div>
        <div style={S.formGroup}>
          <label style={S.label}>描述</label>
          <input style={S.input} value={form.description} onChange={e => update('description', e.target.value)} />
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
          <div style={S.formGroup}>
            <label style={S.label}>类型</label>
            <select style={{ ...S.input }} value={form.type} onChange={e => update('type', e.target.value)}>
              <option value="flow">Flow</option><option value="host">Host</option>
              <option value="interface">Interface</option><option value="system">System</option>
            </select>
          </div>
          <div style={S.formGroup}>
            <label style={S.label}>严重级别</label>
            <select style={{ ...S.input }} value={form.severity} onChange={e => update('severity', e.target.value)}>
              <option value="info">Info</option><option value="warning">Warning</option>
              <option value="error">Error</option><option value="critical">Critical</option>
            </select>
          </div>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr', gap: 12 }}>
          <div style={S.formGroup}>
            <label style={S.label}>指标</label>
            <select style={{ ...S.input }} value={form.condition?.metric} onChange={e => updateCond('metric', e.target.value)}>
              <option value="bandwidth">Bandwidth</option><option value="connections">Connections</option>
              <option value="flows">Flows</option><option value="hosts">Hosts</option><option value="packets">Packets</option>
            </select>
          </div>
          <div style={S.formGroup}>
            <label style={S.label}>运算符</label>
            <select style={{ ...S.input }} value={form.condition?.operator} onChange={e => updateCond('operator', e.target.value)}>
              <option value="gt">&gt;</option><option value="gte">&gt;=</option>
              <option value="lt">&lt;</option><option value="lte">&lt;=</option><option value="eq">=</option>
            </select>
          </div>
          <div style={S.formGroup}>
            <label style={S.label}>阈值</label>
            <input style={S.input} type="number" value={form.condition?.threshold || 0} onChange={e => updateCond('threshold', Number(e.target.value))} />
          </div>
          <div style={S.formGroup}>
            <label style={S.label}>窗口(秒)</label>
            <input style={S.input} type="number" value={form.condition?.window_sec || 60} onChange={e => updateCond('window_sec', Number(e.target.value))} />
          </div>
        </div>
        <div style={S.formGroup}>
          <label style={S.label}>冷却时间(秒)</label>
          <input style={S.input} type="number" value={form.cooldown_sec || 300} onChange={e => update('cooldown_sec', Number(e.target.value))} />
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 20 }}>
          <button style={{ ...S.btn, ...S.btnGhost }} onClick={onClose}>取消</button>
          <button style={{ ...S.btn, ...S.btnPrimary }} onClick={() => onSave(form)}>保存</button>
        </div>
      </div>
    </div>
  )
}

// ══════════════════════════════════════════
// Tab 4: Notification Config
// ══════════════════════════════════════════

function NotificationConfig() {
  const [endpoints, setEndpoints] = useState<any[]>([])
  const [showModal, setShowModal] = useState(false)

  const load = useCallback(async () => {
    try {
      const data = await getNotificationEndpoints()
      setEndpoints(data.endpoints || [])
    } catch { /* ignore */ }
  }, [])

  useEffect(() => { load() }, [load])

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除此端点？')) return
    await deleteNotificationEndpoint(id)
    load()
  }

  const handleSave = async (ep: any) => {
    await saveNotificationEndpoint(ep)
    setShowModal(false)
    load()
  }

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <button style={{ ...S.btn, ...S.btnPrimary }} onClick={() => setShowModal(true)}>+ 新建端点</button>
      </div>

      <div style={S.card}>
        <table style={S.table}>
          <thead>
            <tr>
              <th style={S.th}>名称</th><th style={S.th}>类型</th><th style={S.th}>启用</th><th style={S.th}>操作</th>
            </tr>
          </thead>
          <tbody>
            {endpoints.length === 0 && (
              <tr><td colSpan={4} style={{ ...S.td, textAlign: 'center', color: C.textDim }}>暂无通知端点</td></tr>
            )}
            {endpoints.map((ep: any) => (
              <tr key={ep.id}>
                <td style={{ ...S.td, color: C.textW, fontWeight: 500 }}>{ep.name}</td>
                <td style={S.td}>{ep.type}</td>
                <td style={S.td}>
                  <span style={{ ...S.badge, background: ep.enabled ? 'rgba(194,239,78,0.15)' : 'rgba(255,255,255,0.05)', color: ep.enabled ? theme.colors.success : C.textDim }}>
                    {ep.enabled ? '启用' : '禁用'}
                  </span>
                </td>
                <td style={S.td}>
                  <button style={{ ...S.btn, ...S.btnDanger }} onClick={() => handleDelete(ep.id)}>删除</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showModal && <EndpointModal onSave={handleSave} onClose={() => setShowModal(false)} />}
    </div>
  )
}

function EndpointModal({ onSave, onClose }: { onSave: (ep: any) => void; onClose: () => void }) {
  const [form, setForm] = useState<any>({
    id: 'ep_' + Date.now(), name: '', type: 'email', enabled: true, config: {},
  })

  const update = (key: string, val: any) => setForm((f: any) => ({ ...f, [key]: val }))
  const updateConfig = (key: string, val: string) => setForm((f: any) => ({ ...f, config: { ...f.config, [key]: val } }))

  return (
    <div style={S.modalOverlay} onClick={onClose}>
      <div style={S.modalCard} onClick={e => e.stopPropagation()}>
        <h3 style={{ margin: '0 0 20px', color: C.textW }}>新建通知端点</h3>
        <div style={S.formGroup}>
          <label style={S.label}>名称</label>
          <input style={S.input} value={form.name} onChange={e => update('name', e.target.value)} />
        </div>
        <div style={S.formGroup}>
          <label style={S.label}>类型</label>
          <select style={{ ...S.input }} value={form.type} onChange={e => update('type', e.target.value)}>
            <option value="email">Email</option>
            <option value="webhook">Webhook</option>
          </select>
        </div>

        {form.type === 'email' && (
          <>
            <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 12 }}>
              <div style={S.formGroup}>
                <label style={S.label}>SMTP Host</label>
                <input style={S.input} value={form.config.smtp_host || ''} onChange={e => updateConfig('smtp_host', e.target.value)} />
              </div>
              <div style={S.formGroup}>
                <label style={S.label}>SMTP Port</label>
                <input style={S.input} value={form.config.smtp_port || ''} onChange={e => updateConfig('smtp_port', e.target.value)} />
              </div>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div style={S.formGroup}>
                <label style={S.label}>用户名</label>
                <input style={S.input} value={form.config.username || ''} onChange={e => updateConfig('username', e.target.value)} />
              </div>
              <div style={S.formGroup}>
                <label style={S.label}>密码</label>
                <input style={S.input} type="password" value={form.config.password || ''} onChange={e => updateConfig('password', e.target.value)} />
              </div>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div style={S.formGroup}>
                <label style={S.label}>发件人</label>
                <input style={S.input} value={form.config.from || ''} onChange={e => updateConfig('from', e.target.value)} />
              </div>
              <div style={S.formGroup}>
                <label style={S.label}>收件人</label>
                <input style={S.input} value={form.config.to || ''} onChange={e => updateConfig('to', e.target.value)} placeholder="多个用逗号分隔" />
              </div>
            </div>
            <div style={S.formGroup}>
              <label style={{ ...S.label, display: 'flex', alignItems: 'center', gap: 6 }}>
                <input type="checkbox" checked={form.config.tls === 'true'} onChange={e => updateConfig('tls', e.target.checked ? 'true' : 'false')} />
                启用 TLS
              </label>
            </div>
          </>
        )}

        {form.type === 'webhook' && (
          <>
            <div style={S.formGroup}>
              <label style={S.label}>URL</label>
              <input style={S.input} value={form.config.url || ''} onChange={e => updateConfig('url', e.target.value)} placeholder="https://..." />
            </div>
            <div style={S.formGroup}>
              <label style={S.label}>Secret (可选)</label>
              <input style={S.input} value={form.config.secret || ''} onChange={e => updateConfig('secret', e.target.value)} />
            </div>
            <div style={S.formGroup}>
              <label style={S.label}>Headers (JSON, 可选)</label>
              <input style={S.input} value={form.config.headers || ''} onChange={e => updateConfig('headers', e.target.value)} placeholder='{"Authorization":"Bearer ..."}' />
            </div>
          </>
        )}

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 20 }}>
          <button style={{ ...S.btn, ...S.btnGhost }} onClick={onClose}>取消</button>
          <button style={{ ...S.btn, ...S.btnPrimary }} onClick={() => onSave(form)}>保存</button>
        </div>
      </div>
    </div>
  )
}
