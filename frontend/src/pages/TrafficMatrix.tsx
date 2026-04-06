import { useState, useEffect, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { getTrafficMatrix } from '../api'
import { theme } from '../theme'

// ── 颜色常量（Sentry 紫色调主题）──
const COLORS = {
  bg: theme.colors.bgPrimary,
  card: theme.colors.bgDeep,
  border: theme.colors.border,
  highlight: theme.colors.sentryPurple,
  text: theme.colors.textPrimary,
  textWhite: theme.colors.textPrimary,
  textDim: theme.colors.textDim,
}

// ── 热力图颜色阶梯（紫色系）──
const HEATMAP_STOPS = [
  { threshold: 0, color: 'rgba(66, 32, 130, 0.3)' },      // 无流量 - 深紫透明
  { threshold: 0.01, color: '#422082' },                  // 极低 - 深紫
  { threshold: 0.15, color: '#6a5fc1' },                  // 低 - 紫色
  { threshold: 0.35, color: '#79628c' },                  // 中低 - 亚述紫
  { threshold: 0.55, color: '#fa7faa' },                  // 中 - 粉色
  { threshold: 0.75, color: '#ffb287' },                  // 高 - 珊瑚色
  { threshold: 0.9, color: '#f44336' },                   // 极高 - 红
]

function getHeatColor(value: number, maxValue: number): string {
  if (maxValue <= 0 || value <= 0) return HEATMAP_STOPS[0].color
  const ratio = Math.min(value / maxValue, 1)
  for (let i = HEATMAP_STOPS.length - 1; i >= 0; i--) {
    if (ratio >= HEATMAP_STOPS[i].threshold) return HEATMAP_STOPS[i].color
  }
  return HEATMAP_STOPS[0].color
}

// ── 工具函数 ──
function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return (bytes / Math.pow(1024, i)).toFixed(2) + ' ' + units[i]
}

function nowUnix(): number {
  return Math.floor(Date.now() / 1000)
}

// ── 时间预设 ──
const TIME_PRESETS = [
  { label: '最近 1 小时', seconds: 3600 },
  { label: '最近 6 小时', seconds: 6 * 3600 },
  { label: '最近 24 小时', seconds: 24 * 3600 },
  { label: '最近 7 天', seconds: 7 * 86400 },
]

