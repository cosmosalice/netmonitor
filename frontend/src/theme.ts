export const theme = {
  colors: {
    bgPrimary: '#1f1633',       // 主深紫背景
    bgDeep: '#150f23',          // 深层紫背景
    bgCard: '#1f1633',          // 卡片背景（同主背景或稍亮）
    sentryPurple: '#6a5fc1',    // 主交互色、链接、悬停
    assyrian: '#79628c',        // 按钮背景、次级交互
    deepViolet: '#422082',      // 下拉菜单、活跃态
    lime: '#c2ef4e',            // 高可见性 CTA、徽章
    coral: '#ffb287',           // 焦点背景
    pink: '#fa7faa',            // 焦点轮廓
    textPrimary: '#ffffff',     // 主要文本
    textSecondary: '#e5e7eb',   // 次要文本
    textDim: '#8b8fa3',         // 暗淡文本
    codeYellow: '#dcdcaa',      // 代码语法高亮
    glassWhite: 'rgba(255, 255, 255, 0.18)',  // 毛玻璃按钮
    glassDeep: 'rgba(54, 22, 107, 0.14)',     // 悬停覆盖层
    border: '#362d59',          // 深色边框
    borderLight: '#584674',     // 亮色边框
    success: '#c2ef4e',         // 成功（石灰绿）
    error: '#ff5252',           // 错误
    warning: '#ffb287',         // 警告（珊瑚色）
    // 图表配色（紫色系为主）
    chartColors: ['#6a5fc1', '#c2ef4e', '#fa7faa', '#ffb287', '#79628c', '#422082', '#e5e7eb', '#dcdcaa'],
    // 协议配色（保持多样性但紫色调优先）
    protocolColors: ['#6a5fc1', '#c2ef4e', '#fa7faa', '#ffb287', '#79628c', '#422082', '#00d4ff', '#ff6384', '#36a2eb', '#9966ff'],
  },
  shadows: {
    sunken: 'rgba(0, 0, 0, 0.1) 0px 1px 3px 0px inset',           // 内凹按钮
    surface: 'rgba(0, 0, 0, 0.08) 0px 2px 8px',                    // 玻璃按钮
    elevated: 'rgba(0, 0, 0, 0.1) 0px 10px 15px -3px',             // 卡片
    prominent: 'rgba(0, 0, 0, 0.18) 0px 0.5rem 1.5rem',            // 悬停态
    ambient: 'rgba(22, 15, 36, 0.9) 0px 4px 4px 9px',              // 环境微光
    inputFocus: 'rgba(0, 0, 0, 0.15) 0px 2px 10px inset',          // 输入焦点
  },
  radii: {
    xs: '6px',    // 表单输入
    sm: '8px',    // 按钮、卡片
    md: '10px',   // 大容器
    lg: '12px',   // 玻璃面板
    pill: '13px', // 亚述按钮
    full: '18px', // 药丸形
  },
  typography: {
    fontFamily: "Rubik, -apple-system, system-ui, 'Segoe UI', Helvetica, Arial, sans-serif",
    fontFamilyDisplay: "'Dammit Sans', Rubik, sans-serif",
    fontFamilyMono: "Monaco, Menlo, 'Ubuntu Mono', monospace",
  },
  glass: {
    background: 'rgba(255, 255, 255, 0.18)',
    backdropFilter: 'blur(18px) saturate(180%)',
  },
}

// 同时导出便捷的颜色常量（方便页面内联样式直接引用）
export const COLORS = theme.colors
export const SHADOWS = theme.shadows
