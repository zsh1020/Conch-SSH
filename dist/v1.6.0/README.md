# gssh v1.6.0 — Windows x64

## 安装

1. 下载 `gssh-v1.6.0-windows-amd64.zip`
2. 解压到任意目录
3. 双击 `gssh-v1.6.0-windows-amd64.exe` 运行

> **系统要求**：Windows 10 / Windows 11（WebView2 运行时已内置，无需额外安装）

---

## v1.6 新增功能

### 终端配色方案（12 套主题）

| 主题 | 风格 | 主题 | 风格 |
|------|------|------|------|
| GitHub Dark（默认）| 深蓝灰 | Nord | 北欧蓝灰 |
| Dracula | 紫黑 | Catppuccin Mocha | 柔和紫粉 |
| One Dark | 编辑器经典 | Material Dark | 青绿 |
| Monokai | 黄绿亮色 | Solarized Light | 米黄浅色 |
| Solarized Dark | 蓝绿暖色 | Dark Charcoal | 炭灰 `#1c1c1c` |
| Carbon | 碳黑 `#0c0c0c` | Pure Black | 纯黑 `#000000` |

切换配色方案时，**整个窗口背景色同步**（侧边栏、文件管理器、状态栏一并切换）。

### 字体下拉选择器

- 14 个预设字体（Cascadia Code / JetBrains Mono / Fira Code / Hack / Consolas / Monaco / Menlo / Ubuntu Mono / DejaVu Sans Mono 等）+ 自定义选项
- 实时预览效果

### 窗口几何持久化

- 关闭窗口时自动保存窗口尺寸和最大化状态
- 下次启动直接以保存的尺寸打开

### 美化的对话框

- 全新对话框组件（`info / error / confirm / prompt`），替代原生 alert/confirm/prompt
- 圆角卡片 + 弹出动画 + 主题色图标（ℹ️/⚠️/❓/✏️）
- 错误使用红色主题，确认操作蓝色按钮

### 文件预览改为独立弹窗

- 全屏遮罩 + 居中模态框（最宽 860px）
- 图片自适应居中，文本等宽字体高亮，二进制大图标提示
- Esc 键关闭

---

## v1.6 改进

| 项目 | 说明 |
|------|------|
| 设置按钮响应性 | 修复标题栏 wails-draggable 导致按钮偶发无响应 |
| 删除多余「主题」单选 | 配色方案已自动控制深/浅 UI，独立单选冗余 |
| 终端 ANSI 调色板 | 12 套主题 bright 变体差异化加亮，修复 Carbon 蓝色不可见、Pure Black 绿色刺眼 |
| 弹窗交互 | 所有弹窗禁止点击遮罩关闭，必须显式点击关闭按钮 |
| 远程文件列表 | 图标 / 文件名字号优化，紧凑行距，图标紧贴文件名 |
| 禁用应用区域原生右键 | 阻止 WebView2 默认菜单（刷新 / 另存为 / 查看源码 / 审查元素），自定义菜单不受影响 |

---

## 配置文件位置

- 会话列表：`%APPDATA%\gssh\sessions.json`
- 设置（含配色方案、字体、窗口尺寸）：`%APPDATA%\gssh\settings.json`
- 命令片段：`%APPDATA%\gssh\snippets.json`
- 已知主机：`~/.ssh/known_hosts`

---

## 升级建议

- 直接覆盖旧版本可执行文件，配置数据完全兼容（无需迁移）
- 升级后会自动应用上次保存的窗口尺寸和最大化状态
- 默认配色方案为 GitHub Dark，可在「设置 → 终端外观 → 配色方案」切换
