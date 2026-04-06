import React, { useState, useEffect } from 'react'
import { getUsers, createUser, updateUser, deleteUser, resetPassword } from '../api/index'
import './Users.css'

interface User {
  id: number
  username: string
  role: 'admin' | 'viewer'
  email: string
  created_at: string
  last_login?: string
}

const Users: React.FC = () => {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(false)
  const [showModal, setShowModal] = useState(false)
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [showResetModal, setShowResetModal] = useState(false)
  const [resetUserId, setResetUserId] = useState<number | null>(null)
  const [newPassword, setNewPassword] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const [formData, setFormData] = useState({
    username: '',
    password: '',
    role: 'viewer' as 'admin' | 'viewer',
    email: '',
  })

  const currentUser = JSON.parse(localStorage.getItem('user') || '{}')

  useEffect(() => {
    fetchUsers()
  }, [])

  const fetchUsers = async () => {
    setLoading(true)
    try {
      const response = await getUsers()
      setUsers(response.users || [])
    } catch (err: any) {
      setError(err.response?.data?.error || '获取用户列表失败')
    } finally {
      setLoading(false)
    }
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')

    try {
      await createUser(formData)
      setSuccess('用户创建成功')
      setShowModal(false)
      setFormData({ username: '', password: '', role: 'viewer', email: '' })
      fetchUsers()
    } catch (err: any) {
      setError(err.response?.data?.error || '创建用户失败')
    }
  }

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingUser) return

    setError('')
    setSuccess('')

    try {
      await updateUser(editingUser.id, {
        username: formData.username,
        role: formData.role,
        email: formData.email,
      })
      setSuccess('用户更新成功')
      setShowModal(false)
      setEditingUser(null)
      setFormData({ username: '', password: '', role: 'viewer', email: '' })
      fetchUsers()
    } catch (err: any) {
      setError(err.response?.data?.error || '更新用户失败')
    }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('确定要删除该用户吗？')) return

    setError('')
    setSuccess('')

    try {
      await deleteUser(id)
      setSuccess('用户删除成功')
      fetchUsers()
    } catch (err: any) {
      setError(err.response?.data?.error || '删除用户失败')
    }
  }

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!resetUserId || !newPassword) return

    setError('')
    setSuccess('')

    try {
      await resetPassword(resetUserId, newPassword)
      setSuccess('密码重置成功')
      setShowResetModal(false)
      setResetUserId(null)
      setNewPassword('')
    } catch (err: any) {
      setError(err.response?.data?.error || '密码重置失败')
    }
  }

  const openCreateModal = () => {
    setEditingUser(null)
    setFormData({ username: '', password: '', role: 'viewer', email: '' })
    setError('')
    setShowModal(true)
  }

  const openEditModal = (user: User) => {
    setEditingUser(user)
    setFormData({
      username: user.username,
      password: '',
      role: user.role,
      email: user.email,
    })
    setError('')
    setShowModal(true)
  }

  const openResetModal = (userId: number) => {
    setResetUserId(userId)
    setNewPassword('')
    setError('')
    setShowResetModal(true)
  }

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleString('zh-CN')
  }

  const getRoleBadge = (role: string) => {
    return role === 'admin' ? (
      <span className="role-badge admin">管理员</span>
    ) : (
      <span className="role-badge viewer">查看者</span>
    )
  }

  return (
    <div className="users-page">
      <div className="page-header">
        <h2>用户管理</h2>
        <button className="btn-primary" onClick={openCreateModal}>
          + 创建用户
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}

      <div className="users-table-container">
        <table className="users-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>用户名</th>
              <th>角色</th>
              <th>邮箱</th>
              <th>创建时间</th>
              <th>最后登录</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7} className="text-center">加载中...</td>
              </tr>
            ) : users.length === 0 ? (
              <tr>
                <td colSpan={7} className="text-center">暂无用户</td>
              </tr>
            ) : (
              users.map((user) => (
                <tr key={user.id}>
                  <td>{user.id}</td>
                  <td>{user.username}</td>
                  <td>{getRoleBadge(user.role)}</td>
                  <td>{user.email || '-'}</td>
                  <td>{formatDate(user.created_at)}</td>
                  <td>{formatDate(user.last_login)}</td>
                  <td>
                    <div className="action-buttons">
                      <button
                        className="btn-sm btn-edit"
                        onClick={() => openEditModal(user)}
                        disabled={user.id === currentUser.id}
                        title={user.id === currentUser.id ? '不能编辑自己' : '编辑'}
                      >
                        编辑
                      </button>
                      <button
                        className="btn-sm btn-reset"
                        onClick={() => openResetModal(user.id)}
                        disabled={user.id === currentUser.id}
                        title={user.id === currentUser.id ? '不能重置自己的密码' : '重置密码'}
                      >
                        重置密码
                      </button>
                      <button
                        className="btn-sm btn-delete"
                        onClick={() => handleDelete(user.id)}
                        disabled={user.id === currentUser.id}
                        title={user.id === currentUser.id ? '不能删除自己' : '删除'}
                      >
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Create/Edit Modal */}
      {showModal && (
        <div className="modal-overlay" onClick={() => setShowModal(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>{editingUser ? '编辑用户' : '创建用户'}</h3>
              <button className="modal-close" onClick={() => setShowModal(false)}>
                ×
              </button>
            </div>
            <form onSubmit={editingUser ? handleUpdate : handleCreate}>
              <div className="modal-body">
                <div className="form-group">
                  <label>用户名</label>
                  <input
                    type="text"
                    value={formData.username}
                    onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                    required
                  />
                </div>
                {!editingUser && (
                  <div className="form-group">
                    <label>密码</label>
                    <input
                      type="password"
                      value={formData.password}
                      onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                      required={!editingUser}
                    />
                  </div>
                )}
                <div className="form-group">
                  <label>角色</label>
                  <select
                    value={formData.role}
                    onChange={(e) => setFormData({ ...formData, role: e.target.value as 'admin' | 'viewer' })}
                  >
                    <option value="viewer">查看者</option>
                    <option value="admin">管理员</option>
                  </select>
                </div>
                <div className="form-group">
                  <label>邮箱</label>
                  <input
                    type="email"
                    value={formData.email}
                    onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn-secondary" onClick={() => setShowModal(false)}>
                  取消
                </button>
                <button type="submit" className="btn-primary">
                  {editingUser ? '保存' : '创建'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Reset Password Modal */}
      {showResetModal && (
        <div className="modal-overlay" onClick={() => setShowResetModal(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>重置密码</h3>
              <button className="modal-close" onClick={() => setShowResetModal(false)}>
                ×
              </button>
            </div>
            <form onSubmit={handleResetPassword}>
              <div className="modal-body">
                <div className="form-group">
                  <label>新密码</label>
                  <input
                    type="password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    required
                    minLength={4}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn-secondary" onClick={() => setShowResetModal(false)}>
                  取消
                </button>
                <button type="submit" className="btn-primary">
                  重置
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export default Users
