import axios from 'axios'

const API_BASE_URL = 'http://localhost:8080/api/v1'

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 5000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// 请求拦截器 - 自动附加 Authorization header
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器 - 401 时跳转到登录页
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      window.location.href = '/#/login'
    }
    return Promise.reject(error)
  }
)

// 获取网卡列表
export const getInterfaces = async () => {
  const response = await api.get('/interfaces')
  return response.data
}

// 开始抓包
export const startCapture = async (iface: string) => {
  const response = await api.post('/capture/start', { interface: iface })
  return response.data
}

// 停止抓包
export const stopCapture = async () => {
  const response = await api.post('/capture/stop')
  return response.data
}

// 获取抓包状态
export const getCaptureStatus = async () => {
  const response = await api.get('/capture/status')
  return response.data
}

// 获取汇总统计
export const getSummaryStats = async () => {
  const response = await api.get('/stats/summary')
  return response.data
}

// 获取主机统计
export const getHostStats = async (limit: number = 100) => {
  const response = await api.get('/stats/hosts', { params: { limit } })
  return response.data
}

// 获取协议统计
export const getProtocolStats = async (limit: number = 50) => {
  const response = await api.get('/stats/protocols', { params: { limit } })
  return response.data
}

// 获取活跃 Flow
export const getActiveFlows = async (limit: number = 100) => {
  const response = await api.get('/flows/active', { params: { limit } })
  return response.data
}

// 获取单个 Flow 详情
export const getFlowDetail = async (flowId: string) => {
  const response = await api.get(`/flows/${encodeURIComponent(flowId)}`)
  return response.data
}

// 获取时序数据
export const getTimeseries = async (
  metricType: string,
  startTime: string,
  endTime: string
) => {
  const response = await api.get('/timeseries', {
    params: { metric_type: metricType, start_time: startTime, end_time: endTime },
  })
  return response.data
}

// 更新配置
export const updateConfig = async (config: any) => {
  const response = await api.post('/config', config)
  return response.data
}

// 历史流量趋势
export const getHistoricalTraffic = async (start: number, end: number, granularity?: string) => {
  const response = await api.get('/historical/traffic', { params: { start, end, granularity: granularity || 'auto' } })
  return response.data
}

// 历史 Top Hosts
export const getHistoricalHosts = async (start: number, end: number, top?: number, sort?: string) => {
  const response = await api.get('/historical/hosts', { params: { start, end, top: top || 20, sort: sort || 'total' } })
  return response.data
}

// 历史协议分布
export const getHistoricalProtocols = async (start: number, end: number) => {
  const response = await api.get('/historical/protocols', { params: { start, end } })
  return response.data
}

// 时间段对比
export const getHistoricalCompare = async (p1Start: number, p1End: number, p2Start: number, p2End: number, metric?: string) => {
  const response = await api.get('/historical/compare', { params: { period1_start: p1Start, period1_end: p1End, period2_start: p2Start, period2_end: p2End, metric: metric || 'bandwidth' } })
  return response.data
}

// 历史 Flow 查询
export const getHistoricalFlows = async (params: { start: number; end: number; src_ip?: string; dst_ip?: string; protocol?: string; l7_protocol?: string; limit?: number; offset?: number }) => {
  const response = await api.get('/flows/historical', { params })
  return response.data
}

// 告警列表
export const getAlerts = async (params?: { type?: string; severity?: string; status?: string; start?: number; end?: number; entity_id?: string; limit?: number; offset?: number }) => {
  const response = await api.get('/alerts', { params })
  return response.data
}

// 确认告警
export const acknowledgeAlert = async (id: number) => {
  const response = await api.post(`/alerts/${id}/acknowledge`)
  return response.data
}

// 解决告警
export const resolveAlert = async (id: number) => {
  const response = await api.post(`/alerts/${id}/resolve`)
  return response.data
}

// 告警统计
export const getAlertStats = async () => {
  const response = await api.get('/alerts/stats')
  return response.data
}

// 告警规则列表
export const getAlertRules = async () => {
  const response = await api.get('/alerts/rules')
  return response.data
}

// 保存告警规则
export const saveAlertRule = async (rule: any) => {
  const response = await api.post('/alerts/rules', rule)
  return response.data
}

// 删除告警规则
export const deleteAlertRule = async (id: string) => {
  const response = await api.delete(`/alerts/rules/${id}`)
  return response.data
}