// ── 样式 ──
const styles: Record<string, React.CSSProperties> = {
  page: {
    padding: 24,
    background: COLORS.bg,
    minHeight: '100%',
    color: COLORS.text,
    fontFamily: theme.typography.fontFamily,
  },
  title: {
    fontSize: 22,
    fontWeight: 700,
    color: COLORS.textWhite,
    margin: '0 0 20px',
  },
  card: {
    background: COLORS.card,
    borderRadius: 12,
    padding: 20,
    border: `1px solid ${COLORS.border}`,
    marginBottom: 16,
  },
  cardTitle: {
    fontSize: 15,
    fontWeight: 600,
    marginBottom: 12,
    color: COLORS.textWhite,
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  presetBar: {
    display: 'flex',
    gap: 8,
    alignItems: 'center',
    flexWrap: 'wrap' as const,
    marginBottom: 16,
  },
  presetBtn: {
    padding: '6px 14px',
    borderRadius: 6,
    border: `1px solid ${COLORS.border}`,
    background: 'transparent',
    color: COLORS.text,
    cursor: 'pointer',
    fontSize: 13,
    transition: 'all 0.2s',
  },
  presetBtnActive: {
    padding: '6px 14px',
    borderRadius: 6,
    border: `1px solid ${COLORS.highlight}`,
    background: 'rgba(106, 95, 193, 0.12)',
    color: COLORS.highlight,
    cursor: 'pointer',
    fontSize: 13,
    fontWeight: 600,
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  input: {
    background: theme.colors.bgDeep,
    border: `1px solid ${COLORS.border}`,
    borderRadius: 6,
    padding: '6px 10px',
    color: COLORS.text,
    fontSize: 13,
    outline: 'none',
  },
  selectInput: {
    background: theme.colors.bgDeep,
    border: `1px solid ${COLORS.border}`,
    borderRadius: 6,
    padding: '6px 10px',
    color: COLORS.text,
    fontSize: 13,
    outline: 'none',
    minWidth: 100,
  },
  applyBtn: {
    padding: '6px 16px',
    borderRadius: 6,
    border: 'none',
    background: COLORS.highlight,
    color: '#fff',
    cursor: 'pointer',
    fontSize: 13,
    fontWeight: 600,
    textTransform: 'uppercase',
    letterSpacing: '0.2px',
  },
  placeholder: {
    color: COLORS.textDim,
    textAlign: 'center' as const,
    padding: 40,
  },
}

// ── Tooltip 组件 ──
interface TooltipData {
  src: string
  dst: string
  bytes: number
  flows: number
  protocols: string[]
  x: number
  y: number
}

// ── 主组件 ──
const TrafficMatrix = () => {
  const navigate = useNavigate()

  // Time range
  const [activePreset, setActivePreset] = useState(0) // default 1h
  const [startTime, setStartTime] = useState(() => nowUnix() - 3600)
  const [endTime, setEndTime] = useState(() => nowUnix())
  const [customStart, setCustomStart] = useState('')
  const [customEnd, setCustomEnd] = useState('')

  // Controls
  const [groupBy, setGroupBy] = useState<'host' | 'subnet'>('host')
  const [limit, setLimit] = useState(20)

  // Data
  const [nodes, setNodes] = useState<string[]>([])
  const [matrix, setMatrix] = useState<number[][]>([])
  const [details, setDetails] = useState<Record<string, { bytes: number; flows: number; protocols: string[] }>>({})
  const [loading, setLoading] = useState(false)
  const [maxValue, setMaxValue] = useState(0)

  // Tooltip
  const [tooltip, setTooltip] = useState<TooltipData | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // ── 时间范围切换 ──
  const selectPreset = useCallback((idx: number) => {
    const end = nowUnix()
    const start = end - TIME_PRESETS[idx].seconds
    setActivePreset(idx)
    setStartTime(start)
    setEndTime(end)
    setCustomStart('')
    setCustomEnd('')
  }, [])

  const applyCustomRange = useCallback(() => {
    if (!customStart || !customEnd) return
    const s = Math.floor(new Date(customStart).getTime() / 1000)
    const e = Math.floor(new Date(customEnd).getTime() / 1000)
    if (s >= e) return
    setActivePreset(-1)
    setStartTime(s)
    setEndTime(e)
  }, [customStart, customEnd])

  // ── 数据获取 ──
  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const data = await getTrafficMatrix({ start: startTime, end: endTime, limit, group_by: groupBy })
      const n = data?.nodes || []
      const m = data?.matrix || []
      const d = data?.details || {}
      setNodes(n)
      setMatrix(m)
      setDetails(d)

      // Calculate max value for color scaling
      let max = 0
      for (const row of m) {
        for (const val of row) {
          if (val > max) max = val
        }
      }
      setMaxValue(max)
    } catch {
      setNodes([])
      setMatrix([])
      setDetails({})
      setMaxValue(0)
    }
    setLoading(false)
  }, [startTime, endTime, limit, groupBy])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  // ── Cell handlers ──
  const handleCellHover = (rowIdx: number, colIdx: number, e: React.MouseEvent) => {
    const src = nodes[rowIdx]
    const dst = nodes[colIdx]
    const key = `${src}->${dst}`
    const detail = details[key]
    const val = matrix[rowIdx]?.[colIdx] || 0

    if (!containerRef.current) return
    const rect = containerRef.current.getBoundingClientRect()

    setTooltip({
      src,
      dst,
      bytes: detail?.bytes ?? val,
      flows: detail?.flows ?? 0,
      protocols: detail?.protocols ?? [],
      x: e.clientX - rect.left + 12,
      y: e.clientY - rect.top + 12,
    })
  }

  const handleCellClick = (rowIdx: number, colIdx: number) => {
    const src = nodes[rowIdx]
    const dst = nodes[colIdx]
    // Navigate to History page flows tab with src_ip and dst_ip pre-filled
    navigate(`/history?tab=flows&src_ip=${encodeURIComponent(src)}&dst_ip=${encodeURIComponent(dst)}`)
  }

  const LABEL_WIDTH = 120
  const CELL_SIZE = nodes.length > 15 ? 28 : 36

  return (
    <div style={styles.page}>
      <h1 style={styles.title}>流量矩阵</h1>

      {/* Control Bar */}
      <div style={styles.card}>
        <div style={styles.presetBar}>
          {TIME_PRESETS.map((p, i) => (
            <button
              key={i}
              style={activePreset === i ? styles.presetBtnActive : styles.presetBtn}
              onClick={() => selectPreset(i)}
            >
              {p.label}
            </button>
          ))}
          <span style={{ color: COLORS.textDim, margin: '0 6px' }}>|</span>
          <input
            type="datetime-local"
            style={styles.input}
            value={customStart}
            onChange={(e) => setCustomStart(e.target.value)}
          />
          <span style={{ color: COLORS.textDim }}>至</span>
          <input
            type="datetime-local"
            style={styles.input}
            value={customEnd}
            onChange={(e) => setCustomEnd(e.target.value)}
          />
          <button style={styles.applyBtn} onClick={applyCustomRange}>应用</button>
        </div>
        <div style={{ display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <span style={{ color: COLORS.textDim, fontSize: 13 }}>分组方式:</span>
            {(['host', 'subnet'] as const).map((g) => (
              <button
                key={g}
                style={groupBy === g ? styles.presetBtnActive : styles.presetBtn}
                onClick={() => setGroupBy(g)}
              >
                {g === 'host' ? 'Host (IP)' : 'Subnet (/24)'}
              </button>
            ))}
          </div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <span style={{ color: COLORS.textDim, fontSize: 13 }}>显示数量:</span>
            <select
              style={styles.selectInput}
              value={limit}
              onChange={(e) => setLimit(Number(e.target.value))}
            >
              <option value={10}>Top 10</option>
              <option value={20}>Top 20</option>
              <option value={30}>Top 30</option>
            </select>
          </div>
        </div>
      </div>

      {/* Heatmap */}
      <div style={styles.card}>
        <div style={styles.cardTitle}>
          <span>流量矩阵热力图</span>
          {nodes.length > 0 && (
            <span style={{
              fontSize: 11,
              padding: '2px 8px',
              borderRadius: 4,
              background: COLORS.border,
              color: COLORS.highlight,
              fontWeight: 500,
            }}>
              {nodes.length} × {nodes.length}
            </span>
          )}
        </div>

        {loading ? (
          <div style={styles.placeholder}>加载中...</div>
        ) : nodes.length === 0 ? (
          <div style={styles.placeholder}>暂无数据，请确保有抓包数据后重试</div>
        ) : (
          <div ref={containerRef} style={{ position: 'relative', overflowX: 'auto', overflowY: 'auto' }}>
            {/* Column labels */}
            <div style={{ display: 'flex', marginLeft: LABEL_WIDTH, marginBottom: 4 }}>
              {nodes.map((node, i) => (
                <div
                  key={i}
                  style={{
                    width: CELL_SIZE,
                    minWidth: CELL_SIZE,
                    height: LABEL_WIDTH - 20,
                    display: 'flex',
                    alignItems: 'flex-end',
                    justifyContent: 'flex-start',
                    overflow: 'hidden',
                  }}
                >
                  <span
                    style={{
                      transform: 'rotate(-55deg)',
                      transformOrigin: 'left bottom',
                      fontSize: 10,
                      color: COLORS.textDim,
                      whiteSpace: 'nowrap',
                      fontFamily: theme.typography.fontFamilyMono,
                      display: 'block',
                      maxWidth: 100,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    {node}
                  </span>
                </div>
              ))}
            </div>

            {/* Matrix rows */}
            {matrix.map((row, rowIdx) => (
              <div key={rowIdx} style={{ display: 'flex', alignItems: 'center' }}>
                {/* Row label */}
                <div
                  style={{
                    width: LABEL_WIDTH,
                    minWidth: LABEL_WIDTH,
                    paddingRight: 8,
                    textAlign: 'right',
                    fontSize: 10,
                    fontFamily: theme.typography.fontFamilyMono,
                    color: COLORS.textDim,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                  title={nodes[rowIdx]}
                >
                  {nodes[rowIdx]}
                </div>
                {/* Cells */}
                {row.map((val, colIdx) => (
                  <div
                    key={colIdx}
                    style={{
                      width: CELL_SIZE,
                      height: CELL_SIZE,
                      minWidth: CELL_SIZE,
                      background: getHeatColor(val, maxValue),
                      border: '1px solid rgba(255,255,255,0.05)',
                      cursor: val > 0 ? 'pointer' : 'default',
                      transition: 'transform 0.1s, box-shadow 0.1s',
                      borderRadius: 2,
                    }}
                    onMouseEnter={(e) => handleCellHover(rowIdx, colIdx, e)}
                    onMouseMove={(e) => handleCellHover(rowIdx, colIdx, e)}
                    onMouseLeave={() => setTooltip(null)}
                    onClick={() => val > 0 && handleCellClick(rowIdx, colIdx)}
                  />
                ))}
              </div>
            ))}

            {/* Tooltip */}
            {tooltip && (
              <div
                style={{
                  position: 'absolute',
                  left: tooltip.x,
                  top: tooltip.y,
                  background: 'rgba(21, 15, 35, 0.97)',
                  border: `1px solid ${COLORS.highlight}`,
                  borderRadius: 8,
                  padding: '10px 14px',
                  fontSize: 12,
                  color: COLORS.text,
                  pointerEvents: 'none',
                  zIndex: 100,
                  minWidth: 180,
                  boxShadow: '0 4px 20px rgba(0,0,0,0.5)',
                }}
              >
                <div style={{ fontWeight: 600, color: COLORS.textWhite, marginBottom: 6, fontSize: 13 }}>
                  {tooltip.src} → {tooltip.dst}
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, marginBottom: 3 }}>
                  <span style={{ color: COLORS.textDim }}>流量:</span>
                  <span style={{ color: COLORS.highlight, fontWeight: 600 }}>{formatBytes(tooltip.bytes)}</span>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, marginBottom: 3 }}>
                  <span style={{ color: COLORS.textDim }}>流数:</span>
                  <span>{tooltip.flows}</span>
                </div>
                {tooltip.protocols.length > 0 && (
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16 }}>
                    <span style={{ color: COLORS.textDim }}>协议:</span>
                    <span>{tooltip.protocols.join(', ')}</span>
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Color Legend */}
      <div style={styles.card}>
        <div style={styles.cardTitle}>颜色图例</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ fontSize: 12, color: COLORS.textDim }}>低</span>
          <div style={{ display: 'flex', flex: 1, height: 18, borderRadius: 4, overflow: 'hidden' }}>
            {['#422082', '#6a5fc1', '#79628c', '#fa7faa', '#ffb287', '#f44336'].map((c, i) => (
              <div key={i} style={{ flex: 1, background: c }} />
            ))}
          </div>
          <span style={{ fontSize: 12, color: COLORS.textDim }}>高</span>
          {maxValue > 0 && (
            <span style={{ fontSize: 11, color: COLORS.textDim, marginLeft: 8 }}>
              最大值: {formatBytes(maxValue)}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}

export default TrafficMatrix
