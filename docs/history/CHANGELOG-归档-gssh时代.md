# Changelog

本文件记录 Conch 的所有版本变更，遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/) 规范。

---

## [v1.9.3] — 2026-05-21

### 新增

#### 标签页右键菜单
- 右键标签可弹出操作菜单，包含三项：**复制**（同参数新开连接）、**重新连接**、**关闭**
- 复制：使用相同会话参数开启新标签页
- 重新连接：先断开当前连接再重建，适用于已连接或已断线状态；无连接记录的标签（快速连接未保存）置灰不可用
- 关闭：等同于点击标签上的 ✕ 按钮

### 优化

#### SFTP 上传按钮合并
- 工具栏/右键菜单「上传」从「上传文件 / 上传文件夹」二级菜单合并为单一按钮
- Windows 平台调用原生 `IFileOpenDialog`（COM），移除 `FOS_FILEMUSTEXIST` 标志，支持在同一对话框内同时选择文件或目录
- 非 Windows 平台调用 Wails 多文件选择器，通过 `LocalIsDir` 自动识别所选路径类型
- 所有上传路径统一经 `LocalIsDir` 判断后分别调用 `SFTPUpload` / `SFTPUploadDir`

#### 文件管理器右键菜单
- 「新建文件夹」改为「新建目录」，下载菜单项去掉前置 ⬇ 图标

#### 重新连接体验
- 右键重连时先将标签状态置为 `connecting`，再调用 `Disconnect`
- Terminal 收到 `closed` 事件时检测状态，重连中跳过断线遮罩，消除闪现弹窗

### 修复

- 修复 prompt 对话框 `defaultValue` 初始值在某些情况下未能预填的问题（改用 `watchEffect` 响应式预填）

---

## [v1.9.0] — 2026-05-18

### 新增

#### 文件管理器 — 彩色文件图标
- 引入 Font Awesome 7 Free 图标库，48 个 SVG 图标替换原有 emoji
- 编程语言文件使用官方品牌色（Go 青蓝、Python 蓝、JS 黄、TS 蓝、Rust 橙红、Vue 绿等）
- 支持特殊文件名识别（Dockerfile、Makefile、go.mod、Cargo.toml、package.json 等）
- 文件夹保留 📁 emoji，其余类型使用 SVG 图标

#### 布局调整
- 文件管理器移至终端左侧，会话列表移至窗口右侧
- 文件管理器从窗口顶部开始，TabBar 仅在终端列显示
- 服务器监控面板嵌入会话列表底部（替代原浮层弹窗），随活跃标签页切换自动刷新

#### 会话列表视觉优化
- 分组标题添加文件夹图标（展开 📂 / 收起 📁）
- 每个会话项显示对应的颜色色块
- 颜色生成改用黄金比例 HSL 算法，确保颜色最大分散、减少撞色

#### 文件管理器工具栏
- 路径输入框移至最左，导航按钮统一置于右侧
- 移除后退/前进按钮，保留上级目录和刷新

### 优化

#### SFTP 上传性能
- 启用 `UseConcurrentWrites` + `UseConcurrentReads` + `MaxConcurrentRequestsPerFile(64)` 并发传输
- 单文件上传使用 `ReadFrom` + `progressFileReader`（实现 `Size()`/`ReadAt()`）走并发写路径
- 目录上传改为 4 worker 并行，`sync/atomic` 累计进度
- 进度事件节流从 500ms 降至 200ms，加 `sync.Mutex` 保证并发安全

### 修复

- 修复拖拽上传在多标签场景下 `OnFileDrop` 注册错乱问题（仅活跃标签注册）
- 修复拖拽上传后文件列表选中状态未清除问题

---

## [v1.8.0] — 2026-05-07

### 新增

#### lrzsz / ZMODEM 文件传输
- 终端内置 ZMODEM 协议支持，兼容服务器端 `sz` / `rz` 命令
- `sz`：远端发送文件时终端底部弹出进度条，完成后弹出系统保存对话框
- `rz`：远端等待接收时弹出本地多文件选择框，逐文件分块发送
- 进度条显示方向（↓/↑）、文件名、百分比，支持一键取消
- 基于 `zmodem.js` 库，协议回包经 base64 通道保证二进制安全

