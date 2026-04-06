import React, { useState, useEffect, useCallback } from 'react'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js'
import { Line, Pie, Bar } from 'react-chartjs-2'
import {
  getDashboards,
  getDashboard,
  createDashboard,
  updateDashboard,
  deleteDashboard,
  getSummaryStats,
  getHostStats,
  getProtocolStats,
  getAlerts,
  getTimeseries,
} from '../api'
import { theme } from '../theme'
import './CustomDashboard.css'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend
)

interface WidgetPosition {
  x: number
  y: number
  w: number
  h: number
}

interface WidgetConfig {
  id: string
  type: 'stat' | 'line' | 'pie' | 'bar' | 'table'
  title: string
  data_source: string
  position: WidgetPosition
  config: any
}

interface Dashboard {
  id: string
  name: string
  widgets: WidgetConfig[]
  is_default: boolean
  created_at: string
  updated_at: string
}

const defaultTemplates = [
  { id: 'overview', name: '概览' },
  { id: 'security', name: '安全' },
  { id: 'performance', name: '性能' },
]

const CustomDashboard: React.FC = () => {
  const [dashboards, setDashboards] = useState<Dashboard[]>([])
  const [currentDashboard, setCurrentDashboard] = useState<Dashboard | null>(null)
  const [loading, setLoading] = useState(true)
  const [editMode, setEditMode] = useState(false)
  const [widgetData, setWidgetData] = useState<Record<string, any>>({})
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [newDashboardName, setNewDashboardName] = useState('')
  const [selectedTemplate, setSelectedTemplate] = useState('overview')

  const loadDashboards = useCallback(async () => {
    try {
      const data = await getDashboards()
      setDashboards(data.dashboards || [])
      if (data.dashboards?.length > 0 && !currentDashboard) {
        const defaultDash = data.dashboards.find((d: Dashboard) => d.is_default) || data.dashboards[0]
        loadDashboard(defaultDash.id)
      }
    } catch (err) {
      console.error('Failed to load dashboards:', err)
    }
  }, [currentDashboard])

  const loadDashboard = async (id: string) => {
    try {
      setLoading(true)
      const data = await getDashboard(id)
      setCurrentDashboard(data)
      fetchWidgetData(data.widgets)
    } catch (err) {
      console.error('Failed to load dashboard:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchWidgetData = async (widgets: WidgetConfig[]) => {
    const data: Record<string, any> = {}
    
    for (const widget of widgets) {
      try {
        switch (widget.data_source) {
          case '/api/v1/stats/summary':
            data[widget.id] = await getSummaryStats()
            break
          case '/api/v1/stats/hosts':
            data[widget.id] = await getHostStats(widget.config?.limit || 10)
            break
          case '/api/v1/stats/protocols':
            data[widget.id] = await getProtocolStats()
            break
          case '/api/v1/alerts':
          case '/api/v1/alerts/stats':
            data[widget.id] = await getAlerts({ limit: widget.config?.limit || 10 })
            break
          case '/api/v1/timeseries':
            const end = new Date().toISOString()
            const start = new Date(Date.now() - 3600000).toISOString()
            data[widget.id] = await getTimeseries(widget.config?.type || 'bandwidth', start, end)
            break
          default:
            data[widget.id] = null
        }
      } catch (err) {
        console.error(`Failed to load widget ${widget.id}:`, err)
        data[widget.id] = null
      }
    }
    
    setWidgetData(data)
  }

  useEffect(() => {
    loadDashboards()
    const interval = setInterval(() => {
      if (currentDashboard) {
        fetchWidgetData(currentDashboard.widgets)
      }
    }, 30000)
    return () => clearInterval(interval)
  }, [])

  const handleCreateDashboard = async () => {
    if (!newDashboardName.trim()) return
    
    try {
      const id = `custom-${Date.now()}`
      
      // Find selected template to validate it exists
      const templateExists = defaultTemplates.find(t => t.id === selectedTemplate)
      if (!templateExists) {
        console.error('Template not found')
        return
      }
      
      await createDashboard({
        id,
        name: newDashboardName,
        template: selectedTemplate,
      })
      
      setShowCreateModal(false)
      setNewDashboardName('')
      await loadDashboards()
      loadDashboard(id)
    } catch (err) {
      console.error('Failed to create dashboard:', err)
    }
  }

  const handleDeleteDashboard = async () => {
    if (!currentDashboard || currentDashboard.is_default) return
    if (!confirm(`确定要删除 "${currentDashboard.name}" 吗？`)) return
    
    try {
      await deleteDashboard(currentDashboard.id)
      setCurrentDashboard(null)
      await loadDashboards()
    } catch (err) {
      console.error('Failed to delete dashboard:', err)
    }
  }

  const handleSaveLayout = async () => {
    if (!currentDashboard) return
    
    try {
      await updateDashboard(currentDashboard.id, {
        name: currentDashboard.name,
        widgets: currentDashboard.widgets,
      })
      setEditMode(false)
    } catch (err) {
      console.error('Failed to save dashboard:', err)
    }
  }

  const renderWidget = (widget: WidgetConfig) => {
    const data = widgetData[widget.id]
    
    switch (widget.type) {
      case 'stat':
        return renderStatWidget(widget, data)
      case 'line':
        return renderLineWidget(widget, data)
      case 'pie':
        return renderPieWidget(widget, data)
      case 'bar':
        return renderBarWidget(widget, data)
      case 'table':
        return renderTableWidget(widget, data)
      default:
        return <div className="widget-placeholder">Unknown widget type</div>
    }
  }

  const renderStatWidget = (widget: WidgetConfig, data: any) => {
    let value = '-'
    let label = widget.title
    
    if (data) {
      switch (widget.config?.metric) {
        case 'total_bytes':
          value = formatBytes(data.total_bytes || 0)
          break
        case 'active_hosts':
          value = (data.active_hosts || 0).toString()
          break
        case 'active_flows':
          value = (data.active_flows || 0).toString()
          break
        case 'total':
          value = (data.total || data.by_status?.triggered || 0).toString()
          break
        case 'packets_per_sec':
          value = (data.bandwidth?.packets_per_sec || 0).toFixed(0)
          break
        case 'bytes_per_sec':
          value = formatBytes(data.bandwidth?.bytes_per_sec || 0) + '/s'
          break
        default:
          value = '-'
      }
    }
    
    return (
      <div className="stat-widget">
        <div className="stat-value">{value}</div>
        <div className="stat-label">{label}</div>
      </div>
    )
  }

  const renderLineWidget = (widget: WidgetConfig, data: any) => {
    if (!data?.data || data.data.length === 0) {
      return <div className="widget-no-data">暂无数据</div>
    }
    
    const chartData = {
      labels: data.data.map((d: any) => new Date(d.timestamp).toLocaleTimeString()),
      datasets: [
        {
          label: widget.title,
          data: data.data.map((d: any) => d.value),
          borderColor: theme.colors.sentryPurple,
          backgroundColor: 'rgba(106, 95, 193, 0.1)',
          fill: true,
          tension: 0.4,
        },
      ],
    }
    
    const options = {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
      },
      scales: {
        x: { display: false },
        y: {
          grid: { color: 'rgba(255, 255, 255, 0.05)' },
          ticks: { color: theme.colors.textDim, font: { size: 10 } },
        },
      },
    }
    
    return <Line data={chartData} options={options} />
  }

  const renderPieWidget = (widget: WidgetConfig, data: any) => {
    if (!data?.protocols || data.protocols.length === 0) {
      return <div className="widget-no-data">暂无数据</div>
    }
    
    const limit = widget.config?.limit || 10
    const protocols = data.protocols.slice(0, limit)
    const colors = [theme.colors.sentryPurple, theme.colors.lime, theme.colors.warning, theme.colors.error, theme.colors.pink, theme.colors.assyrian, theme.colors.deepViolet, '#ff6384']
    
    const chartData = {
      labels: protocols.map((p: any) => p.name),
      datasets: [
        {
          data: protocols.map((p: any) => p.bytes),
          backgroundColor: colors,
          borderWidth: 0,
        },
      ],
    }
    
    const options = {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: {
          position: 'right' as const,
          labels: { color: theme.colors.textDim, font: { size: 11 }, boxWidth: 12 },
        },
      },
    }
    
    return <Pie data={chartData} options={options} />
  }

  const renderBarWidget = (widget: WidgetConfig, data: any) => {
    if (!data?.hosts || data.hosts.length === 0) {
      return <div className="widget-no-data">暂无数据</div>
    }
    
    const hosts = data.hosts.slice(0, widget.config?.limit || 10)
    
    const chartData = {
      labels: hosts.map((h: any) => h.ip),
      datasets: [
        {
          label: 'Bytes',
          data: hosts.map((h: any) => h.bytes_sent + h.bytes_recv),
          backgroundColor: 'rgba(106, 95, 193, 0.6)',
          borderColor: theme.colors.sentryPurple,
          borderWidth: 1,
        },
      ],
    }
    
    const options = {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
      },
      scales: {
        x: {
          ticks: { color: theme.colors.textDim, font: { size: 10 } },
          grid: { display: false },
        },
        y: {
          grid: { color: 'rgba(255, 255, 255, 0.05)' },
          ticks: { color: theme.colors.textDim, font: { size: 10 } },
        },
      },
    }
    
    return <Bar data={chartData} options={options} />
  }

  const renderTableWidget = (widget: WidgetConfig, data: any) => {
    if (!data?.alerts || data.alerts.length === 0) {
      return <div className="widget-no-data">暂无数据</div>
    }
    
    const alerts = data.alerts.slice(0, widget.config?.limit || 10)
    
    return (
      <div className="table-widget">
        <table>
          <thead>
            <tr>
              <th>时间</th>
              <th>类型</th>
              <th>严重级别</th>
              <th>标题</th>
            </tr>
          </thead>
          <tbody>
            {alerts.map((alert: any) => (
              <tr key={alert.id}>
                <td>{new Date(alert.triggered_at).toLocaleString()}</td>
                <td>{alert.type}</td>
                <td>
                  <span className={`severity-badge ${alert.severity}`}>
                    {alert.severity}
                  </span>
                </td>
                <td>{alert.title}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  if (loading && !currentDashboard) {
    return <div className="custom-dashboard"><div className="loading">加载中...</div></div>
  }

  return (
    <div className="custom-dashboard">
      <div className="dashboard-header">
        <div className="header-left">
          <select
            className="dashboard-select"
            value={currentDashboard?.id || ''}
            onChange={(e) => loadDashboard(e.target.value)}
          >
            {dashboards.map(d => (
              <option key={d.id} value={d.id}>
                {d.name} {d.is_default ? '(默认)' : ''}
              </option>
            ))}
          </select>
        </div>
        
        <div className="header-actions">
          <button
            className="btn-create"
            onClick={() => setShowCreateModal(true)}
          >
            + 新建
          </button>
          
          {!currentDashboard?.is_default && (
            <button
              className="btn-delete"
              onClick={handleDeleteDashboard}
            >
              删除
            </button>
          )}
          
          {editMode ? (
            <>
              <button className="btn-save" onClick={handleSaveLayout}>
                保存
              </button>
              <button className="btn-cancel" onClick={() => setEditMode(false)}>
                取消
              </button>
            </>
          ) : (
            <button className="btn-edit" onClick={() => setEditMode(true)}>
              编辑
            </button>
          )}
        </div>
      </div>

      {currentDashboard && (
        <div className="dashboard-grid">
          {currentDashboard.widgets.map(widget => (
            <div
              key={widget.id}
              className="dashboard-widget"
              style={{
                gridColumn: `span ${widget.position.w}`,
                gridRow: `span ${widget.position.h}`,
              }}
            >
              <div className="widget-header">
                <h3>{widget.title}</h3>
                {editMode && (
                  <button className="widget-settings">⚙️</button>
                )}
              </div>
              <div className="widget-content">
                {renderWidget(widget)}
              </div>
            </div>
          ))}
        </div>
      )}

      {showCreateModal && (
        <div className="modal-overlay" onClick={() => setShowCreateModal(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h3>创建新 Dashboard</h3>
            <input
              type="text"
              placeholder="Dashboard 名称"
              value={newDashboardName}
              onChange={(e) => setNewDashboardName(e.target.value)}
            />
            <select
              value={selectedTemplate}
              onChange={(e) => setSelectedTemplate(e.target.value)}
            >
              {defaultTemplates.map(t => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
            </select>
            <div className="modal-actions">
              <button onClick={() => setShowCreateModal(false)}>取消</button>
              <button className="btn-primary" onClick={handleCreateDashboard}>
                创建
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default CustomDashboard