// 通知端点列表
export const getNotificationEndpoints = async () => {
  const response = await api.get('/alerts/notification-endpoints')
  return response.data
}

// 保存通知端点
export const saveNotificationEndpoint = async (endpoint: any) => {
  const response = await api.post('/alerts/notification-endpoints', endpoint)
  return response.data
}

// 删除通知端点
export const deleteNotificationEndpoint = async (id: string) => {
  const response = await api.delete(`/alerts/notification-endpoints/${id}`)
  return response.data
}

// 流量矩阵
export const getTrafficMatrix = async (params?: { start?: number; end?: number; limit?: number; group_by?: string }) => {
  const response = await api.get('/stats/traffic-matrix', { params })
  return response.data
}

// 导出 API（返回 blob 数据）
export const exportFlows = async (params: { format: string; start?: number; end?: number; src_ip?: string; dst_ip?: string; protocol?: string }) => {
  const response = await api.get('/export/flows', { params, responseType: 'blob' })
  return response
}

export const exportHosts = async (params: { format: string; sort?: string; limit?: number }) => {
  const response = await api.get('/export/hosts', { params, responseType: 'blob' })
  return response
}

export const exportTimeseries = async (params: { format: string; type?: string; start?: number; end?: number }) => {
  const response = await api.get('/export/timeseries', { params, responseType: 'blob' })
  return response
}

// 触发浏览器下载
export function downloadBlob(blob: Blob, filename: string) {
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}

// 报表列表
export const getReports = async () => {
  const response = await api.get('/reports')
  return response.data
}

// 下载报表
export const downloadReport = async (id: string) => {
  const response = await api.get(`/reports/${encodeURIComponent(id)}/download`, { responseType: 'text' } as any)
  return response.data
}

// 手动生成报表
export const generateReport = async (type: string, date: string) => {
  const response = await api.post('/reports/generate', { type, date })
  return response.data
}

// 获取报表配置
export const getReportConfigs = async () => {
  const response = await api.get('/reports/configs')
  return response.data
}

// 保存报表配置
export const saveReportConfig = async (config: any) => {
  const response = await api.post('/reports/configs', config)
  return response.data
}

// GeoIP 主机列表（带地理信息）
export const getGeoHosts = async (limit: number = 100) => {
  const response = await api.get('/geo/hosts', { params: { limit } })
  return response.data
}

// 获取主机风险评分列表
export const getHostRisks = async (params?: { sort?: string; limit?: number }) => {
  const response = await api.get('/hosts/risks', { params })
  return response.data
}

// 获取单主机风险详情
export const getHostRiskDetail = async (ip: string) => {
  const response = await api.get(`/hosts/${encodeURIComponent(ip)}/risk`)
  return response.data
}

// 按国家统计
export const getCountryStats = async () => {
  const response = await api.get('/stats/countries')
  return response.data
}

// 按 ASN 统计
export const getASNStats = async () => {
  const response = await api.get('/stats/asn')
  return response.data
}

// DNS 分析 API
export const getDNSSummary = async () => {
  const response = await api.get('/dns/summary')
  return response.data
}

export const getDNSDomains = async (limit: number = 50) => {
  const response = await api.get('/dns/domains', { params: { limit } })
  return response.data
}

export const getDNSServers = async (limit: number = 20) => {
  const response = await api.get('/dns/servers', { params: { limit } })
  return response.data
}

export const getDNSResponseCodes = async () => {
  const response = await api.get('/dns/response-codes')
  return response.data
}

export const getDNSQueryTypes = async () => {
  const response = await api.get('/dns/query-types')
  return response.data
}

// 获取主机 OS 信息
export const getHostOS = async (ip: string) => {
  const response = await api.get(`/hosts/${encodeURIComponent(ip)}/os`)
  return response.data
}

// HTTP/TLS 分析 API
export const getHTTPSummary = async () => {
  const response = await api.get('/http/summary')
  return response.data
}

export const getHTTPHosts = async (limit: number = 50) => {
  const response = await api.get('/http/hosts', { params: { limit } })
  return response.data
}

export const getHTTPUserAgents = async (limit: number = 30) => {
  const response = await api.get('/http/user-agents', { params: { limit } })
  return response.data
}

export const getHTTPMethods = async () => {
  const response = await api.get('/http/methods')
  return response.data
}

export const getHTTPStatusCodes = async () => {
  const response = await api.get('/http/status-codes')
  return response.data
}

