import React, { useState, useEffect } from 'react'
import {
  getIntegrations,
  getSyslogConfig,
  updateSyslogConfig,
  testSyslogConnection,
  getESConfig,
  updateESConfig,
  testESConnection,
} from '../api'
import './Integrations.css'

interface SyslogConfig {
  target: string
  port: number
  protocol: string
  facility: number
  severity: number
  use_tls: boolean
  enabled: boolean
}

interface ESConfig {
  url: string
  index_prefix: string
  username: string
  password: string
  batch_size: number
  flush_interval: number
  enabled: boolean
}

const Integrations: React.FC = () => {
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState<string | null>(null)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const [syslogConfig, setSyslogConfig] = useState<SyslogConfig>({
    target: 'localhost',
    port: 514,
    protocol: 'udp',
    facility: 16,
    severity: 6,
    use_tls: false,
    enabled: false,
  })

  const [esConfig, setEsConfig] = useState<ESConfig>({
    url: 'http://localhost:9200',
    index_prefix: 'netmonitor',
    username: '',
    password: '',
    batch_size: 100,
    flush_interval: 30,
    enabled: false,
  })

  const [syslogStatus, setSyslogStatus] = useState<{ connected: boolean; last_error: string }>({
    connected: false,
    last_error: '',
  })

  const [esStatus, setEsStatus] = useState<{ connected: boolean; last_error: string; buffer_size: number }>({
    connected: false,
    last_error: '',
    buffer_size: 0,
  })

  useEffect(() => {
    loadConfigs()
    const interval = setInterval(loadStatus, 10000)
    return () => clearInterval(interval)
  }, [])

  const loadConfigs = async () => {
    try {
      setLoading(true)
      const [integrationsData, syslog, es] = await Promise.all([
        getIntegrations(),
        getSyslogConfig(),
        getESConfig(),
      ])
      // Use integrations data if needed
      console.log('Integrations:', integrationsData)

      if (syslog) {
        setSyslogConfig({
          target: syslog.target || 'localhost',
          port: syslog.port || 514,
          protocol: syslog.protocol || 'udp',
          facility: syslog.facility ?? 16,
          severity: syslog.severity ?? 6,
          use_tls: syslog.use_tls || false,
          enabled: syslog.enabled || false,
        })
        setSyslogStatus({
          connected: syslog.connected || false,
          last_error: syslog.last_error || '',
        })
      }

      if (es) {
        setEsConfig({
          url: es.url || 'http://localhost:9200',
          index_prefix: es.index_prefix || 'netmonitor',
          username: es.username || '',
          password: es.password || '',
          batch_size: es.batch_size || 100,
          flush_interval: es.flush_interval || 30,
          enabled: es.enabled || false,
        })
        setEsStatus({
          connected: es.running || false,
          last_error: es.last_error || '',
          buffer_size: es.buffer_size || 0,
        })
      }
    } catch (err) {
      console.error('Failed to load configs:', err)
    } finally {
      setLoading(false)
    }
  }

  const loadStatus = async () => {
    try {
      const [syslog, es] = await Promise.all([getSyslogConfig(), getESConfig()])
      if (syslog) {
        setSyslogStatus({
          connected: syslog.connected || false,
          last_error: syslog.last_error || '',
        })
      }
      if (es) {
        setEsStatus({
          connected: es.running || false,
          last_error: es.last_error || '',
          buffer_size: es.buffer_size || 0,
        })
      }
    } catch (err) {
      // Ignore errors on status refresh
    }
  }

  const handleSaveSyslog = async () => {
    try {
      setSaving(true)
      await updateSyslogConfig(syslogConfig)
      setMessage({ type: 'success', text: 'Syslog 配置已保存' })
      loadStatus()
    } catch (err: any) {
      setMessage({ type: 'error', text: err.message || '保存失败' })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const handleTestSyslog = async () => {
    try {
      setTesting('syslog')
      const result = await testSyslogConnection({
        target: syslogConfig.target,
        port: syslogConfig.port,
        protocol: syslogConfig.protocol,
        use_tls: syslogConfig.use_tls,
      })
      if (result.success) {
        setMessage({ type: 'success', text: 'Syslog 连接测试成功' })
      } else {
        setMessage({ type: 'error', text: result.error || '连接测试失败' })
      }
    } catch (err: any) {
      setMessage({ type: 'error', text: err.message || '测试失败' })
    } finally {
      setTesting(null)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const handleSaveES = async () => {
    try {
      setSaving(true)
      await updateESConfig(esConfig)
      setMessage({ type: 'success', text: 'Elasticsearch 配置已保存' })
      loadStatus()
    } catch (err: any) {
      setMessage({ type: 'error', text: err.message || '保存失败' })
    } finally {
      setSaving(false)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const handleTestES = async () => {
    try {
      setTesting('es')
      const result = await testESConnection({
        url: esConfig.url,
        username: esConfig.username,
        password: esConfig.password,
      })
      if (result.success) {
        setMessage({ type: 'success', text: 'Elasticsearch 连接测试成功' })
      } else {
        setMessage({ type: 'error', text: result.error || '连接测试失败' })
      }
    } catch (err: any) {
      setMessage({ type: 'error', text: err.message || '测试失败' })
    } finally {
      setTesting(null)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const getStatusBadge = (connected: boolean, error: string) => {
    if (error) {
      return <span className="status-badge error">错误</span>
    }
    if (connected) {
      return <span className="status-badge connected">已连接</span>
    }
    return <span className="status-badge disconnected">未连接</span>
  }

  if (loading) {
    return <div className="integrations-page"><div className="loading">加载中...</div></div>
  }

  return (
    <div className="integrations-page">
      <h1>外部系统集成</h1>

      {message && (
        <div className={`message ${message.type}`}>
          {message.text}
        </div>
      )}

      {/* Syslog 配置卡片 */}
      <div className="integration-card">
        <div className="card-header">
          <h2>Syslog 转发</h2>
          <div className="header-actions">
            {getStatusBadge(syslogStatus.connected, syslogStatus.last_error)}
            <label className="toggle-switch">
              <input
                type="checkbox"
                checked={syslogConfig.enabled}
                onChange={(e) => setSyslogConfig({ ...syslogConfig, enabled: e.target.checked })}
              />
              <span className="slider"></span>
            </label>
          </div>
        </div>

        <div className="card-body">
          <div className="form-row">
            <div className="form-group">
              <label>目标地址</label>
              <input
                type="text"
                value={syslogConfig.target}
                onChange={(e) => setSyslogConfig({ ...syslogConfig, target: e.target.value })}
                placeholder="syslog.example.com"
              />
            </div>
            <div className="form-group">
              <label>端口</label>
              <input
                type="number"
                value={syslogConfig.port}
                onChange={(e) => setSyslogConfig({ ...syslogConfig, port: parseInt(e.target.value) || 514 })}
              />
            </div>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label>协议</label>
              <select
                value={syslogConfig.protocol}
                onChange={(e) => setSyslogConfig({ ...syslogConfig, protocol: e.target.value })}
              >
                <option value="udp">UDP</option>
                <option value="tcp">TCP</option>
              </select>
            </div>
            <div className="form-group">
              <label>Facility</label>
              <select
                value={syslogConfig.facility}
                onChange={(e) => setSyslogConfig({ ...syslogConfig, facility: parseInt(e.target.value) })}
              >
                <option value={0}>kern (0)</option>
                <option value={1}>user (1)</option>
                <option value={2}>mail (2)</option>
                <option value={3}>daemon (3)</option>
                <option value={4}>auth (4)</option>
                <option value={16}>local0 (16)</option>
                <option value={17}>local1 (17)</option>
                <option value={18}>local2 (18)</option>
                <option value={19}>local3 (19)</option>
                <option value={20}>local4 (20)</option>
                <option value={21}>local5 (21)</option>
                <option value={22}>local6 (22)</option>
                <option value={23}>local7 (23)</option>
              </select>
            </div>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label>Severity</label>
              <select
                value={syslogConfig.severity}
                onChange={(e) => setSyslogConfig({ ...syslogConfig, severity: parseInt(e.target.value) })}
              >
                <option value={0}>Emergency</option>
                <option value={1}>Alert</option>
                <option value={2}>Critical</option>
                <option value={3}>Error</option>
                <option value={4}>Warning</option>
                <option value={5}>Notice</option>
                <option value={6}>Info</option>
                <option value={7}>Debug</option>
              </select>
            </div>
            <div className="form-group checkbox-group">
              <label>
                <input
                  type="checkbox"
                  checked={syslogConfig.use_tls}
                  onChange={(e) => setSyslogConfig({ ...syslogConfig, use_tls: e.target.checked })}
                  disabled={syslogConfig.protocol !== 'tcp'}
                />
                使用 TLS (仅 TCP)
              </label>
            </div>
          </div>

          {syslogStatus.last_error && (
            <div className="error-message">
              错误: {syslogStatus.last_error}
            </div>
          )}

          <div className="card-actions">
            <button
              className="btn-test"
              onClick={handleTestSyslog}
              disabled={testing === 'syslog'}
            >
              {testing === 'syslog' ? '测试中...' : '测试连接'}
            </button>
            <button
              className="btn-save"
              onClick={handleSaveSyslog}
              disabled={saving}
            >
              {saving ? '保存中...' : '保存配置'}
            </button>
          </div>
        </div>
      </div>

      {/* Elasticsearch 配置卡片 */}
      <div className="integration-card">
        <div className="card-header">
          <h2>Elasticsearch 导出</h2>
          <div className="header-actions">
            {getStatusBadge(esStatus.connected, esStatus.last_error)}
            <label className="toggle-switch">
              <input
                type="checkbox"
                checked={esConfig.enabled}
                onChange={(e) => setEsConfig({ ...esConfig, enabled: e.target.checked })}
              />
              <span className="slider"></span>
            </label>
          </div>
        </div>

        <div className="card-body">
          <div className="form-row">
            <div className="form-group">
              <label>ES URL</label>
              <input
                type="text"
                value={esConfig.url}
                onChange={(e) => setEsConfig({ ...esConfig, url: e.target.value })}
                placeholder="http://localhost:9200"
              />
            </div>
            <div className="form-group">
              <label>索引前缀</label>
              <input
                type="text"
                value={esConfig.index_prefix}
                onChange={(e) => setEsConfig({ ...esConfig, index_prefix: e.target.value })}
                placeholder="netmonitor"
              />
            </div>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label>用户名</label>
              <input
                type="text"
                value={esConfig.username}
                onChange={(e) => setEsConfig({ ...esConfig, username: e.target.value })}
                placeholder="可选"
              />
            </div>
            <div className="form-group">
              <label>密码</label>
              <input
                type="password"
                value={esConfig.password}
                onChange={(e) => setEsConfig({ ...esConfig, password: e.target.value })}
                placeholder="可选"
              />
            </div>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label>批量大小</label>
              <input
                type="number"
                value={esConfig.batch_size}
                onChange={(e) => setEsConfig({ ...esConfig, batch_size: parseInt(e.target.value) || 100 })}
                min={10}
                max={1000}
              />
            </div>
            <div className="form-group">
              <label>刷新间隔 (秒)</label>
              <input
                type="number"
                value={esConfig.flush_interval}
                onChange={(e) => setEsConfig({ ...esConfig, flush_interval: parseInt(e.target.value) || 30 })}
                min={5}
                max={300}
              />
            </div>
          </div>

          {esStatus.buffer_size > 0 && (
            <div className="info-message">
              缓冲区: {esStatus.buffer_size} 条记录待发送
            </div>
          )}

          {esStatus.last_error && (
            <div className="error-message">
              错误: {esStatus.last_error}
            </div>
          )}

          <div className="card-actions">
            <button
              className="btn-test"
              onClick={handleTestES}
              disabled={testing === 'es'}
            >
              {testing === 'es' ? '测试中...' : '测试连接'}
            </button>
            <button
              className="btn-save"
              onClick={handleSaveES}
              disabled={saving}
            >
              {saving ? '保存中...' : '保存配置'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default Integrations