#### 文件管理器 — 递归目录传输
- 新增「⬆ 目录」工具栏按钮，支持整个文件夹递归上传
- 拖拽文件夹到文件列表区域自动识别目录并递归上传（`webkitGetAsEntry` API）
- 右键菜单及批量下载按钮支持选中目录后整体递归下载
- 传输进度两阶段：预扫描统计总字节数，再逐文件累积更新进度条

#### 文件管理器 — 列宽拖拽调整
- 表头各列右边缘加拖拽 handle，拖动可精确调整列宽
- 双击 handle 重置为默认宽度
- 列宽持久化到 localStorage（`gssh-fm-col-widths`）

#### 终端 — 选中即复制
- 终端内鼠标选中文字后自动写入系统剪贴板
- 右键菜单「复制」保持不变，两种方式并存

#### 关于页面
- 设置对话框新增「关于」Tab
- 显示应用名、版本号（`v{{ AppVersion }}`）、技术栈标签、配置路径等信息
- 版本号从 Go `version.go` 统一管理，构建时可通过 `-ldflags "-X main.AppVersion=x.y.z"` 覆盖

### 优化

- **文件管理器工具栏**重新分为三行：导航行 / 过滤行（搜索框独占一行，不再被压缩）/ 操作行
- **设置对话框高度固定**（540px），切换 Tab 时不再跳动
- **右键菜单下载项**去掉 `↓` 前缀图标，视觉更简洁
- **端口转发面板**新增关闭按钮（emit close），点击背景遮罩也可关闭
- **阻止浏览器访问** Wails dev server（`blockBrowserMiddleware`，User-Agent 不含 `wails.io` 的请求返回 403）
- **终端失焦停止光标闪烁**，聚焦后按设置恢复，避免后台 tab 持续闪烁

### 修复

- 非活跃 tab 小蓝点误触发：新增 `lfCount >= 2` 条件，避免长主机名的提示符重绘误判为新输出

---

## [v1.7.0] — 2026-04-29

### 新增

#### 会话侧边栏分组拖拽排序
- 侧边栏分组 hover 时左侧出现 `⠿` 拖拽手柄
- 拖拽分组到目标位置后自动保存新顺序
- 分组顺序持久化到 `~/.config/gssh/sessions.json` 的 `group_order` 字段
- `⏱ 最近` 分组不可拖拽，始终在顶部
- 后端 `store.GroupOrder` 字段 + `SaveGroupOrder` 函数
- `GetGroups()` 优先返回显式排序，回退到 session 出现顺序

### 修复

#### 非活跃 tab 小圆点误触发
- 重写 `hasVisibleOutput`：逐字节扫描跳过所有 ANSI 控制序列
- 有 `\n` 或可见字符 > 25 才触发小圆点
- 准确过滤纯提示符重绘包（如 `[user@server ~]$ `），保留真实命令输出

---

## [v1.6.0] — 2026-04-28

### 新增

#### 终端配色方案（12 套主题）
- 设置 → 终端外观新增「配色方案」下拉，含 8 色调色板预览
- 内置主题：GitHub Dark / Dracula / One Dark / Monokai / Solarized Dark / Nord / Catppuccin Mocha / Material Dark / Solarized Light / Dark Charcoal `#1c1c1c` / Carbon `#0c0c0c` / Pure Black `#000000`
- 切换配色方案时**整个窗口背景色同步**（侧边栏、文件管理器、状态栏一并切换）
- Solarized Light 自动切换为浅色 UI 主题（`data-theme="light"`），保证文字可读
- 优化所有主题的 ANSI 调色板，bright 变体差异化加亮，特别修复 Carbon 蓝色 `#0037da`（深到不可见）和 Pure Black 绿色 `#55ff55`（刺眼）