export const getTLSSummary = async () => {
  const response = await api.get('/tls/summary')
  return response.data
}

export const getTLSSNI = async (limit: number = 50) => {
  const response = await api.get('/tls/sni', { params: { limit } })
  return response.data
}

export const getTLSJA3 = async (limit: number = 30) => {
  const response = await api.get('/tls/ja3', { params: { limit } })
  return response.data
}

export const getTLSVersions = async () => {
  const response = await api.get('/tls/versions')
  return response.data
}

// 下载 PCAP 文件
export const downloadPCAP = async (type_: string, id: string) => {
  const response = await api.get(`/pcap/${type_}/${encodeURIComponent(id)}`, { responseType: 'blob' })
  return response
}

// 下载实时 PCAP
export const downloadLivePCAP = async (filter?: string, duration?: number) => {
  const response = await api.get('/pcap/live', { params: { filter, duration }, responseType: 'blob' })
  return response
}

// 网络拓扑 API
export const getTopology = async (limit: number = 30) => {
  const response = await api.get('/topology', { params: { limit } })
  return response.data
}

// VLAN API
export const getVLANs = async () => {
  const response = await api.get('/vlans')
  return response.data
}

export const getVLANHosts = async (vlanId: number) => {
  const response = await api.get(`/vlans/${vlanId}/hosts`)
  return response.data
}

export const getVLANFlows = async (vlanId: number, limit: number = 100) => {
  const response = await api.get(`/vlans/${vlanId}/flows`, { params: { limit } })
  return response.data
}

// SNMP API
export const getSNMPDevices = async () => {
  const response = await api.get('/snmp/devices')
  return response.data
}

export const addSNMPDevice = async (device: { name: string; ip: string; community?: string; version?: string; port?: number }) => {
  const response = await api.post('/snmp/devices', device)
  return response.data
}

export const deleteSNMPDevice = async (id: string) => {
  const response = await api.delete(`/snmp/devices/${encodeURIComponent(id)}`)
  return response.data
}

export const getSNMPDevice = async (id: string) => {
  const response = await api.get(`/snmp/devices/${encodeURIComponent(id)}`)
  return response.data
}

export const pollSNMPDevice = async (id: string) => {
  const response = await api.post(`/snmp/devices/${encodeURIComponent(id)}/poll`)
  return response.data
}

// Monitoring API
export const getMonitoringProbes = async () => {
  const response = await api.get('/monitoring/probes')
  return response.data
}

export const createMonitoringProbe = async (probe: { name: string; type: string; host?: string; port?: number; url?: string; interval?: number; timeout?: number; enabled?: boolean }) => {
  const response = await api.post('/monitoring/probes', probe)
  return response.data
}

export const deleteMonitoringProbe = async (id: string) => {
  const response = await api.delete(`/monitoring/probes/${encodeURIComponent(id)}`)
  return response.data
}

export const getMonitoringProbe = async (id: string) => {
  const response = await api.get(`/monitoring/probes/${encodeURIComponent(id)}`)
  return response.data
}

export const getMonitoringProbeResults = async (id: string, limit: number = 100) => {
  const response = await api.get(`/monitoring/probes/${encodeURIComponent(id)}/results`, { params: { limit } })
  return response.data
}

export const testMonitoringProbe = async (id: string) => {
  const response = await api.post(`/monitoring/probes/${encodeURIComponent(id)}/test`)
  return response.data
}

// Device Discovery API
export const getDevices = async (params?: { sort?: string; limit?: number; offset?: number; search?: string }) => {
  const response = await api.get('/devices', { params })
  return response.data
}

export const getDevice = async (mac: string) => {
  const response = await api.get(`/devices/${encodeURIComponent(mac)}`)
  return response.data
}

export const getDeviceFlows = async (mac: string, limit: number = 100) => {
  const response = await api.get(`/devices/${encodeURIComponent(mac)}/flows`, { params: { limit } })
  return response.data
}

export const getDeviceStats = async () => {
  const response = await api.get('/devices/stats')
  return response.data
}

// ==================== Auth API ====================

// 登录
export const login = async (username: string, password: string) => {
  const response = await api.post('/auth/login', { username, password })
  return response.data
}

// 登出
export const logout = async () => {
  const response = await api.post('/auth/logout')
  return response.data
}

// 获取当前用户信息
export const getMe = async () => {
  const response = await api.get('/auth/me')
  return response.data
}

