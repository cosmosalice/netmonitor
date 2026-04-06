import React, { useState, useEffect } from 'react'
import { getSNMPDevices, addSNMPDevice, deleteSNMPDevice, pollSNMPDevice } from '../api/index'
import { theme } from '../theme'

interface SNMPInterface {
  index: number
  name: string
  status: string
  in_octets: number
  out_octets: number
  speed: number
}

interface SNMPDevice {
  id: string
  name: string
  ip: string
  community: string
  version: string
  port: number
  enabled: boolean
  status: string
  last_polled: string | null
  sys_descr: string
  sys_name: string
  interfaces: SNMPInterface[]
}

const SNMP: React.FC = () => {
  const [devices, setDevices] = useState<SNMPDevice[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [expandedDevice, setExpandedDevice] = useState<string | null>(null)
  const [formData, setFormData] = useState({
    name: '',
    ip: '',
    community: 'public',
    version: 'v2c',
    port: 161
  })

  const fetchDevices = async () => {
    try {
      const data = await getSNMPDevices()
      setDevices(data.devices || [])
    } catch (error) {
      console.error('Failed to fetch SNMP devices:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchDevices()
    const timer = setInterval(fetchDevices, 30000)
    return () => clearInterval(timer)
  }, [])

  const handleAddDevice = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await addSNMPDevice(formData)
      setShowModal(false)
      setFormData({ name: '', ip: '', community: 'public', version: 'v2c', port: 161 })
      fetchDevices()
    } catch (error) {
      console.error('Failed to add device:', error)
      alert('Failed to add device')
    }
  }

  const handleDeleteDevice = async (id: string) => {
    if (!window.confirm('Are you sure you want to delete this device?')) return
    try {
      await deleteSNMPDevice(id)
      fetchDevices()
    } catch (error) {
      console.error('Failed to delete device:', error)
    }
  }

  const handlePollDevice = async (id: string) => {
    try {
      await pollSNMPDevice(id)
      fetchDevices()
    } catch (error) {
      console.error('Failed to poll device:', error)
    }
  }

  const maskCommunity = (community: string) => {
    if (!community || community.length <= 2) return '***'
    return community.slice(0, 2) + '*'.repeat(community.length - 2)
  }

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  const formatSpeed = (speed: number) => {
    if (speed === 0) return 'Unknown'
    return (speed / 1000000).toFixed(0) + ' Mbps'
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'up': return theme.colors.sentryPurple
      case 'down': return theme.colors.error
      default: return theme.colors.warning
    }
  }
  
  const getInterfaceStatusColor = (status: string) => {
    switch (status) {
      case 'up': return theme.colors.success
      case 'down': return theme.colors.error
      default: return theme.colors.textDim
    }
  }

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <h1 style={styles.title}>SNMP 管理</h1>
        <button style={styles.addButton} onClick={() => setShowModal(true)}>
          + 添加设备
        </button>
      </div>

      {loading ? (
        <div style={styles.loading}>Loading...</div>
      ) : devices.length === 0 ? (
        <div style={styles.empty}>暂无 SNMP 设备，点击"添加设备"开始监控</div>
      ) : (
        <div style={styles.deviceList}>
          {devices.map((device) => (
            <div key={device.id} style={styles.deviceCard}>
              <div 
                style={styles.deviceHeader}
                onClick={() => setExpandedDevice(expandedDevice === device.id ? null : device.id)}
              >
                <div style={styles.deviceInfo}>
                  <span style={{...styles.statusDot, backgroundColor: getStatusColor(device.status)}} />
                  <span style={styles.deviceName}>{device.name || device.ip}</span>
                  <span style={styles.deviceIP}>{device.ip}</span>
                </div>
                <div style={styles.deviceMeta}>
                  <span style={styles.badge}>{device.version}</span>
                  <span style={styles.badge}>Port: {device.port}</span>
                  <span style={styles.communityMask}>Community: {maskCommunity(device.community)}</span>
                </div>
                <div style={styles.deviceActions}>
                  <button 
                    style={styles.actionButton}
                    onClick={(e) => { e.stopPropagation(); handlePollDevice(device.id) }}
                  >
                    轮询
                  </button>
                  <button 
                    style={{...styles.actionButton, backgroundColor: theme.colors.error}}
                    onClick={(e) => { e.stopPropagation(); handleDeleteDevice(device.id) }}
                  >
                    删除
                  </button>
                </div>
              </div>
              
              <div style={styles.deviceDetails}>
                <div style={styles.detailItem}>
                  <span style={styles.detailLabel}>状态:</span>
                  <span style={{...styles.detailValue, color: getStatusColor(device.status)}}>
                    {device.status.toUpperCase()}
                  </span>
                </div>
                <div style={styles.detailItem}>
                  <span style={styles.detailLabel}>最后轮询:</span>
                  <span style={styles.detailValue}>
                    {device.last_polled ? new Date(device.last_polled).toLocaleString() : 'Never'}
                  </span>
                </div>
                {device.sys_name && (
                  <div style={styles.detailItem}>
                    <span style={styles.detailLabel}>系统名称:</span>
                    <span style={styles.detailValue}>{device.sys_name}</span>
                  </div>
                )}
              </div>

              {expandedDevice === device.id && device.interfaces && device.interfaces.length > 0 && (
                <div style={styles.interfacesSection}>
                  <h3 style={styles.interfacesTitle}>接口列表 ({device.interfaces.length})</h3>
                  <div style={styles.interfacesTable}>
                    <div style={styles.tableHeader}>
                      <span style={styles.tableCell}>名称</span>
                      <span style={styles.tableCell}>状态</span>
                      <span style={styles.tableCell}>速率</span>
                      <span style={styles.tableCell}>流入</span>
                      <span style={styles.tableCell}>流出</span>
                    </div>
                    {device.interfaces.map((iface, index) => (
                      <div key={index} style={styles.tableRow}>
                        <span style={styles.tableCell}>{iface.name || `Interface ${iface.index}`}</span>
                        <span style={{...styles.tableCell, color: getInterfaceStatusColor(iface.status)}}>
                          {iface.status.toUpperCase()}
                        </span>
                        <span style={styles.tableCell}>{formatSpeed(iface.speed)}</span>
                        <span style={styles.tableCell}>{formatBytes(iface.in_octets)}</span>
                        <span style={styles.tableCell}>{formatBytes(iface.out_octets)}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {showModal && (
        <div style={styles.modalOverlay} onClick={() => setShowModal(false)}>
          <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
            <h2 style={styles.modalTitle}>添加 SNMP 设备</h2>
            <form onSubmit={handleAddDevice}>
              <div style={styles.formGroup}>
                <label style={styles.label}>设备名称</label>
                <input
                  style={styles.input}
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({...formData, name: e.target.value})}
                  placeholder="e.g., Router-1"
                />
              </div>
              <div style={styles.formGroup}>
                <label style={styles.label}>IP 地址 *</label>
                <input
                  style={styles.input}
                  type="text"
                  value={formData.ip}
                  onChange={(e) => setFormData({...formData, ip: e.target.value})}
                  placeholder="e.g., 192.168.1.1"
                  required
                />
              </div>
              <div style={styles.formGroup}>
                <label style={styles.label}>Community String</label>
                <input
                  style={styles.input}
                  type="text"
                  value={formData.community}
                  onChange={(e) => setFormData({...formData, community: e.target.value})}
                  placeholder="public"
                />
              </div>
              <div style={styles.formRow}>
                <div style={styles.formGroup}>
                  <label style={styles.label}>SNMP 版本</label>
                  <select
                    style={styles.select}
                    value={formData.version}
                    onChange={(e) => setFormData({...formData, version: e.target.value})}
                  >
                    <option value="v1">v1</option>
                    <option value="v2c">v2c</option>
                  </select>
                </div>
                <div style={styles.formGroup}>
                  <label style={styles.label}>端口</label>
                  <input
                    style={styles.input}
                    type="number"
                    value={formData.port}
                    onChange={(e) => setFormData({...formData, port: parseInt(e.target.value) || 161})}
                  />
                </div>
              </div>
              <div style={styles.modalActions}>
                <button type="button" style={styles.cancelButton} onClick={() => setShowModal(false)}>
                  取消
                </button>
                <button type="submit" style={styles.submitButton}>
                  添加
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
  },
  deviceList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '16px',
  },
  deviceCard: {
    backgroundColor: theme.colors.bgDeep,
    borderRadius: '12px',
    padding: '20px',
    border: `1px solid ${theme.colors.border}`,
  },
  deviceHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    cursor: 'pointer',
    flexWrap: 'wrap',
    gap: '12px',
  },
  deviceInfo: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    flex: 1,
    minWidth: '200px',
  },
  statusDot: {
    width: '10px',
    height: '10px',
    borderRadius: '50%',
  },
  deviceName: {
    fontSize: '18px',
    fontWeight: 600,
  },
  deviceIP: {
    color: theme.colors.textDim,
    fontSize: '14px',
  },
  deviceMeta: {
    display: 'flex',
    gap: '8px',
    alignItems: 'center',
  },
  badge: {
    padding: '4px 10px',
    backgroundColor: theme.colors.deepViolet,
    borderRadius: '4px',
    fontSize: '12px',
    color: theme.colors.sentryPurple,
  },
  communityMask: {
    color: theme.colors.textDim,
    fontSize: '12px',
  },
  deviceActions: {
    display: 'flex',
    gap: '8px',
  },
  actionButton: {
    padding: '6px 14px',
    backgroundColor: theme.colors.deepViolet,
    color: theme.colors.textPrimary,
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '12px',
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  deviceDetails: {
    display: 'flex',
    gap: '24px',
    marginTop: '16px',
    paddingTop: '16px',
    borderTop: `1px solid ${theme.colors.border}`,
    flexWrap: 'wrap',
  },
  detailItem: {
    display: 'flex',
    gap: '8px',
  },
  detailLabel: {
    color: theme.colors.textDim,
    fontSize: '13px',
  },
  detailValue: {
    fontSize: '13px',
  },
  interfacesSection: {
    marginTop: '16px',
    paddingTop: '16px',
    borderTop: `1px solid ${theme.colors.border}`,
  },
  interfacesTitle: {
    fontSize: '16px',
    fontWeight: 600,
    marginBottom: '12px',
    color: theme.colors.sentryPurple,
  },
  interfacesTable: {
    display: 'flex',
    flexDirection: 'column',
    gap: '8px',
  },
  tableHeader: {
    display: 'grid',
    gridTemplateColumns: '2fr 1fr 1fr 1fr 1fr',
    gap: '12px',
    padding: '8px 12px',
    backgroundColor: theme.colors.deepViolet,
    borderRadius: '4px',
    fontSize: '12px',
    fontWeight: 600,
    color: theme.colors.sentryPurple,
  },
  tableRow: {
    display: 'grid',
    gridTemplateColumns: '2fr 1fr 1fr 1fr 1fr',
    gap: '12px',
    padding: '8px 12px',
    backgroundColor: theme.colors.bgPrimary,
    borderRadius: '4px',
    fontSize: '13px',
  },
  tableCell: {
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
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

export default SNMP