#### 字体下拉选择器
- 字体设置由文本输入改为下拉选择，14 个预设字体 + "自定义…" 选项
- 实时预览（`AaBbCc 0123 if (x) { return; } → ≠ ≤ ≥`）
- 预设涵盖 Cascadia Code / JetBrains Mono / Fira Code / Hack / Source Code Pro / Consolas / Monaco / Menlo / Ubuntu Mono / DejaVu Sans Mono 等

#### 窗口几何持久化
- 关闭窗口时保存当前窗口尺寸和最大化状态到 `settings.json`
- 下次启动直接以保存的尺寸打开（无需用户重新拖拽）
- 新增 `MinWidth: 800, MinHeight: 600` 防止窗口缩得过小
- 配色方案 `color_scheme` 字段补入后端 `Settings` struct（此前前端发送但后端字段缺失，导致从未写入磁盘）

#### 美化错误弹窗（替代浏览器原生 alert / confirm / prompt）
- 新增 `stores/dialogs.ts` Pinia store + `components/DialogHost.vue`，提供 Promise 驱动的 `info / error / confirm / prompt` API
- 统一对话框样式：圆角卡片 + 弹出动画 + 主题色图标（ℹ️/⚠️/❓/✏️）
- error 类型标题红色 + danger 色按钮，confirm/prompt 蓝色主按钮 + 灰色取消按钮
- 全项目替换 14 处 `alert/confirm/prompt`：FileManager、SessionSidebar、TunnelPanel、SnippetDialog、App.vue 等

#### 文件预览改为独立弹窗
- 内联侧栏预览面板移除，改为 `<Teleport to="body">` 全屏遮罩 + 居中模态框
- 弹窗最宽 860px、最高 90vh，图片自适应居中，文本等宽字体高亮，二进制大图标提示
- 加载状态有旋转环，错误状态红色图标提示
- Esc 键关闭

### 改进

#### 设置按钮响应性
- 标题栏 `--wails-draggable: drag` 包裹整个区域导致按钮点击偶发被识别为拖拽
- `.title-actions` 容器加 `style="--wails-draggable: no-drag"` 明确排除按钮区域，点击 100% 触发

#### 删除多余的"主题"单选框
- 配色方案已自动控制 `data-theme`（浅色方案自动切浅色 UI），独立的深/浅色单选框冗余
- 设置对话框终端外观标签页移除该字段，UI 更精简

#### 终端文字配色优化
- 12 套主题的 ANSI 调色板全面优化，bright 颜色与 normal 区分度增强
- Carbon 主题：blue `#0037da`（深蓝近黑）→ `#3b78ff`，magenta `#881798` → `#b4009e`
- Pure Black 主题：green 从刺眼的 `#55ff55` → `#50fa7b`（Dracula 绿，更协调）
- One Dark / Monokai / Nord / Catppuccin / Material 等主题 bright 变体全部差异化加亮

#### 弹窗交互改进
- 所有弹窗（设置 / 预览 / 端口转发 / 监控 / 帮助 / 快速连接 / 会话编辑 / Snippet 管理 / 添加转发规则等）**禁止点击遮罩关闭**，必须显式点击关闭按钮（✕）或取消按钮
- 共修改 10 处 `@click.self` 行为，避免误点导致数据丢失

#### 远程文件列表样式
- 文件图标 13px → 14px（右对齐），文件名 / 正文字号 12px → 13px，表头 11px → 12px
- 单元格行距压缩为 3px（紧凑视图）
- 修复 CSS 优先级 bug：`.fm-table td` 特异性 `(0,0,1,1)` 高于纯类名 `(0,0,1,0)`，覆盖规则需用 `td.col-icon` / `td.fm-col-name` 提升特异性才能生效
- 图标列右 padding 0 + 文件名列左 padding 4px，图标紧贴文件名

### 安全 / 用户体验

#### 禁用应用区域原生右键菜单
- 应用根元素 `@contextmenu.prevent` 阻止 WebView2 默认右键菜单（刷新页面、另存为、查看源码、审查元素等）
- 终端、文件列表、会话侧边栏的自定义右键菜单不受影响（事件先被组件捕获）

### 变更文件

