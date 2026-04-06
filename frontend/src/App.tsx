import { HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Realtime from './pages/Realtime'
import Protocols from './pages/Protocols'
import Hosts from './pages/Hosts'
import Devices from './pages/Devices'
import Settings from './pages/Settings'
import FlowDetail from './pages/FlowDetail'
import History from './pages/History'
import Alerts from './pages/Alerts'
import Reports from './pages/Reports'
import TrafficMatrix from './pages/TrafficMatrix'
import DNS from './pages/DNS'
import Topology from './pages/Topology'
import VLANs from './pages/VLANs'
import HTTPAnalysis from './pages/HTTPAnalysis'
import SNMP from './pages/SNMP'
import Monitoring from './pages/Monitoring'
import Interfaces from './pages/Interfaces'
import Collectors from './pages/Collectors'
import Login from './pages/Login'
import Users from './pages/Users'
import Integrations from './pages/Integrations'
import CustomDashboard from './pages/CustomDashboard'

// 路由守卫组件
const PrivateRoute = ({ children }: { children: React.ReactNode }) => {
  const token = localStorage.getItem('token')
  return token ? <>{children}</> : <Navigate to="/login" replace />
}

function App() {
  return (
    <HashRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/" element={
          <PrivateRoute>
            <Layout />
          </PrivateRoute>
        }>
          <Route index element={<Navigate to="/dashboard" replace />} />
          <Route path="dashboard" element={<Dashboard />} />
          <Route path="realtime" element={<Realtime />} />
          <Route path="protocols" element={<Protocols />} />
          <Route path="hosts" element={<Hosts />} />
          <Route path="devices" element={<Devices />} />
          <Route path="settings" element={<Settings />} />
          <Route path="history" element={<History />} />
          <Route path="alerts" element={<Alerts />} />
          <Route path="traffic-matrix" element={<TrafficMatrix />} />
          <Route path="topology" element={<Topology />} />
          <Route path="vlans" element={<VLANs />} />
          <Route path="dns" element={<DNS />} />
          <Route path="http-tls" element={<HTTPAnalysis />} />
          <Route path="snmp" element={<SNMP />} />
          <Route path="monitoring" element={<Monitoring />} />
          <Route path="interfaces" element={<Interfaces />} />
          <Route path="collectors" element={<Collectors />} />
          <Route path="reports" element={<Reports />} />
          <Route path="users" element={<Users />} />
          <Route path="integrations" element={<Integrations />} />
          <Route path="custom-dashboard" element={<CustomDashboard />} />
          <Route path="flow/:id" element={<FlowDetail />} />
        </Route>
      </Routes>
    </HashRouter>
  )
}

export default App
