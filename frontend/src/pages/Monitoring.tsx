import React, { useState, useEffect } from 'react'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'react-chartjs-2'
import {
  getMonitoringProbes,
  createMonitoringProbe,
  deleteMonitoringProbe,
  getMonitoringProbe,
  testMonitoringProbe
} from '../api/index'
import { theme } from '../theme'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

interface MonitorResult {
  timestamp: string
  status: string
  latency_ms: number
  error?: string
}

interface MonitorTarget {
  id: string
  name: string
  type: string
  host: string
  port?: number
  url?: string
  interval_sec: number
  timeout_ms: number
  enabled: boolean
  status: string
}

interface ProbeDetail extends MonitorTarget {
  results?: MonitorResult[]
}

interface Summary {
  total: number
  up: number
  down: number
  unknown: number
}

const Monitoring: React.FC = () => {
  const [probes, setProbes] = useState<MonitorTarget[]>([])
  const [summary, setSummary] = useState<Summary>({ total: 0, up: 0, down: 0, unknown: 0 })
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [selectedProbe, setSelectedProbe] = useState<ProbeDetail | null>(null)
  const [formData, setFormData] = useState({
    name: '',
    type: 'ping',
    host: '',
    port: 0,
    url: '',
    interval: 60,
    timeout: 5000,
    enabled: true
  })

  const fetchData = async () => {
    try {
      const data = await getMonitoringProbes()
      setProbes(data.probes || [])
      setSummary(data.summary || { total: 0, up: 0, down: 0, unknown: 0 })
    } catch (error) {
      console.error('Failed to fetch probes:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    const timer = setInterval(fetchData, 10000)
    return () => clearInterval(timer)
  }, [])

  const handleCreateProbe = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await createMonitoringProbe({
        name: formData.name,
        type: formData.type,
        host: formData.type !== 'http' ? formData.host : undefined,
        port: formData.type === 'tcp' ? formData.port : undefined,
        url: formData.type === 'http' ? formData.url : undefined,
        interval: formData.interval,
        timeout: formData.timeout,
        enabled: formData.enabled
      })
      setShowModal(false)
      setFormData({
        name: '',
        type: 'ping',
        host: '',
        port: 0,
        url: '',
        interval: 60,
        timeout: 5000,
        enabled: true
      })
      fetchData()
    } catch (error) {
      console.error('Failed to create probe:', error)
      alert('Failed to create probe')
    }
  }

  const handleDeleteProbe = async (id: string) => {
    if (!window.confirm('Are you sure you want to delete this probe?')) return
    try {
      await deleteMonitoringProbe(id)
      if (selectedProbe?.id === id) setSelectedProbe(null)
      fetchData()
    } catch (error) {
      console.error('Failed to delete probe:', error)
    }
  }

  const handleViewProbe = async (id: string) => {
    try {
      const data = await getMonitoringProbe(id)
      setSelectedProbe({ ...data.probe, results: data.results || [] })
    } catch (error) {
      console.error('Failed to get probe details:', error)
    }
  }

  const handleTestProbe = async (id: string) => {
    try {
      await testMonitoringProbe(id)
      fetchData()
    } catch (error) {
      console.error('Failed to test probe:', error)
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'up': return theme.colors.success
      case 'down': return theme.colors.error
      default: return theme.colors.warning
    }
  }

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'ping': return '📡'
      case 'tcp': return '🔌'
      case 'http': return '🌐'
      default: return '❓'
    }
  }

  const getChartData = (results: MonitorResult[]) => {
    const labels = results.slice(-50).map(r => 
      new Date(r.timestamp).toLocaleTimeString()
    )
    const data = results.slice(-50).map(r => r.latency_ms)

    return {
      labels,
      datasets: [
        {
          label: 'Response Time (ms)',
          data,
          borderColor: theme.colors.sentryPurple,
          backgroundColor: 'rgba(106, 95, 193, 0.1)',
          fill: true,
          tension: 0.4,
        }
      ]
    }
  }

  const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: false
      }
    },
    scales: {
      x: {
        ticks: { color: theme.colors.textDim, maxTicksLimit: 8 },
        grid: { color: 'rgba(255,255,255,0.1)' }
      },
      y: {
        ticks: { color: theme.colors.textDim },
        grid: { color: 'rgba(255,255,255,0.1)' },
        beginAtZero: true
      }
    }
  }

  const calculateUptime = (results: MonitorResult[]) => {
    if (!results || results.length === 0) return 0
    const upCount = results.filter(r => r.status === 'up').length
    return ((upCount / results.length) * 100).toFixed(1)
  }

  const calculateAvgLatency = (results: MonitorResult[]) => {
    if (!results || results.length === 0) return 0
    const validResults = results.filter(r => r.latency_ms > 0)
    if (validResults.length === 0) return 0
    const sum = validResults.reduce((acc, r) => acc + r.latency_ms, 0)
    return (sum / validResults.length).toFixed(0)
  }

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <h1 style={styles.title}>主动监控</h1>
        <button style={styles.addButton} onClick={() => setShowModal(true)}>
          + 添加探针
        </button>
      </div>

      {/* Summary Cards */}
      <div style={styles.summaryCards}>
        <div style={{...styles.summaryCard, borderLeftColor: theme.colors.sentryPurple}}>
          <div style={styles.summaryValue}>{summary.total}</div>
          <div style={styles.summaryLabel}>总探针数</div>
        </div>
        <div style={{...styles.summaryCard, borderLeftColor: theme.colors.success}}>
          <div style={styles.summaryValue}>{summary.up}</div>
          <div style={styles.summaryLabel}>正常</div>
        </div>
        <div style={{...styles.summaryCard, borderLeftColor: theme.colors.error}}>
          <div style={styles.summaryValue}>{summary.down}</div>
          <div style={styles.summaryLabel}>异常</div>
        </div>
        <div style={{...styles.summaryCard, borderLeftColor: theme.colors.warning}}>
          <div style={styles.summaryValue}>
            {selectedProbe && selectedProbe.results ? calculateAvgLatency(selectedProbe.results) : 0}
          </div>
          <div style={styles.summaryLabel}>平均延迟 (ms)</div>
        </div>
      </div>

      <div style={styles.mainContent}>
        {/* Probe List */}
        <div style={styles.probeList}>
          {loading ? (
            <div style={styles.loading}>Loading...</div>
          ) : probes.length === 0 ? (
            <div style={styles.empty}>暂无监控探针，点击"添加探针"开始监控</div>
          ) : (
            probes.map((probe) => (
              <div 
                key={probe.id} 
                style={{
                  ...styles.probeCard,
                  borderColor: selectedProbe?.id === probe.id ? theme.colors.sentryPurple : theme.colors.border
                }}
                onClick={() => handleViewProbe(probe.id)}
              >
                <div style={styles.probeHeader}>
                  <div style={styles.probeInfo}>
                    <span style={styles.probeIcon}>{getTypeIcon(probe.type)}</span>
                    <div>
                      <div style={styles.probeName}>{probe.name}</div>
                      <div style={styles.probeTarget}>
                        {probe.type === 'http' ? probe.url : `${probe.host}${probe.port ? ':' + probe.port : ''}`}
                      </div>
                    </div>
                  </div>
                  <div style={{...styles.statusBadge, backgroundColor: getStatusColor(probe.status)}}>
                    {probe.status.toUpperCase()}
                  </div>
                </div>
                <div style={styles.probeMeta}>
                  <span>间隔: {probe.interval_sec}s</span>
                  <span>超时: {probe.timeout_ms}ms</span>
                </div>
                <div style={styles.probeActions}>
                  <button 
                    style={styles.actionButton}
                    onClick={(e) => { e.stopPropagation(); handleTestProbe(probe.id) }}
                  >
                    测试
                  </button>
                  <button 
                    style={{...styles.actionButton, backgroundColor: theme.colors.error}}
                    onClick={(e) => { e.stopPropagation(); handleDeleteProbe(probe.id) }}
                  >
                    删除
                  </button>
                </div>
              </div>
            ))
          )}
        </div>

        {/* Probe Detail */}
        {selectedProbe && selectedProbe.results && (
          <div style={styles.detailPanel}>
            <h2 style={styles.detailTitle}>{selectedProbe.name} - 详情</h2>
            
            <div style={styles.detailStats}>
              <div style={styles.detailStat}>
                <span style={styles.detailStatValue}>{calculateUptime(selectedProbe.results)}%</span>
                <span style={styles.detailStatLabel}>可用率</span>
              </div>
              <div style={styles.detailStat}>
                <span style={styles.detailStatValue}>{calculateAvgLatency(selectedProbe.results)} ms</span>
                <span style={styles.detailStatLabel}>平均响应</span>
              </div>
              <div style={styles.detailStat}>
                <span style={styles.detailStatValue}>{selectedProbe.results.length}</span>
                <span style={styles.detailStatLabel}>检测次数</span>
              </div>
            </div>

            <div style={styles.chartContainer}>
              <Line data={getChartData(selectedProbe.results)} options={chartOptions} />
            </div>

            <div style={styles.resultsList}>
              <h3 style={styles.resultsTitle}>最近检测结果</h3>
              {selectedProbe.results.slice(-10).reverse().map((result, index) => (
                <div key={index} style={styles.resultItem}>
                  <span style={{...styles.resultStatus, color: getStatusColor(result.status)}}>
                    {result.status.toUpperCase()}
                  </span>
                  <span style={styles.resultLatency}>{result.latency_ms.toFixed(0)} ms</span>
                  <span style={styles.resultTime}>
                    {new Date(result.timestamp).toLocaleString()}
                  </span>
                  {result.error && (
                    <span style={styles.resultError}>{result.error}</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Add Probe Modal */}
      {showModal && (
        <div style={styles.modalOverlay} onClick={() => setShowModal(false)}>
          <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
            <h2 style={styles.modalTitle}>添加监控探针</h2>
            <form onSubmit={handleCreateProbe}>
              <div style={styles.formGroup}>
                <label style={styles.label}>探针名称 *</label>
                <input
                  style={styles.input}
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({...formData, name: e.target.value})}
                  placeholder="e.g., Web Server Ping"
                  required
                />
              </div>
              <div style={styles.formGroup}>
                <label style={styles.label}>类型 *</label>
                <select
                  style={styles.select}
                  value={formData.type}
                  onChange={(e) => setFormData({...formData, type: e.target.value})}
                >
                  <option value="ping">Ping (ICMP/TCP)</option>
                  <option value="tcp">TCP 端口检测</option>
                  <option value="http">HTTP(S) 检测</option>
                </select>
              </div>
              
              {formData.type !== 'http' && (
                <div style={styles.formGroup}>
                  <label style={styles.label}>目标主机 *</label>
                  <input
                    style={styles.input}
                    type="text"
                    value={formData.host}
                    onChange={(e) => setFormData({...formData, host: e.target.value})}
                    placeholder="e.g., 192.168.1.1 or example.com"
                    required
                  />
                </div>
              )}

              {formData.type === 'tcp' && (
                <div style={styles.formGroup}>
                  <label style={styles.label}>端口 *</label>
                  <input
                    style={styles.input}
                    type="number"
                    value={formData.port}
                    onChange={(e) => setFormData({...formData, port: parseInt(e.target.value) || 0})}
                    placeholder="e.g., 80"
                    required
                  />
                </div>
              )}

              {formData.type === 'http' && (
                <div style={styles.formGroup}>
                  <label style={styles.label}>URL *</label>
                  <input
                    style={styles.input}
                    type="text"
                    value={formData.url}
                    onChange={(e) => setFormData({...formData, url: e.target.value})}
                    placeholder="e.g., https://example.com"
                    required
                  />
                </div>
              )}

              <div style={styles.formRow}>
                <div style={styles.formGroup}>
                  <label style={styles.label}>检测间隔 (秒)</label>
                  <input
                    style={styles.input}
                    type="number"
                    value={formData.interval}
                    onChange={(e) => setFormData({...formData, interval: parseInt(e.target.value) || 60})}
                  />
                </div>
                <div style={styles.formGroup}>
                  <label style={styles.label}>超时 (毫秒)</label>
                  <input
                    style={styles.input}
                    type="number"
                    value={formData.timeout}
                    onChange={(e) => setFormData({...formData, timeout: parseInt(e.target.value) || 5000})}
                  />
                </div>
              </div>

              <div style={styles.formGroup}>
                <label style={styles.checkboxLabel}>
                  <input
                    type="checkbox"
                    checked={formData.enabled}
                    onChange={(e) => setFormData({...formData, enabled: e.target.checked})}
                  />
                  启用探针
                </label>
              </div>

              <div style={styles.modalActions}>
                <button type="button" style={styles.cancelButton} onClick={() => setShowModal(false)}>
                  取消
                </button>
                <button type="submit" style={styles.submitButton}>
                  创建
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    padding: '24px',
    backgroundColor: theme.colors.bgPrimary,
    minHeight: '100vh',
    color: theme.colors.textPrimary,
    fontFamily: theme.typography.fontFamily,
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '24px',
  },
  title: {
    fontSize: '28px',
    fontWeight: 600,
    margin: 0,
    color: theme.colors.sentryPurple,
  },
  addButton: {
    padding: '10px 20px',
    backgroundColor: theme.colors.deepViolet,
    color: theme.colors.sentryPurple,
    border: `1px solid ${theme.colors.sentryPurple}`,
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '14px',
    fontWeight: 500,
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  summaryCards: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
    gap: '16px',
    marginBottom: '24px',
  },
  summaryCard: {
    backgroundColor: theme.colors.bgDeep,
    borderRadius: '8px',
    padding: '20px',
    borderLeft: '4px solid',
  },
  summaryValue: {
    fontSize: '32px',
    fontWeight: 700,
    color: theme.colors.textPrimary,
  },
  summaryLabel: {
    fontSize: '14px',
    color: theme.colors.textDim,
    marginTop: '4px',
  },
  mainContent: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '24px',
  },
  probeList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
  },
  loading: {
    textAlign: 'center',
    padding: '40px',
    color: theme.colors.textDim,
  },
  empty: {
    textAlign: 'center',
    padding: '40px',
    color: theme.colors.textDim,
    fontSize: '16px',
    backgroundColor: theme.colors.bgDeep,
    borderRadius: '12px',
  },
  probeCard: {
    backgroundColor: theme.colors.bgDeep,
    borderRadius: '8px',
    padding: '16px',
    border: '1px solid',
    cursor: 'pointer',
    transition: 'border-color 0.2s',
  },
  probeHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '12px',
  },
  probeInfo: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
  },
  probeIcon: {
    fontSize: '24px',
  },
  probeName: {
    fontSize: '16px',
    fontWeight: 600,
  },
  probeTarget: {
    fontSize: '12px',
    color: theme.colors.textDim,
  },
  statusBadge: {
    padding: '4px 12px',
    borderRadius: '12px',
    fontSize: '11px',
    fontWeight: 600,
    color: theme.colors.textPrimary,
  },
  probeMeta: {
    display: 'flex',
    gap: '16px',
    fontSize: '12px',
    color: theme.colors.textDim,
    marginBottom: '12px',
  },
  probeActions: {
    display: 'flex',
    gap: '8px',
  },
  actionButton: {
    padding: '6px 12px',
    backgroundColor: theme.colors.deepViolet,
    color: theme.colors.textPrimary,
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '12px',
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  detailPanel: {
    backgroundColor: theme.colors.bgDeep,
    borderRadius: '12px',
    padding: '24px',
  },
  detailTitle: {
    fontSize: '20px',
    fontWeight: 600,
    marginBottom: '20px',
    color: theme.colors.sentryPurple,
  },
  detailStats: {
    display: 'grid',
    gridTemplateColumns: 'repeat(3, 1fr)',
    gap: '16px',
    marginBottom: '24px',
  },
  detailStat: {
    textAlign: 'center',
  },
  detailStatValue: {
    display: 'block',
    fontSize: '24px',
    fontWeight: 700,
    color: theme.colors.sentryPurple,
  },
  detailStatLabel: {
    fontSize: '12px',
    color: theme.colors.textDim,
  },
  chartContainer: {
    height: '200px',
    marginBottom: '24px',
  },
  resultsList: {
    maxHeight: '300px',
    overflowY: 'auto',
  },
  resultsTitle: {
    fontSize: '14px',
    fontWeight: 600,
    marginBottom: '12px',
    color: theme.colors.textDim,
  },
  resultItem: {
    display: 'grid',
    gridTemplateColumns: '60px 80px 1fr auto',
    gap: '12px',
    padding: '8px 0',
    borderBottom: `1px solid ${theme.colors.border}`,
    fontSize: '12px',
    alignItems: 'center',
  },
  resultStatus: {
    fontWeight: 600,
  },
  resultLatency: {
    color: theme.colors.sentryPurple,
  },
  resultTime: {
    color: theme.colors.textDim,
  },
  resultError: {
    color: theme.colors.error,
    fontSize: '11px',
    gridColumn: '1 / -1',
  },
  modalOverlay: {
    position: 'fixed',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: 'rgba(0, 0, 0, 0.7)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 1000,
  },
  modal: {
    backgroundColor: theme.colors.bgDeep,
    borderRadius: '12px',
    padding: '24px',
    width: '400px',
    maxWidth: '90vw',
    maxHeight: '90vh',
    overflowY: 'auto',
  },
  modalTitle: {
    fontSize: '20px',
    fontWeight: 600,
    marginBottom: '20px',
    color: theme.colors.sentryPurple,
  },
  formGroup: {
    marginBottom: '16px',
  },
  formRow: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '16px',
  },
  label: {
    display: 'block',
    marginBottom: '6px',
    fontSize: '13px',
    color: theme.colors.textDim,
  },
  input: {
    width: '100%',
    padding: '10px 12px',
    backgroundColor: theme.colors.bgPrimary,
    border: `1px solid ${theme.colors.border}`,
    borderRadius: '6px',
    color: theme.colors.textPrimary,
    fontSize: '14px',
    boxSizing: 'border-box',
  },
  select: {
    width: '100%',
    padding: '10px 12px',
    backgroundColor: theme.colors.bgPrimary,
    border: `1px solid ${theme.colors.border}`,
    borderRadius: '6px',
    color: theme.colors.textPrimary,
    fontSize: '14px',
    boxSizing: 'border-box',
  },
  checkboxLabel: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    cursor: 'pointer',
    fontSize: '14px',
  },
  modalActions: {
    display: 'flex',
    justifyContent: 'flex-end',
    gap: '12px',
    marginTop: '24px',
  },
  cancelButton: {
    padding: '10px 20px',
    backgroundColor: 'transparent',
    color: theme.colors.textDim,
    border: `1px solid ${theme.colors.textDim}`,
    borderRadius: '6px',
    cursor: 'pointer',
  },
  submitButton: {
    padding: '10px 20px',
    backgroundColor: theme.colors.sentryPurple,
    color: theme.colors.textPrimary,
    border: 'none',
    borderRadius: '6px',
    cursor: 'pointer',
    fontWeight: 600,
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
}

export default Monitoring