| 文件 | 类型 | 说明 |
|------|------|------|
| `main.go` | 修改 | 启动前读配置 + `MinWidth/MinHeight` + `OnBeforeClose` |
| `app.go` | 修改 | `startup` 恢复最大化、新增 `beforeClose` 保存窗口几何 |
| `backend/config/settings.go` | 修改 | 补 `ColorScheme`、`WindowWidth/Height/Maximized` 字段 |
| `frontend/src/stores/dialogs.ts` | 新增 | 对话框 store（info/error/confirm/prompt） |
| `frontend/src/components/DialogHost.vue` | 新增 | 全局对话框宿主 |
| `frontend/src/App.vue` | 修改 | 禁用右键、注入 DialogHost、移除点击遮罩关闭 |
| `frontend/src/components/Terminal.vue` | 修改 | 12 套 THEMES、`termBg` computed、ANSI 优化 |
| `frontend/src/components/SettingsDialog.vue` | 修改 | 配色方案选择器 + 调色板预览 + 字体下拉 + 删除主题单选 |
| `frontend/src/components/FileManager.vue` | 修改 | 预览弹窗化、对话框替换、文件列表样式优化 |
| `frontend/src/components/SessionSidebar.vue` | 修改 | 删除会话改用 dialog confirm |
| `frontend/src/components/TunnelPanel.vue` | 修改 | 错误提示改 dialog error |
| `frontend/src/components/SnippetDialog.vue` | 修改 | 删除片段改 dialog confirm |
| `frontend/src/components/SnippetPanel.vue` | 修改 | 移除点击遮罩关闭 |
| `frontend/src/components/ConnectDialog.vue` | 修改 | 移除点击遮罩关闭 |
| `frontend/src/components/SessionDialog.vue` | 修改 | 移除点击遮罩关闭 |
| `frontend/src/stores/settings.ts` | 修改 | 新增配色方案 SCHEME_BG 映射 + applyColorScheme |
| `frontend/src/style.css` | 修改 | 移除 `--term-bg` 硬编码、xterm-viewport `transparent !important` |

---

## [v1.5.0] — 2026-04-27

### 新增 / 改进

#### 文件管理器（FileManager.vue）体验优化
- **右键菜单新增「预览」选项**：取消单击自动预览，预览改为显式触发（右键 → 预览），避免误触加载大文件
- **路径栏可直接输入跳转**：点击工具栏路径文字 → 蓝色边框输入框（全选当前路径），输入目标路径后 Enter 跳转；Esc 取消
- **文件图标按后缀区分**（纯 Emoji，无外部依赖）：图片 🖼️、视频 🎬、音频 🎵、压缩包 📦、代码 📜、配置/脚本 ⚙️、PDF 📕、Office 📝/📊、文本 📄
- **「用本地编辑器打开」** 仅对文件显示，对目录右键时自动隐藏

#### 会话侧边栏精简
- 每条会话仅显示**会话名称**，去除 `username@host:port` 副标题行
- 服务器地址改为 `title` tooltip（悬停可见）
- 去除名称旁内联 `star-badge`，收藏状态由右侧 ★ 按钮统一呈现
- 行高从双行 ~40px 压缩至单行 ~24px，同屏可显示更多会话

### 变更文件

| 文件 | 类型 | 说明 |
|------|------|------|
| `frontend/src/components/FileManager.vue` | 修改 | 路径输入、图标映射、右键预览、预览触发方式 |
| `frontend/src/components/SessionSidebar.vue` | 修改 | 精简显示内容、减小行高 |

---

## [v1.4.0] — 2026-04-27

### 新增

#### 快速重连 / 断线遮罩（P0）
- 连接断开时终端中央显示遮罩，提供「重新连接」和「关闭标签」两个操作
- 「重新连接」复用上次连接参数（`lastConnectReq`），无需重新选择会话
- Tab 数据结构新增 `lastConnectReq`、`reconnectCount` 字段

