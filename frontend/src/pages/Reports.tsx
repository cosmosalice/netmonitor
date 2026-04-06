import React, { useState, useEffect, useCallback } from 'react'
import { getReports, downloadReport, generateReport, getReportConfigs, saveReportConfig } from '../api/index'
import { theme } from '../theme'

const API_BASE = 'http://localhost:8080/api/v1'

const typeLabels: Record<string, string> = { daily: '日报', weekly: '周报', monthly: '月报' }

function formatSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return (bytes / 1024 / 1024).toFixed(2) + ' MB'
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return bytes + ' B'
}

const Reports: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'list' | 'config'>('list')
  const [reports, setReports] = useState<any[]>([])
  const [configs, setConfigs] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [genType, setGenType] = useState('daily')
  const [genDate, setGenDate] = useState(() => {
    const d = new Date(); d.setDate(d.getDate() - 1)
    return d.toISOString().split('T')[0]
  })
  const [previewHtml, setPreviewHtml] = useState<string | null>(null)
  const [error, setError] = useState('')

  const fetchReports = useCallback(async () => {
    setLoading(true)
    try {
      const data = await getReports()
      setReports(data.reports || [])
    } catch { setError('加载报表列表失败') }
    setLoading(false)
  }, [])

  const fetchConfigs = useCallback(async () => {
    try {
      const data = await getReportConfigs()
      setConfigs(data.configs || [])
    } catch { setError('加载报表配置失败') }
  }, [])

  useEffect(() => {
    fetchReports()
    fetchConfigs()
  }, [fetchReports, fetchConfigs])

  const handleGenerate = async () => {
    setGenerating(true); setError('')
    try {
      await generateReport(genType, genDate)
      fetchReports()
    } catch (e: any) {
      setError(e?.response?.data?.error || '生成报表失败')
    }
    setGenerating(false)
  }

  const handlePreview = async (id: string) => {
    try {
      const html = await downloadReport(id)
      setPreviewHtml(html)
    } catch { setError('预览报表失败') }
  }

  const handleToggle = async (cfg: any) => {
    try {
      await saveReportConfig({ ...cfg, enabled: !cfg.enabled })
      fetchConfigs()
    } catch { setError('保存配置失败') }
  }

  return (
    <div style={{ padding: 24, color: theme.colors.textPrimary, fontFamily: theme.typography.fontFamily, background: theme.colors.bgPrimary, minHeight: '100%' }}>
      <h1 style={{ color: theme.colors.sentryPurple, marginBottom: 20, fontSize: 22 }}>📋 报表管理</h1>

      {error && <div style={{ background: '#ff444433', border: '1px solid #ff4444', borderRadius: 8, padding: '8px 16px', marginBottom: 16, color: theme.colors.error }}>{error}<button onClick={() => setError('')} style={{ float: 'right', background: 'none', border: 'none', color: theme.colors.error, cursor: 'pointer' }}>✕</button></div>}

      {/* Tabs */}
      <div style={{ display: 'flex', gap: 0, marginBottom: 20, borderBottom: `2px solid ${theme.colors.border}` }}>
        {(['list', 'config'] as const).map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            style={{
              padding: '10px 24px', background: activeTab === tab ? theme.colors.deepViolet : 'transparent',
              color: activeTab === tab ? theme.colors.sentryPurple : theme.colors.textDim, border: 'none', cursor: 'pointer',
              borderBottom: activeTab === tab ? `2px solid ${theme.colors.sentryPurple}` : '2px solid transparent',
              fontSize: 14, fontWeight: 600, marginBottom: -2, textTransform: 'uppercase', letterSpacing: '0.2px',
            }}>
            {tab === 'list' ? '报表列表' : '报表配置'}
          </button>
        ))}
      </div>

      {activeTab === 'list' && (
        <>
          {/* Generate controls */}
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginBottom: 20, background: theme.colors.bgDeep, padding: 16, borderRadius: 8, border: `1px solid ${theme.colors.border}` }}>
            <span style={{ color: theme.colors.textDim, fontSize: 13 }}>手动生成:</span>
            <select value={genType} onChange={e => setGenType(e.target.value)}
              style={{ background: theme.colors.bgPrimary, color: theme.colors.textPrimary, border: `1px solid ${theme.colors.border}`, borderRadius: 6, padding: '6px 12px', fontSize: 13 }}>
              <option value="daily">日报</option>
              <option value="weekly">周报</option>
              <option value="monthly">月报</option>
            </select>
            <input type="date" value={genDate} onChange={e => setGenDate(e.target.value)}
              style={{ background: theme.colors.bgPrimary, color: theme.colors.textPrimary, border: `1px solid ${theme.colors.border}`, borderRadius: 6, padding: '6px 12px', fontSize: 13 }} />
            <button onClick={handleGenerate} disabled={generating}
              style={{ background: generating ? '#555' : theme.colors.deepViolet, color: theme.colors.sentryPurple, border: `1px solid ${theme.colors.sentryPurple}40`, borderRadius: 6, padding: '6px 20px', cursor: generating ? 'wait' : 'pointer', fontSize: 13, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.2px' }}>
              {generating ? '生成中...' : '生成报表'}
            </button>
            <button onClick={fetchReports} style={{ background: theme.colors.bgPrimary, color: theme.colors.textDim, border: `1px solid ${theme.colors.border}60`, borderRadius: 6, padding: '6px 14px', cursor: 'pointer', fontSize: 13 }}>刷新</button>
          </div>

          {/* Reports table */}
          {loading ? <div style={{ textAlign: 'center', padding: 40, color: theme.colors.textDim }}>加载中...</div> : (
            <div style={{ background: theme.colors.bgDeep, borderRadius: 8, border: `1px solid ${theme.colors.border}`, overflow: 'hidden' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ background: theme.colors.bgPrimary }}>
                    {['名称', '类型', '时段', '生成时间', '大小', '操作'].map(h => (
                      <th key={h} style={{ padding: '10px 14px', textAlign: 'left', color: theme.colors.sentryPurple, fontSize: 13, borderBottom: `2px solid ${theme.colors.border}` }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {reports.length === 0 ? (
                    <tr><td colSpan={6} style={{ padding: 30, textAlign: 'center', color: theme.colors.textDim }}>暂无报表，点击上方"生成报表"创建</td></tr>
                  ) : reports.map((r: any) => (
                    <tr key={r.id} style={{ borderBottom: `1px solid ${theme.colors.border}40` }}>
                      <td style={{ padding: '10px 14px', fontSize: 13 }}>{r.name}</td>
                      <td style={{ padding: '10px 14px', fontSize: 13 }}>
                        <span style={{ background: `${theme.colors.deepViolet}60`, padding: '2px 10px', borderRadius: 4, fontSize: 12 }}>{typeLabels[r.type] || r.type}</span>
                      </td>
                      <td style={{ padding: '10px 14px', fontSize: 13, fontFamily: theme.typography.fontFamilyMono }}>{r.period}</td>
                      <td style={{ padding: '10px 14px', fontSize: 13, color: theme.colors.textDim }}>{new Date(r.generated_at).toLocaleString()}</td>
                      <td style={{ padding: '10px 14px', fontSize: 13, color: theme.colors.textDim }}>{formatSize(r.file_size)}</td>
                      <td style={{ padding: '10px 14px', display: 'flex', gap: 8 }}>
                        <button onClick={() => handlePreview(r.id)}
                          style={{ background: theme.colors.deepViolet, color: theme.colors.sentryPurple, border: 'none', borderRadius: 4, padding: '4px 12px', cursor: 'pointer', fontSize: 12, textTransform: 'uppercase', letterSpacing: '0.2px' }}>查看</button>
                        <a href={`${API_BASE}/reports/${encodeURIComponent(r.id)}/download`} target="_blank" rel="noreferrer"
                          style={{ background: theme.colors.bgPrimary, color: theme.colors.textDim, border: `1px solid ${theme.colors.border}60`, borderRadius: 4, padding: '4px 12px', cursor: 'pointer', fontSize: 12, textDecoration: 'none', display: 'inline-block' }}>下载</a>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {activeTab === 'config' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {configs.map((cfg: any) => (
            <div key={cfg.id} style={{ background: theme.colors.bgDeep, borderRadius: 8, border: `1px solid ${theme.colors.border}`, padding: 20, display: 'flex', alignItems: 'center', gap: 20 }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 16, fontWeight: 600, color: cfg.enabled ? theme.colors.sentryPurple : theme.colors.textDim }}>{cfg.name || typeLabels[cfg.type] || cfg.type}</div>
                <div style={{ fontSize: 12, color: theme.colors.textDim, marginTop: 4 }}>
                  类型: {typeLabels[cfg.type] || cfg.type} | 输出目录: {cfg.output_dir || 'reports'}
                  {cfg.last_gen_time && <> | 上次生成: {new Date(cfg.last_gen_time).toLocaleString()}</>}
                </div>
              </div>
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
                <span style={{ fontSize: 13, color: theme.colors.textDim }}>{cfg.enabled ? '已启用' : '已禁用'}</span>
                <div onClick={() => handleToggle(cfg)}
                  style={{
                    width: 44, height: 24, borderRadius: 12, position: 'relative', cursor: 'pointer',
                    background: cfg.enabled ? theme.colors.sentryPurple : '#333', transition: 'background .2s',
                  }}>
                  <div style={{
                    width: 20, height: 20, borderRadius: 10, background: '#fff', position: 'absolute', top: 2,
                    left: cfg.enabled ? 22 : 2, transition: 'left .2s',
                  }} />
                </div>
              </label>
            </div>
          ))}
          {configs.length === 0 && <div style={{ textAlign: 'center', padding: 40, color: theme.colors.textDim }}>暂无配置</div>}
        </div>
      )}

      {/* Preview modal */}
      {previewHtml && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.8)', zIndex: 9999, display: 'flex', alignItems: 'center', justifyContent: 'center' }}
          onClick={() => setPreviewHtml(null)}>
          <div style={{ width: '90%', height: '90%', background: theme.colors.bgPrimary, borderRadius: 12, border: `1px solid ${theme.colors.border}`, overflow: 'hidden', position: 'relative' }}
            onClick={e => e.stopPropagation()}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '10px 16px', background: theme.colors.bgDeep, borderBottom: `1px solid ${theme.colors.border}` }}>
              <span style={{ color: theme.colors.sentryPurple, fontWeight: 600 }}>报表预览</span>
              <button onClick={() => setPreviewHtml(null)}
                style={{ background: '#ff444433', color: theme.colors.error, border: 'none', borderRadius: 4, padding: '4px 12px', cursor: 'pointer' }}>关闭</button>
            </div>
            <iframe srcDoc={previewHtml} style={{ width: '100%', height: 'calc(100% - 44px)', border: 'none' }} title="report-preview" />
          </div>
        </div>
      )}
    </div>
  )
}

export default Reports
