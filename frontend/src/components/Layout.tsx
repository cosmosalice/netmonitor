import React, { useState, useEffect } from 'react'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { getAlertStats, logout } from '../api/index'
import { COLORS } from '../theme'
import './Layout.css'

const Layout: React.FC = () => {
  const navigate = useNavigate()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState(false)
  const [alertCount, setAlertCount] = useState(0)
  const [user, setUser] = useState<any>(null)
  const [showUserMenu, setShowUserMenu] = useState(false)

  useEffect(() => {
    // 获取当前用户信息
    const userStr = localStorage.getItem('user')
    if (userStr) {
      setUser(JSON.parse(userStr))
    }

    const fetchAlertCount = () => {
      getAlertStats().then(data => {
        const triggered = data?.by_status?.triggered || 0
        const acked = data?.by_status?.acknowledged || 0
        setAlertCount(triggered + acked)
      }).catch(() => {})
    }
    fetchAlertCount()
    const timer = setInterval(fetchAlertCount, 30000)
    return () => clearInterval(timer)
  }, [])

  const handleLogout = async () => {
    try {
      await logout()
    } catch (e) {
      // ignore
    }
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    navigate('/login')
  }

  const menuItems = [
    { key: '/dashboard', label: 'Dashboard', icon: '📊' },
    { key: '/realtime', label: '实时监控', icon: '📈' },
    { key: '/protocols', label: '协议分析', icon: '🔍' },
    { key: '/hosts', label: '主机管理', icon: '💻' },
    { key: '/devices', label: '设备发现', icon: '🔍' },
    { key: '/history', label: '历史分析', icon: '📜' },
    { key: '/traffic-matrix', label: '流量矩阵', icon: '🔥' },
    { key: '/topology', label: '网络拓扑', icon: '🗺️' },
    { key: '/vlans', label: 'VLAN 分析', icon: '🔀' },
    { key: '/dns', label: 'DNS 分析', icon: '🌐' },
    { key: '/http-tls', label: 'HTTP/TLS 分析', icon: '🔐' },
    { key: '/snmp', label: 'SNMP 管理', icon: '📡' },
    { key: '/monitoring', label: '主动监控', icon: '🔔' },
    { key: '/interfaces', label: '网络接口', icon: '🔌' },
    { key: '/collectors', label: 'Flow 收集器', icon: '📥' },
    { key: '/alerts', label: '告警管理', icon: '🚨' },
    { key: '/reports', label: '报表管理', icon: '📋' },
    { key: '/integrations', label: '系统集成', icon: '🔌' },
    { key: '/custom-dashboard', label: '自定义面板', icon: '🎛️' },
    ...(user?.role === 'admin' ? [{ key: '/users', label: '用户管理', icon: '👥' }] : []),
    { key: '/settings', label: '系统设置', icon: '⚙️' },
  ]

  return (
    <div className="layout">
      <aside className={`sidebar ${collapsed ? 'collapsed' : ''}`}>
        <div className="logo">
          <h2>{collapsed ? 'NM' : 'NetMonitor'}</h2>
          <button onClick={() => setCollapsed(!collapsed)} className="collapse-btn">
            {collapsed ? '→' : '←'}
          </button>
        </div>
        <nav className="menu">
          {menuItems.map(item => (
            <div
              key={item.key}
              className={`menu-item ${location.pathname === item.key ? 'active' : ''}`}
              onClick={() => navigate(item.key)}
            >
              <span className="icon">{item.icon}</span>
              {!collapsed && <span className="label">{item.label}</span>}
              {!collapsed && item.key === '/alerts' && alertCount > 0 && (
                <span style={{ marginLeft: 'auto', background: COLORS.error, color: '#fff', borderRadius: 10, padding: '1px 7px', fontSize: 11, fontWeight: 700 }}>{alertCount}</span>
              )}
            </div>
          ))}
        </nav>
      </aside>
      <main className="main-content">
        <header className="top-header">
          <div className="header-spacer"></div>
          <div className="user-section">
            <div 
              className="user-info"
              onClick={() => setShowUserMenu(!showUserMenu)}
            >
              <span className="user-avatar">👤</span>
              <span className="user-name">{user?.username || 'User'}</span>
              <span className="user-role">{user?.role === 'admin' ? '管理员' : '查看者'}</span>
              <span className="dropdown-arrow">▼</span>
            </div>
            {showUserMenu && (
              <div className="user-dropdown">
                <div className="dropdown-item" onClick={() => { navigate('/settings'); setShowUserMenu(false) }}>
                  ⚙️ 设置
                </div>
                <div className="dropdown-divider"></div>
                <div className="dropdown-item logout" onClick={handleLogout}>
                  🚪 退出登录
                </div>
              </div>
            )}
          </div>
        </header>
        <div className="content-wrapper">
          <Outlet />
        </div>
      </main>
    </div>
  )
}

export default Layout