#### 最近连接 & 会话收藏（P0）
- 侧边栏顶部「⏱ 最近」分组，展示最近使用的 5 条会话（按 `last_used` 排序）
- 时间显示相对格式（「刚刚」/ 「X 分钟前」 / 「X 小时前」/ 「X 天前」）
- 会话右侧 ★ 按钮（hover 显示）支持收藏/取消收藏，已收藏高亮金色
- 收藏状态持久化（`sessions.json` 新增 `starred` 字段）
- 右键菜单增加「收藏 ☆」/「取消收藏 ★」快捷操作

#### Snippet 参数化执行（P0）
- 命令片段支持 `{变量名}` / `{变量名:默认值}` 占位符语法
- 点击含参数片段时弹出内嵌填写弹窗，预填默认值，Enter 确认 / Esc 取消
- 含参片段在列表中显示 `{…}` 徽章标识

#### 标签页拖拽排序（P1）
- 标签栏支持 HTML5 拖拽排序，拖拽时高亮目标位置（蓝色竖线）
- 排序仅影响前端显示顺序，不影响后端连接

#### 监控历史折线图（P1）
- MonitorPanel 新增内联 SVG 折线图组件（无外部依赖）
- 维护 60 秒循环缓冲区（30 个采样点 × 2s 间隔）
- CPU、内存各一条折线；网络收发双线（rx 实线，tx 虚线）
- 面板宽度由 320px 扩展至 340px

#### 快捷键速查面板（P1）
- 标题栏 `?` 圆形按钮打开速查浮层，按 `?` 键或 `Esc` 关闭
- 分五类展示所有快捷键：标签页 / 终端 / 侧边栏 / 工具栏 / 通用

#### 会话 Tags 过滤（P2）
- 侧边栏搜索框下方新增 Tag 筛选条（有 Tag 时显示）
- 点击 Tag 过滤会话（AND 逻辑，支持多选），✕ 一键清除
- SessionDialog 表单 Tags 字段已支持逗号分隔输入

#### 端口转发规则预设库（P2）
- 「添加规则」对话框新增「快速预设」选项：MySQL / PostgreSQL / Redis / MongoDB / Kubernetes / HTTP 8080 / SOCKS5 动态代理
- 点击预设自动填入类型、本地地址、远端地址，可手动覆盖

#### 终端 URL 可点击（P2）
- 通过 `@xterm/addon-web-links` 实现，终端输出中的 URL Ctrl+Click 打开浏览器

### 变更文件

| 文件 | 类型 | 说明 |
|------|------|------|
| `backend/store/sessions.go` | 修改 | `Session.Starred bool` 字段 |
| `frontend/src/stores/tabs.ts` | 修改 | `ConnectRequest`、`lastConnectReq`、`reorderTab()` |
| `frontend/src/stores/sessions.ts` | 修改 | `recentSessions`、`starredSessions`、`allTags`、`toggleStar()` |
| `frontend/src/App.vue` | 修改 | 拖拽排序、帮助面板、`reconnect()` 重连逻辑 |
| `frontend/src/components/Terminal.vue` | 修改 | 断线遮罩、`doReconnect()` |
| `frontend/src/components/SessionSidebar.vue` | 修改 | 最近分组、星标、Tag 过滤条 |
| `frontend/src/components/SnippetPanel.vue` | 修改 | 参数解析、内嵌参数弹窗 |
| `frontend/src/components/MonitorPanel.vue` | 修改 | 历史折线图 `TrendChart` 组件 |
| `frontend/src/components/TunnelPanel.vue` | 修改 | 转发规则预设库 |

---

## [v1.3.0] — 2026-04-27

### 新增

#### 密码 AES-256-GCM 加密存储
- `sessions.json` 中密码字段以 `enc:<base64>` 格式加密存储
- 密钥由机器指纹（hostname + home 目录）SHA-256 派生，无需外部依赖
- 读取时自动解密，历史明文数据兼容（首次保存时自动迁移）

#### 已知主机指纹变更告警
- 检测到主机指纹变更时不再直接拒绝，而是弹出警告对话框
- 对话框显示旧指纹和新指纹，用户可选择「信任新指纹」或「拒绝连接」
- 选择信任后自动更新 `~/.ssh/known_hosts`，删除旧条目写入新条目