// 修改密码
export const changePassword = async (oldPassword: string, newPassword: string) => {
  const response = await api.put('/auth/password', { old_password: oldPassword, new_password: newPassword })
  return response.data
}

// ==================== User Management API ====================

// 获取用户列表（admin only）
export const getUsers = async () => {
  const response = await api.get('/users')
  return response.data
}

// 创建用户（admin only）
export const createUser = async (user: { username: string; password: string; role: string; email?: string }) => {
  const response = await api.post('/users', user)
  return response.data
}

// 更新用户（admin only）
export const updateUser = async (id: number, user: { username?: string; role?: string; email?: string }) => {
  const response = await api.put(`/users/${id}`, user)
  return response.data
}

// 删除用户（admin only）
export const deleteUser = async (id: number) => {
  const response = await api.delete(`/users/${id}`)
  return response.data
}

// 重置密码（admin only）
export const resetPassword = async (id: number, newPassword: string) => {
  const response = await api.post(`/users/${id}/reset-password`, { new_password: newPassword })
  return response.data
}

// ==================== Integrations API ====================

export const getIntegrations = async () => {
  const response = await api.get('/integrations')
  return response.data
}

export const getSyslogConfig = async () => {
  const response = await api.get('/integrations/syslog')
  return response.data
}

export const updateSyslogConfig = async (config: any) => {
  const response = await api.put('/integrations/syslog', config)
  return response.data
}

export const testSyslogConnection = async (config: any) => {
  const response = await api.post('/integrations/syslog/test', config)
  return response.data
}

export const getESConfig = async () => {
  const response = await api.get('/integrations/elasticsearch')
  return response.data
}

export const updateESConfig = async (config: any) => {
  const response = await api.put('/integrations/elasticsearch', config)
  return response.data
}

export const testESConnection = async (config: any) => {
  const response = await api.post('/integrations/elasticsearch/test', config)
  return response.data
}

// ==================== Dashboards API ====================

export const getDashboards = async () => {
  const response = await api.get('/dashboards')
  return response.data
}

export const getDashboard = async (id: string) => {
  const response = await api.get(`/dashboards/${encodeURIComponent(id)}`)
  return response.data
}

export const createDashboard = async (dashboard: any) => {
  const response = await api.post('/dashboards', dashboard)
  return response.data
}

export const updateDashboard = async (id: string, dashboard: any) => {
  const response = await api.put(`/dashboards/${encodeURIComponent(id)}`, dashboard)
  return response.data
}

export const deleteDashboard = async (id: string) => {
  const response = await api.delete(`/dashboards/${encodeURIComponent(id)}`)
  return response.data
}

// ==================== Interface Management API ====================

export const getAllInterfaces = async () => {
  const response = await api.get('/interfaces/all')
  return response.data
}

export const getActiveInterfaceList = async () => {
  const response = await api.get('/interfaces/active')
  return response.data
}

export const enableInterface = async (name: string, bpfFilter?: string) => {
  const response = await api.post(`/interfaces/${encodeURIComponent(name)}/enable`, {
    bpf_filter: bpfFilter,
  })
  return response.data
}

export const disableInterface = async (name: string) => {
  const response = await api.post(`/interfaces/${encodeURIComponent(name)}/disable`)
  return response.data
}

export const getInterfaceStats = async (name: string) => {
  const response = await api.get(`/interfaces/${encodeURIComponent(name)}/stats`)
  return response.data
}

export const getInterfacesAggregateStats = async () => {
  const response = await api.get('/interfaces/stats/aggregate')
  return response.data
}

// ==================== Flow Collector API ====================

export const getCollectors = async () => {
  const response = await api.get('/collectors')
  return response.data
}

export const getCollectorStats = async () => {
  const response = await api.get('/collectors/stats')
  return response.data
}

export const getCollectorFlows = async () => {
  const response = await api.get('/collectors/flows')
  return response.data
}

export const startNetFlowCollector = async (port: number = 2055) => {
  const response = await api.post('/collectors/netflow/start', { port })
  return response.data
}

export const stopNetFlowCollector = async () => {
  const response = await api.post('/collectors/netflow/stop')
  return response.data
}

export const startSFlowCollector = async (port: number = 6343) => {
  const response = await api.post('/collectors/sflow/start', { port })
  return response.data
}

export const stopSFlowCollector = async () => {
  const response = await api.post('/collectors/sflow/stop')
  return response.data
}

export default api
