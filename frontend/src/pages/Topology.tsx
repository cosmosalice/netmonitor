import React, { useEffect, useRef, useState } from 'react'
import * as d3 from 'd3'
import { getTopology } from '../api/index'
import { theme } from '../theme'
import './Topology.css'

interface TopologyNode {
  id: string
  label: string
  type: string
  bytes: number
  flows: number
}

interface TopologyLink {
  source: string
  target: string
  bytes: number
  flows: number
}

interface TopologyData {
  nodes: TopologyNode[]
  links: TopologyLink[]
}

// Extended node type with position
interface SimNode extends TopologyNode {
  x: number
  y: number
}

interface SimLink {
  source: SimNode
  target: SimNode
  bytes: number
  flows: number
}

const Topology: React.FC = () => {
  const svgRef = useRef<SVGSVGElement>(null)
  const [data, setData] = useState<TopologyData>({ nodes: [], links: [] })
  const [loading, setLoading] = useState(true)
  const [tooltip, setTooltip] = useState<{ visible: boolean; x: number; y: number; data: TopologyNode | null }>({
    visible: false,
    x: 0,
    y: 0,
    data: null,
  })

  useEffect(() => {
    const fetchData = () => {
      getTopology(30).then((res: { nodes: TopologyNode[]; links: TopologyLink[] }) => {
        setData(res)
        setLoading(false)
      }).catch((err: Error) => {
        console.error('Failed to fetch topology:', err)
        setLoading(false)
      })
    }

    fetchData()
    const interval = setInterval(fetchData, 30000) // Refresh every 30 seconds
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    if (!svgRef.current || data.nodes.length === 0) return

    const svg = d3.select(svgRef.current)
    const width = svgRef.current.clientWidth
    const height = 600

    // Clear previous content
    svg.selectAll('*').remove()

    // Create container for zoom/pan
    const g = svg.append('g')

    // Zoom behavior
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.1, 4])
      .on('zoom', (event) => {
        g.attr('transform', event.transform)
      })

    svg.call(zoom)

    // Create extended nodes with positions
    const nodes: SimNode[] = data.nodes.map((node) => ({
      ...node,
      x: width / 2 + (Math.random() - 0.5) * 100,
      y: height / 2 + (Math.random() - 0.5) * 100,
    }))

    // Create node map for link references
    const nodeMap = new Map<string, SimNode>()
    nodes.forEach((node) => {
      nodeMap.set(node.id, node)
    })

    // Prepare links with node references
    const links: SimLink[] = data.links
      .filter((link) => nodeMap.has(link.source) && nodeMap.has(link.target))
      .map((link) => ({
        source: nodeMap.get(link.source)!,
        target: nodeMap.get(link.target)!,
        bytes: link.bytes,
        flows: link.flows,
      }))

    // Calculate max values for scaling
    const maxNodeBytes = Math.max(...nodes.map((n) => n.bytes), 1)
    const maxLinkBytes = Math.max(...links.map((l) => l.bytes), 1)

    // Create force simulation
    const simulation = d3.forceSimulation<SimNode>(nodes)
      .force('link', d3.forceLink<SimNode, SimLink>(links).id((d) => d.id).distance(150))
      .force('charge', d3.forceManyBody().strength(-400))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collision', d3.forceCollide<SimNode>().radius((d) => 8 + (d.bytes / maxNodeBytes) * 22 + 5))

    // Draw links
    const linkGroup = g.append('g')
      .attr('class', 'links')
      .selectAll('line')
      .data(links)
      .join('line')
      .attr('stroke', theme.colors.border)
      .attr('stroke-width', (d) => Math.max(1, Math.min(5, (d.bytes / maxLinkBytes) * 5)))
      .attr('stroke-opacity', 0.6)

    // Draw nodes
    const nodeGroup = g.append('g')
      .attr('class', 'nodes')
      .selectAll<SVGGElement, SimNode>('g')
      .data(nodes)
      .join('g')
      .attr('class', 'node')
      .style('cursor', 'pointer')

    // Add drag behavior
    const drag = d3.drag<SVGGElement, SimNode>()
      .on('start', (event, d) => {
        if (!event.active) simulation.alphaTarget(0.3).restart()
        d.x = event.x
        d.y = event.y
      })
      .on('drag', (event, d) => {
        d.x = event.x
        d.y = event.y
      })
      .on('end', (event) => {
        if (!event.active) simulation.alphaTarget(0)
      })

    nodeGroup.call(drag)

    // Node circles
    nodeGroup.append('circle')
      .attr('r', (d) => Math.max(8, Math.min(30, 8 + (d.bytes / maxNodeBytes) * 22)))
      .attr('fill', (d) => getNodeColor(d.type))
      .attr('stroke', '#fff')
      .attr('stroke-width', 2)
      .on('mouseover', (event, d) => {
        setTooltip({
          visible: true,
          x: event.pageX,
          y: event.pageY,
          data: d,
        })
      })
      .on('mouseout', () => {
        setTooltip({ visible: false, x: 0, y: 0, data: null })
      })

    // Node labels (IP addresses)
    nodeGroup.append('text')
      .text((d) => d.label)
      .attr('x', 0)
      .attr('y', (d) => Math.max(8, Math.min(30, 8 + (d.bytes / maxNodeBytes) * 22)) + 15)
      .attr('text-anchor', 'middle')
      .attr('fill', theme.colors.textPrimary)
      .attr('font-size', '10px')
      .attr('pointer-events', 'none')

    // Update positions on tick
    simulation.on('tick', () => {
      linkGroup
        .attr('x1', (d) => d.source.x)
        .attr('y1', (d) => d.source.y)
        .attr('x2', (d) => d.target.x)
        .attr('y2', (d) => d.target.y)

      nodeGroup.attr('transform', (d) => `translate(${d.x},${d.y})`)
    })

    // Cleanup
    return () => {
      simulation.stop()
    }
  }, [data])

  const getNodeColor = (type: string): string => {
    switch (type) {
      case 'pc':
        return theme.colors.sentryPurple
      case 'gateway':
        return theme.colors.lime
      case 'server':
        return theme.colors.warning
      case 'external':
        return theme.colors.error
      default:
        return theme.colors.pink
    }
  }

  const formatBytes = (bytes: number): string => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
    return (bytes / 1024 / 1024 / 1024).toFixed(1) + ' GB'
  }

  return (
    <div className="topology-page">
      <div className="topology-header">
        <h1>网络拓扑</h1>
        <p>基于活跃流量的网络节点关系图（每 30 秒自动刷新）</p>
      </div>

      <div className="topology-legend">
        <div className="legend-item">
          <span className="legend-dot" style={{ background: theme.colors.sentryPurple }}></span>
          <span>PC/终端</span>
        </div>
        <div className="legend-item">
          <span className="legend-dot" style={{ background: theme.colors.lime }}></span>
          <span>网关/路由器</span>
        </div>
        <div className="legend-item">
          <span className="legend-dot" style={{ background: theme.colors.warning }}></span>
          <span>服务器</span>
        </div>
        <div className="legend-item">
          <span className="legend-dot" style={{ background: theme.colors.error }}></span>
          <span>外部网络</span>
        </div>
      </div>

      <div className="topology-container">
        {loading ? (
          <div className="loading">加载中...</div>
        ) : data.nodes.length === 0 ? (
          <div className="no-data">暂无拓扑数据，请启动抓包</div>
        ) : (
          <svg
            ref={svgRef}
            style={{ width: '100%', height: '600px', background: theme.colors.bgPrimary }}
          />
        )}
      </div>

      {tooltip.visible && tooltip.data && (
        <div
          className="tooltip"
          style={{
            left: tooltip.x + 10,
            top: tooltip.y + 10,
          }}
        >
          <div className="tooltip-title">{tooltip.data.label}</div>
          <div className="tooltip-row">
            <span>类型:</span>
            <span>{tooltip.data.type}</span>
          </div>
          <div className="tooltip-row">
            <span>流量:</span>
            <span>{formatBytes(tooltip.data.bytes)}</span>
          </div>
          <div className="tooltip-row">
            <span>流数:</span>
            <span>{tooltip.data.flows}</span>
          </div>
        </div>
      )}
    </div>
  )
}

export default Topology