#### 命令片段（Snippet）管理
- 新增 `~/.config/gssh/snippets.json` 持久化存储
- 片段支持名称、分组、命令三个字段
- 标签栏「📋」按钮打开下拉面板，按分组展示片段，支持一键发送到终端
- 「管理」按钮打开完整 CRUD 对话框

#### 批量广播
- 标签栏「📢」按钮激活广播模式
- 顶部广播条：Tab 多选 + 命令输入框 + 发送按钮
- 激活时自动勾选所有已连接的标签页
- 按 Enter 或点击「发送」，命令同时发送到所有选中终端

#### 服务器状态监控
- 标签栏「📊」按钮打开监控面板
- 每 2 秒通过 SSH exec 采集一次 `/proc/stat`、`/proc/meminfo`、`/proc/net/dev`
- 单次 SSH channel 读取三段数据，用哨兵字符分割，减少 SSH 会话开销
- 展示 CPU 使用率、内存用量（含进度条）、网络收发速率
- 进度条颜色：正常（蓝）/ 警告 ≥60%（橙）/ 危险 ≥85%（红）
- 非 Linux 服务器（无 `/proc`）时显示友好提示，不报硬错误
- 关闭面板后自动停止后台采样 goroutine

### 修复

- 修复监控面板数据全为零但无错误提示的问题：原因是 `sh -c` 命令链末尾 `cat /proc/net/dev` 退出码非零导致 `sess.Output()` 误判失败，改为在命令末尾追加 `; true` 强制退出码为 0
- 修复设置面板「文件管理」标签页中复选框因全局 `input { width: 100% }` 样式被撑满的布局问题
- 修复端口转发「添加规则」对话框在特定布局下被截断的问题

### 变更文件

| 文件 | 类型 | 说明 |
|------|------|------|
| `backend/store/sessions.go` | 修改 | 密码加密/解密 |
| `backend/knownhosts/manager.go` | 修改 | 主机变更告警回调 + `removeHost()` |
| `backend/ssh/client.go` | 修改 | 透传 `OnChanged` 回调 |
| `backend/store/snippets.go` | 新增 | Snippet CRUD |
| `backend/monitor/collector.go` | 新增 | SSH exec 采样 + 解析 |
| `app.go` | 修改 | Snippet / Monitor Wails 方法 |
| `frontend/src/stores/snippets.ts` | 新增 | Snippet Pinia store |
| `frontend/src/components/SnippetPanel.vue` | 新增 | 快速发送下拉面板 |
| `frontend/src/components/SnippetDialog.vue` | 新增 | CRUD 对话框 |
| `frontend/src/components/MonitorPanel.vue` | 新增 | 实时监控面板 |
| `frontend/src/App.vue` | 修改 | 广播条 + 工具按钮组 |
| `frontend/src/components/SettingsDialog.vue` | 修改 | 复选框布局修复 |

---

## [v1.2.0] — 2026-04-24

### 新增
- 终端与文件管理器**同时显示**（左终端 + 右文件面板），移除视图切换按钮
- 文件面板宽度可拖拽调整（200–600px），宽度重启后记忆
- 系统文件选择框上传（工具栏「⬆」按钮，支持多选）
- 系统目录选择框选择下载目录（「…」按钮），下载目录持久化记忆
- 文件列表支持多列排序和可配置显示字段
- 端口转发「🔗」按钮移至标签栏右侧工具区
- `OpenDirectoryDialog()` 和 `GetDownloadsDir()` 后端方法

### 变更
- 文件管理器移除本地文件浏览栏，简化为纯远端操作面板
- 移除 Tab 的 `view` 字段和 `setView()` 方法

### 修复
- 布局重构中修复多处 flex 溢出和 z-index 层叠问题

---

## [v1.1.0] — 2026-04-23

### 新增
- 终端内搜索（Ctrl+F，`@xterm/addon-search`，实时高亮，上/下导航）
- 终端右键菜单（复制所选 / 粘贴 / 全选 / 清屏）
- 侧边栏宽度拖拽调整（160–480px，持久化到 localStorage）
- 远端文件预览面板（文本 / 图片 / 二进制三类渲染，点击文件自动加载）
- 本地文件拖拽上传（从 OS 资源管理器拖入文件面板）
- 本地默认程序编辑远端文件，文件保存后弹出「是否上传」对话框
- 编辑器监视支持多文件并发、手动停止（`fsnotify` + 800ms 去抖）
- 新增后端方法：`SFTPReadFile`、`OpenWithEditor`、`UploadEditedFile`、`StopEditorWatch`
- 新增事件：`editor:changed:{watchID}`

---

## [v1.0.0] — 2026-04-23

### 第五期（commit `4b22796`）

**新增**
- 深色 / 浅色主题实时切换（CSS 变量 + `data-theme` 属性）
- 终端字体族 / 字号 / 光标样式 / 闪烁 / 回滚行数个性化设置，`watch` 热更新
- 设置持久化到 `~/.config/gssh/settings.json`
- SSH 密钥生成（Ed25519 / RSA-4096），写入 `~/.ssh/`，公钥内键复制
- 从 `~/.ssh/config` 导入会话（Host / HostName / Port / User / IdentityFile / ProxyJump）
- AES-256-GCM 加密工具（`backend/config/keystore.go`，机器指纹派生密钥）
- `backend/config/settings.go`、`backend/sshconfig/importer.go`
- `frontend/src/stores/settings.ts`、`frontend/src/components/SettingsDialog.vue`

---

### 第四期（commit `16e7ab3`）

**新增**
- 本地端口转发 `-L`（`net.Listen` → `sshConn.Dial`）
- 远程端口转发 `-R`（`sshConn.Listen` → `net.Dial`）
- SOCKS5 动态代理 `-D`（内嵌 SOCKS5 握手，支持 IPv4 / IPv6 / 域名）
- ProxyJump 跳板机（`[user@]host[:port]` 格式，复用认证方式）
- 隧道并发连接计数（原子操作）
- SSH 断开时自动停止全部隧道
- 转发规则管理 UI（类型色区分：蓝 -L / 绿 -R / 橙 -D）
- `backend/tunnel/`（manager / local / remote / dynamic）
- `frontend/src/components/TunnelPanel.vue`

---

### 第三期（commit `ca30306`）

**新增**
- SFTP 文件管理器（复用已有 SSH 连接，SFTP 懒加载）
- 本地 / 远端双栏浏览，双击进入子目录
- 文件上传 / 下载，进度条实时显示（每 500ms 推送一次）
- 远端右键菜单：下载 / 重命名 / 删除
- 新建远端文件夹
- 传输完成 5 秒后自动清除队列条目
- 标签页「⌨ 终端 / 📁 文件」视图切换

---

### 第二期（commit `ea8a7d0`）

**新增**
- 会话列表按分组树形展示，分组可折叠/展开
- 搜索框过滤（会话名 / 主机 / 用户名）
- 右键菜单：连接 / 编辑 / 删除
- 双击会话直接连接
- 侧边栏折叠（220px ↔ 48px）
- 新建/编辑会话完整表单（名称 / 分组 / 认证方式 / ProxyJump / 标签）
- 多标签页：Ctrl+T 新建、Ctrl+W 关闭、Ctrl+Tab 切换、Ctrl+1~9 跳转
- 标签页连接状态指示

---

### 第一期（commit `4036e5f`）

**新增**
- Wails v2 + Vue 3 + TypeScript 项目骨架
- SSH 连接（密码 / 私钥 / SSH Agent 三种认证）
- PTY 分配 + Shell 启动 + Base64 双向 IO 泵
- xterm.js 终端渲染，ResizeObserver 尺寸同步
- TOFU 主机指纹验证（首次连接弹窗，写入 known_hosts）
- 30 秒保活心跳
- 会话 CRUD + JSON 持久化（`~/.config/gssh/sessions.json`）
- 快速连接对话框

