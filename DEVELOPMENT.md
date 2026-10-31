# Conch 开发文档

> **项目仓库**：https://github.com/zsh1020/Conch-SSH  
> **当前版本**：v1.0.0（开源首发，版本号重新起算）  
> **文档更新**：2026-04-29

---

## 目录

1. [项目简介](#1-项目简介)
2. [技术架构](#2-技术架构)
3. [项目目录结构](#3-项目目录结构)
4. [核心模块说明](#4-核心模块说明)
5. [数据结构与接口](#5-数据结构与接口)
6. [前后端通信协议](#6-前后端通信协议)
7. [开发环境搭建](#7-开发环境搭建)
8. [构建与运行](#8-构建与运行)
9. [分期进度总览](#9-分期进度总览)
10. [已完成功能记录（历史存档）](#10-已完成功能记录历史存档)
19. [后续版本规划](#19-后续版本规划)

---

## 1. 项目简介

**Conch**（🐚）是一款使用 Go 语言开发的跨平台 SSH 桌面客户端，目标是提供主流专业终端工具级别的体验。

| 特性 | 说明 |
|------|------|
| 界面类型 | 桌面 GUI（Wails v2 + WebView） |
| 终端渲染 | xterm.js（业界标准，支持全彩、Unicode） |
| SSH 协议 | 纯 Go 实现，无系统 ssh 依赖 |
| 跨平台 | Windows / macOS / Linux |
| 配置存储 | `~/.config/conch/sessions.json` |

---

## 2. 技术架构

```
┌─────────────────────────────────────────────────────────┐
│                    桌面窗口（Wails v2）                   │
│                                                         │
│  ┌────────────────────────────────────────────────────┐ │
│  │                 前端（WebView）                      │ │
│  │                                                    │ │
│  │     Vue 3 + TypeScript + TailwindCSS + Pinia       │ │
│  │                                                    │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────┐  │ │
│  │  │SessionSidebar│  │   Terminal   │  │ Session │  │ │
│  │  │  会话树/搜索  │  │  xterm.js   │  │ Dialog  │  │ │
│  │  └──────────────┘  └──────────────┘  └─────────┘  │ │
│  │  ┌────────────────────────────────────────────┐    │ │
│  │  │  Pinia Store: sessions.ts / tabs.ts        │    │ │
│  │  └────────────────────────────────────────────┘    │ │
│  └──────────────────────┬─────────────────────────────┘ │
│                         │  Wails Bindings（自动生成 TS）  │
│  ┌──────────────────────▼─────────────────────────────┐ │
│  │                  后端（Go）                          │ │
│  │                                                    │ │
│  │  app.go ←→ backend/ssh/   ←→ SSH Server            │ │
│  │            backend/auth/                           │ │
│  │            backend/knownhosts/                     │ │
│  │            backend/store/   ←→ ~/.config/conch/     │ │
│  │            backend/sftp/    （三期）                 │ │
│  │            backend/tunnel/  （四期）                 │ │
│  └────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 技术栈明细

| 层次 | 技术 | 版本 | 用途 |
|------|------|------|------|
| 桌面框架 | Wails v2 | v2.12.0 | Go 后端 + WebView，跨平台桌面应用 |
| 前端框架 | Vue 3 | ^3.2.37 | 响应式 UI 组件 |
| 类型系统 | TypeScript | ^4.6.4 | 前端类型安全 |
| 样式 | TailwindCSS | ^3.x + PostCSS | 原子化 CSS |
| 状态管理 | Pinia | ^3.0.4 | 全局会话/标签页状态 |
| 终端渲染 | xterm.js | ^6.0.0 | SSH 终端模拟器 |
| 终端自适应 | @xterm/addon-fit | ^0.11.0 | 终端尺寸自动适应 |
| 链接识别 | @xterm/addon-web-links | ^0.12.0 | 终端内 URL 可点击 |
| 图标库 | lucide-vue-next | ^1.0.0 | UI 图标（三期起使用） |
| SSH 协议 | golang.org/x/crypto/ssh | v0.50.0 | SSH 协议核心 |
| Known Hosts | golang.org/x/crypto/ssh/knownhosts | v0.50.0 | 主机指纹验证 |
| SSH Agent | golang.org/x/crypto/ssh/agent | v0.50.0 | Agent 转发认证 |
| SFTP | github.com/pkg/sftp | v1.13.10 | 文件传输（三期实现） |
| UUID | github.com/google/uuid | v1.6.0 | 连接 ID 生成 |
| 构建工具 | Vite | v3.2.11 | 前端打包 |

---

## 3. 项目目录结构

```
conch/
├── main.go                        # Wails 入口，窗口参数（1024×768）
├── app.go                         # 所有暴露给前端的 Go 方法（Wails Bindings）
├── wails.json                     # Wails 项目配置
├── go.mod / go.sum                # Go 依赖
├── .gitignore
├── DEVELOPMENT.md                 # 本文档
│
├── backend/                       # Go 后端业务逻辑
│   ├── auth/
│   │   └── methods.go             # 认证构建：密码/私钥/SSH Agent
│   ├── config/
│   │   ├── settings.go            # 全局设置 CRUD（字体/字号/主题/光标等）
│   │   └── keystore.go            # AES-256-GCM 加密工具（机器指纹派生密钥）
│   ├── editor/
│   │   └── watcher.go             # fsnotify 文件监视器（本地编辑器集成）
│   ├── knownhosts/
│   │   └── manager.go             # known_hosts TOFU 策略 + 指纹变更告警回调
│   ├── monitor/
│   │   └── collector.go           # ★ v1.3 新增：SSH exec 采样 /proc，CPU/内存/网络
│   ├── ssh/
│   │   ├── client.go              # SSH Dial + 连接保活（30s keepalive）
│   │   └── session.go             # PTY 分配、Shell 启动、双向 IO 泵
│   ├── sftp/
│   │   ├── client.go              # SFTP 连接封装
│   │   └── ops.go                 # List/Upload/Download/Mkdir/Delete/Rename
│   ├── sshconfig/
│   │   └── importer.go            # 解析 ~/.ssh/config → Session 列表
│   ├── tunnel/
│   │   ├── manager.go             # Tunnel 生命周期管理
│   │   ├── local.go               # 本地转发 -L
│   │   ├── remote.go              # 远程转发 -R
│   │   └── dynamic.go             # SOCKS5 动态转发 -D
│   └── store/
│       ├── sessions.go            # Session CRUD + JSON 持久化 + 密码加密
│       └── snippets.go            # ★ v1.3 新增：Snippet CRUD
│
├── frontend/
│   ├── index.html
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   ├── postcss.config.js
│   │
│   └── src/
│       ├── main.ts                # Vue 入口，注册 Pinia
│       ├── App.vue                # 根组件：标题栏 + 侧边栏 + 标签页 + 终端区
│       ├── style.css              # 全局样式 + CSS 变量设计令牌（深/浅双主题）
│       │
│       ├── stores/
│       │   ├── sessions.ts        # 会话列表 Pinia store
│       │   ├── tabs.ts            # 标签页 Pinia store
│       │   ├── settings.ts        # 应用设置 Pinia store
│       │   └── snippets.ts        # ★ v1.3 新增：Snippet Pinia store
│       │
│       └── components/
│           ├── Terminal.vue       # xterm.js 终端，ResizeObserver 尺寸同步
│           ├── ConnectDialog.vue  # 快速连接对话框（无需保存会话）
│           ├── SessionDialog.vue  # 新建/编辑会话完整表单
│           ├── SessionSidebar.vue # 左侧会话树（分组/搜索/右键菜单）
│           ├── FileManager.vue    # SFTP 远端文件管理器（含拖拽/预览/编辑器集成）
│           ├── SettingsDialog.vue # 设置面板（外观/密钥生成/导入）
│           ├── TunnelPanel.vue    # 端口转发管理面板
│           ├── MonitorPanel.vue   # ★ v1.3 新增：CPU/内存/网络实时监控
│           ├── SnippetPanel.vue   # ★ v1.3 新增：快速命令发送下拉面板
│           └── SnippetDialog.vue  # ★ v1.3 新增：命令片段 CRUD 对话框
│
└── build/                         # Wails 打包资源
    ├── appicon.png
    ├── windows/icon.ico / info.json / installer/
    └── darwin/Info.plist
```

> ★ 标注的目录/文件尚未创建，为后续各期预留占位。

---

## 4. 核心模块说明

### 4.1 `app.go` — Wails 绑定层

所有从前端 JavaScript 调用的 Go 方法集中定义于此，Wails 自动生成 TypeScript 绑定。

| 方法 | 签名 | 说明 | 期次 |
|------|------|------|------|
| `GetSessions` | `() → ([]Session, error)` | 获取全部已保存会话 | 一期 |
| `GetGroups` | `() → ([]string, error)` | 获取所有分组名称 | 二期 |
| `SaveSession` | `(Session) → error` | 新增或更新会话 | 一期 |
| `DeleteSession` | `(id) → error` | 删除会话 | 一期 |
| `Connect` | `(ConnectRequest, cols, rows) → (connID, error)` | 建立 SSH 连接启动 Shell | 一期 |
| `SendInput` | `(connID, data) → error` | 发送键盘输入（UTF-8） | 一期 |
| `SendInputBase64` | `(connID, data) → error` | 发送 Base64 二进制数据 | 一期 |
| `ResizeTerminal` | `(connID, cols, rows) → error` | 同步 PTY 尺寸 | 一期 |
| `Disconnect` | `(connID) → error` | 关闭 SSH 连接 | 一期 |
| `SFTPList` | `(connID, path) → ([]FileInfo, error)` | 列出远端目录 | 三期 |
| `SFTPUpload` | `(connID, local, remote) → error` | 上传文件 | 三期 |
| `SFTPDownload` | `(connID, remote, local) → error` | 下载文件 | 三期 |
| `SFTPDelete` | `(connID, path) → error` | 删除远端文件/目录 | 三期 |
| `SFTPMkdir` | `(connID, path) → error` | 创建远端目录 | 三期 |
| `SFTPRename` | `(connID, old, new) → error` | 重命名/移动 | 三期 |
| `SFTPReadFile` | `(connID, path, maxBytes) → (string, error)` | 读取远端文件（Base64，最多 512 KB） | v1.1 |
| `OpenWithEditor` | `(connID, path) → (watchID, error)` | 临时下载并用系统程序打开 | v1.1 |
| `UploadEditedFile` | `(connID, watchID) → error` | 上传本地编辑后的文件 | v1.1 |
| `StopEditorWatch` | `(watchID)` | 停止 fsnotify 监视 | v1.1 |
| `StartTunnel` | `(connID, TunnelConfig) → (tunnelID, error)` | 启动端口转发 | 四期 |
| `StopTunnel` | `(tunnelID) → error` | 停止端口转发 | 四期 |
| `ListTunnels` | `(connID) → ([]TunnelStatus, error)` | 列出转发规则 | 四期 |
| `GetSettings` | `() → (Settings, error)` | 获取应用设置 | 五期 |
| `SaveSettings` | `(Settings) → error` | 保存应用设置 | 五期 |
| `GenerateKey` | `(keyType, comment, passphrase) → (pubKey, error)` | 生成 SSH 密钥对 | 五期 |
| `ImportSSHConfig` | `(path) → ([]Session, error)` | 导入 ~/.ssh/config | 五期 |
| `LocalList` | `(path) → ([]LocalFileInfo, error)` | 列出本地目录 | 三期 |
| `LocalHomePath` | `() → (string, error)` | 返回用户 Home 路径 | 三期 |
| `OpenDirectoryDialog` | `() → (string, error)` | 系统目录选择框 | v1.2 |
| `GetDownloadsDir` | `() → (string, error)` | 返回 ~/Downloads 路径 | v1.2 |
| `GetSnippets` | `() → ([]Snippet, error)` | 获取所有命令片段 | **v1.3** |
| `SaveSnippet` | `(Snippet) → error` | 新增或更新命令片段 | **v1.3** |
| `DeleteSnippet` | `(id) → error` | 删除命令片段 | **v1.3** |
| `StartMonitor` | `(connID) → error` | 启动服务器状态监控 | **v1.3** |
| `StopMonitor` | `(connID)` | 停止服务器状态监控 | **v1.3** |

---

### 4.2 `backend/ssh/` — SSH 核心

**client.go**
```
Dial(ctx, ConnectOptions) → *Client
  ├── auth.BuildMethods()        构建认证列表
  ├── knownhosts.New()           指纹验证器
  ├── net.Dialer.DialContext()   TCP 连接（15s 超时）
  ├── gossh.NewClientConn()      SSH 握手
  └── go startKeepalive()        保活协程（30s 间隔）
```

**session.go**
```
StartShell(client, cols, rows, outputFn) → *Session
  ├── RequestPty("xterm-256color", rows, cols, modes)
  ├── StdoutPipe / StderrPipe / StdinPipe
  ├── Shell()
  └── go pump(stdout/stderr) → base64 → outputFn("")=EOF 哨兵
```

---

### 4.3 `backend/auth/` — 认证方法

| `authType` | 认证方式 | 实现细节 |
|------------|----------|----------|
| `"password"` | 密码 | `ssh.Password(password)` |
| `"key"` | 公钥 | 读取文件，支持带 passphrase；`~` 路径自动展开 |
| `"agent"` | SSH Agent | 连接 `SSH_AUTH_SOCK` Unix socket |
| 空/其他 | 自动尝试 | Agent → 私钥 → 密码，逐一尝试 |

---

### 4.4 `backend/knownhosts/` — 主机指纹管理

TOFU 策略：
- **已知且匹配** → 直接通过
- **指纹冲突** → 始终拒绝，显示 MITM 警告
- **未知主机** → 调用 `ConfirmFunc`，由 `app.go` 通过 `runtime.MessageDialog` 弹窗确认，确认后追加 `~/.ssh/known_hosts`
- **指纹已变更** → 调用 `HostKeyChangedFunc`（v1.3 新增），弹出警告对话框显示新旧指纹；用户选择「信任」后删除旧条目、写入新条目；选择「拒绝」则连接失败

---

### 4.5 `backend/store/` — 会话持久化

- **存储路径**：`~/.config/conch/sessions.json`（会话）、`~/.config/conch/snippets.json`（片段）
- **并发安全**：`sync.RWMutex` 保护文件读写
- **接口**：`Init / GetAll / GetByID / GetGroups / Save / Delete / TouchLastUsed`
- **密码加密**（v1.3）：`Save()` 写盘前对 `Password` 字段 AES-256-GCM 加密（前缀 `enc:`），`Load()` 读取后自动解密；使用写时复制（copy-on-write）避免修改内存中的 Session 对象

---

### 4.6 前端 Pinia Stores

**`stores/sessions.ts`**

```typescript
// 对外暴露
sessions: Session[]           // 全部会话列表
groups: string[]              // 所有分组名
sessionsByGroup: computed     // { [groupName]: Session[] }
fetchAll()                    // 从后端加载
save(sess)                    // 新建或更新
remove(id)                    // 删除
getById(id)                   // 按 ID 查找
```

**`stores/tabs.ts`**

```typescript
// 对外暴露
tabs: Tab[]                   // 标签页列表
activeConnId: string          // 当前激活的 connId
activeTab: computed           // 当前激活的 Tab 对象
openTab(connId, label, sessionId?)
closeTab(connId)              // 同时调用后端 Disconnect
setStatus(connId, status)     // 'connecting'|'connected'|'closed'
activateNext()                // Ctrl+Tab
activatePrev()                // Ctrl+Shift+Tab
activateByIndex(n)            // Ctrl+1~9
```

---

### 4.7 前端组件

| 组件 | 文件 | 功能 |
|------|------|------|
| `Terminal` | `Terminal.vue` | xterm.js 实例，ResizeObserver 尺寸同步，事件监听清理 |
| `ConnectDialog` | `ConnectDialog.vue` | 快速临时连接（不保存），支持密码/私钥/Agent |
| `SessionDialog` | `SessionDialog.vue` | 新建/编辑会话（名称/分组/标签/认证/高级选项），编辑模式自动回显 |
| `SessionSidebar` | `SessionSidebar.vue` | 分组树形列表，可折叠（220px↔48px），搜索过滤，右键菜单 |

---

## 5. 数据结构与接口

> 📖 后端暴露给前端的**全部方法、推送事件与数据结构**以 [`docs/API.md`](docs/API.md) 为准（随接口变更同步维护）。本节仅保留核心结构概览。

### Session（会话配置）

```go
// backend/store/sessions.go
type Session struct {
    ID           string    `json:"id"`            // UUID v4
    Name         string    `json:"name"`           // 显示名称
    Group        string    `json:"group"`          // 分组，空值归入"默认"组
    Host         string    `json:"host"`           // 主机名或 IP
    Port         int       `json:"port"`           // 默认 22
    Username     string    `json:"username"`
    AuthType     string    `json:"auth_type"`      // "password"|"key"|"agent"
    IdentityFile string    `json:"identity_file,omitempty"`
    Password     string    `json:"password,omitempty"`  // 明文（五期加密）
    ProxyJump    string    `json:"proxy_jump,omitempty"` // 四期启用
    Tags         []string  `json:"tags"`
    LastUsed     time.Time `json:"last_used,omitempty"`
}
```

### ConnectRequest（连接请求）

```go
// app.go
type ConnectRequest struct {
    SessionID    string `json:"session_id,omitempty"` // 引用已保存会话
    Host         string `json:"host,omitempty"`
    Port         int    `json:"port,omitempty"`
    Username     string `json:"username,omitempty"`
    AuthType     string `json:"auth_type,omitempty"`
    Password     string `json:"password,omitempty"`
    IdentityFile string `json:"identity_file,omitempty"`
}
```

> 指定 `session_id` 时加载保存的配置，同时传 `password` 可覆盖存储值（用于密码框输入）。

### FileInfo

```go
// backend/sftp/ops.go
type FileInfo struct {
    Name    string `json:"name"`
    Size    int64  `json:"size"`
    Mode    string `json:"mode"`     // 如 "-rwxr-xr-x"
    ModTime string `json:"mod_time"` // RFC3339
    IsDir   bool   `json:"is_dir"`
    Owner   string `json:"owner"`
    Group   string `json:"group"`
}
```

### TunnelConfig / TunnelStatus

```go
// backend/tunnel/manager.go
type TunnelConfig struct {
    Type       string `json:"type"`        // "local"|"remote"|"dynamic"
    LocalAddr  string `json:"local_addr"`  // 如 "127.0.0.1:18080"
    RemoteAddr string `json:"remote_addr"` // 如 "db.internal:5432"（dynamic 忽略）
}

type TunnelStatus struct {
    ID         string       `json:"id"`
    Config     TunnelConfig `json:"config"`
    ActiveConn int          `json:"active_conn"` // 当前活跃连接数
    BytesIn    int64        `json:"bytes_in"`
    BytesOut   int64        `json:"bytes_out"`
}
```

### sessions.json 存储格式

```json
{
  "version": 1,
  "sessions": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "生产 Web 服务器",
      "group": "Production",
      "host": "192.168.1.100",
      "port": 22,
      "username": "root",
      "auth_type": "key",
      "identity_file": "~/.ssh/id_ed25519",
      "proxy_jump": "",
      "tags": ["prod", "web"],
      "last_used": "2026-04-23T10:00:00Z"
    }
  ]
}
```

---

## 6. 前后端通信协议

### 6.1 方法调用（前端 → 后端）

```typescript
import { Connect, SendInput, ResizeTerminal, Disconnect,
         GetSessions, SaveSession, DeleteSession, GetGroups }
  from '../wailsjs/go/main/App'

// 连接
const connId = await Connect({ session_id: 'xxx' }, 120, 30)

// 发送输入
await SendInput(connId, 'ls -la\n')

// 调整尺寸
await ResizeTerminal(connId, cols, rows)

// 断开
await Disconnect(connId)
```

### 6.2 事件推送（后端 → 前端）

| 事件名 | 数据类型 | 触发时机 |
|--------|----------|----------|
| `terminal:output:{connID}` | `string`（Base64） | 远端有输出（stdout + stderr） |
| `terminal:closed:{connID}` | 无 | Shell 退出或连接断开 |
| `sftp:progress:{transferID}` | `{done, total int64}` | 文件传输进度（三期） |
| `editor:changed:{watchID}` | `{watch_id, remote_path, local_path}` | 本地临时文件被保存（800ms 去抖，v1.1） |
| `monitor:stats:{connID}` | `MonitorStats` | 服务器采样数据，每 2 秒推送（**v1.3**） |

### 6.3 终端数据流

```
用户键盘 → xterm.onData(str)
         → Wails.SendInput(connID, str)
         → session.stdin.Write([]byte)
         → SSH → 远端 Shell 处理
         → StdoutPipe.Read
         → base64.Encode(chunk)
         → EventsEmit("terminal:output:{connID}", b64)
         → EventsOn → atob(b64) → term.write(bytes)
```

### 6.4 PTY 尺寸同步

```
ResizeObserver(容器 DOM 变化)
  → fitAddon.fit() → 计算 cols/rows
  → Wails.ResizeTerminal(connID, cols, rows)
  → session.WindowChange(rows, cols)
```

---

## 7. 开发环境搭建

### 系统要求

| 工具 | 最低版本 | 安装方式 |
|------|----------|----------|
| Go | 1.22+ | https://go.dev/dl/ |
| Node.js | 18+ | https://nodejs.org/ |
| Wails CLI | v2.12.0 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| WebView2 | 最新 | Windows 11 已内置 |
| gcc（CGO） | 任意 | Windows 需 [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) 或 MSYS2 |

### 克隆与初始化

```bash
git clone https://github.com/zsh1020/Conch-SSH.git
cd Conch
go mod tidy
cd frontend && npm install && cd ..
```

---

## 8. 构建与运行

```bash
# 开发模式（热更新）
wails dev

# 生产构建
wails build
# → build/bin/conch.exe

# 仅构建前端
cd frontend && npm run build

# 重新生成 TS 绑定（修改 app.go 公开方法后必须执行）
wails generate module
```

**注意**：`frontend/wailsjs/` 目录在 `.gitignore` 中，不提交至仓库。每次 `wails build` 或 `wails dev` 时自动重新生成。手动修改 `app.go` 后可单独运行 `wails generate module`。

---

## 9. 分期进度总览

| 期次 | 功能范围 | 状态 | Commit |
|------|----------|------|--------|
| **第一期** | 项目骨架 + SSH 终端核心（PTY/认证/known_hosts） | ✅ 已完成 | `4036e5f` |
| **第二期** | 会话管理 UI + 多标签页 + 侧边栏 | ✅ 已完成 | `ea8a7d0` |
| **第三期** | SFTP 文件管理器（双栏/上传/下载/进度） | ✅ 已完成 | `ca30306` |
| **第四期** | 端口转发（本地/远程/SOCKS5）+ ProxyJump | ✅ 已完成 | `16e7ab3` |
| **第五期** | 设置/主题/密钥管理/ssh/config 导入 | ✅ 已完成 | `4b22796` |
| **v1.1** | 终端搜索/右键粘贴/侧边栏拖拽/文件预览/编辑器集成 | ✅ 已完成 | `157f16d` |
| **v1.2** | 布局重构：终端+文件面板同显，去除视图切换按钮 | ✅ 已完成 | `5d60ffc` |
| **v1.3** | Snippet 管理、广播、密码加密、主机告警、服务器监控 | ✅ 已完成 | `572b61e` |
| **v1.4** | 快速重连、最近收藏、Snippet 参数化、标签拖拽、监控折线图、快捷键面板、Tags 过滤、转发预设 | ✅ 已完成 | `d6e2fc0` |
| **v1.5** | 文件管理器体验（路径输入、图标、右键预览）、侧边栏精简 | ✅ 已完成 | `1f21cff` |

---

## 10. 已完成功能记录（历史存档）

各期详细任务记录已归档，请查看 [`docs/history/v1.0-v1.5-history.md`](./docs/history/v1.0-v1.5-history.md)。

| 期次 | 关键 Commit |
|------|------------|
| 第一期 | `4036e5f` — 项目骨架 + SSH 终端核心 |
| 第二期 | `ea8a7d0` — 会话管理 UI + 多标签页 |
| 第三期 | `ca30306` — SFTP 文件管理器 |
| 第四期 | `16e7ab3` — 端口转发 + ProxyJump |
| 第五期 | `4b22796` — 设置/主题/密钥/导入 |
| v1.1   | `157f16d` — 终端搜索/文件预览/编辑器集成 |
| v1.2   | `5d60ffc` — 布局重构（终端+文件同显）|
| v1.3   | `572b61e` — Snippet/广播/密码加密/监控 |
| v1.4   | `d6e2fc0` — 重连/收藏/参数化/折线图 |
| v1.5   | `1f21cff` — 文件管理体验/侧边栏精简 |

---

## 19. 后续版本规划

> 总体方向见 [`ROADMAP.md`](./ROADMAP.md)。本节聚焦每个版本的**具体技术任务**，作为开发清单使用。

### 版本时间表（一览）

| 版本 | 主题 | 预计交付 | 状态 |
|------|------|----------|------|
| v1.0.0 | 开源首发（项目由 gssh 更名，版本号重新起算） | 2026-10-31 | ✅ 已发布 |
| **v1.1** | **文件管理深化** | 2026-06 | 🚧 待启动 |
| v1.2 | 终端与会话深化 | 2026-08–09 | 📋 规划中 |
| v1.3 | 跨平台与发布工具链 | 2026-11 | 📋 规划中 |
| v2.0 | 插件架构 + 多设备同步 + AI 辅助 | 2027 Q1–Q2 | 📋 规划中 |

> gssh 时代（v1.0–v1.9.3）的版本记录见 [docs/history/CHANGELOG-归档-gssh时代.md](docs/history/CHANGELOG-归档-gssh时代.md)。

---

### gssh 时代已交付 — 分组排序 + 小圆点修复（归档）

**交付内容**：
- 侧边栏分组拖拽排序（`⠿` 手柄），顺序持久化到 `sessions.json`
- 后端 `store.GroupOrder` 字段 + `SaveGroupOrder` 函数
- `GetGroups()` 优先返回显式排序，回退到 session 出现顺序
- 非活跃 tab 小圆点修复：逐字节扫描过滤提示符重绘包

### v1.1 — 文件管理深化（详细任务）

**目标**：把 SFTP 文件管理器从"够用"变成"好用"，覆盖目录递归操作、批量编辑、权限修改。

#### 后端任务（Go）

| 文件 | 任务 |
|------|------|
| `backend/sftp/ops.go` | 新增 `UploadDir(localDir, remoteDir, progressFn)` 递归遍历 + 创建远端目录 + 文件并发上传 |
| `backend/sftp/ops.go` | 新增 `DownloadDir(remoteDir, localDir, progressFn)` 同上对称实现 |
| `backend/sftp/ops.go` | 新增 `Chmod(path, mode os.FileMode)`、`Chown(path, uid, gid int)` |
| `backend/sftp/ops.go` | 新增 `SearchByName(remoteDir, pattern string, recursive bool, maxResults int)` |
| `app.go` | 暴露 `SFTPUploadDir / SFTPDownloadDir / SFTPChmod / SFTPSearch` |
| `app.go` | 新增传输任务管理：`PauseTransfer / ResumeTransfer / CancelTransfer / ListTransfers` |
| `backend/sftp/transfer.go` ★ | 新文件：传输队列 manager，支持暂停 / 恢复（基于 SFTP `Read/Write` 偏移）/ 取消（context 取消）|

#### 前端任务（Vue）

| 文件 | 任务 |
|------|------|
| `frontend/src/components/FileManager.vue` | 表格行支持多选（Shift / Ctrl 点击）+ 选区高亮 |
| `frontend/src/components/FileManager.vue` | 拖拽接收同时支持文件 + 目录（DataTransferItem.webkitGetAsEntry）|
| `frontend/src/components/FileManager.vue` | 右键菜单新增「权限…」「在此搜索…」「批量删除…」 |
| `frontend/src/components/PermissionDialog.vue` ★ | 新建：rwx 矩阵 9 个 checkbox + 八进制输入双向绑定 |
| `frontend/src/components/TransferPanel.vue` ★ | 新建：独立的传输管理面板，支持暂停 / 恢复 / 重试 / 移除 |
| `frontend/src/components/CommandPalette.vue` ★ | 新建：Cmd+K 全局命令面板（fuzzy 搜索会话 / 片段 / 配色 / 文件）|
| `frontend/src/stores/transfers.ts` ★ | 新建：传输任务状态全局 store（与 backend transfer manager 镜像）|
| `frontend/src/components/SettingsDialog.vue` | 设置自动保存：去除 `保存设置` 按钮，所有字段 watch + 防抖 200ms |

#### 待评估技术问题

- [ ] SFTP 协议是否原生支持断点续传？需查 `pkg/sftp` `OpenFile(O_APPEND)` 行为
- [ ] 大目录递归列表的内存与性能（10k+ 文件）：是否需要分页 / 虚拟滚动
- [ ] 命令面板的 fuzzy 算法选型：fuse.js 还是手写 Subsequence 匹配

---

### v1.2 — 终端与会话深化（详细任务）

**目标**：终端面板专业化，匹配主流专业终端工具体验。

#### 后端任务

| 文件 | 任务 |
|------|------|
| `backend/recorder/asciinema.go` ★ | 新建：写入 [asciinema v2 cast](https://docs.asciinema.org/manual/asciicast/v2/) 文件，时间戳 + 数据流 |
| `app.go` | 新增 `StartRecording / StopRecording / ListRecordings / PlayRecording` |
| `backend/keymap/manager.go` ★ | 新建：自定义快捷键映射加载与持久化（`~/.config/conch/keymap.json`）|
| `backend/log/session.go` ★ | 新建：会话输出归档 to `~/.gssh/logs/{sessionId}/{date}.log` |

#### 前端任务

| 文件 | 任务 |
|------|------|
| `frontend/src/components/Terminal.vue` | 重构为支持父级 `TerminalPane.vue` 容器，单 Tab 内可嵌套多 Terminal |
| `frontend/src/components/TerminalPane.vue` ★ | 新建：分屏容器，水平 / 垂直分割，每个分区一个 Terminal 实例 |
| `frontend/src/components/AsciinemaPlayer.vue` ★ | 新建：内嵌 [asciinema-player](https://github.com/asciinema/asciinema-player) WebComponent |
| `frontend/src/components/ProxyJumpEditor.vue` ★ | 新建：多跳板拓扑可视化（节点 + 箭头）|
| `frontend/src/components/KeyManager.vue` ★ | 新建：`~/.ssh` 密钥列表管理界面 |
| `frontend/src/components/KeymapEditor.vue` ★ | 新建：快捷键自定义 UI（点击空白行录制按键组合）|
| `frontend/src/stores/workspace.ts` ★ | 新建：标签分组持久化（`workspaces.json`）|

#### 待评估技术问题

- [ ] 分屏的尺寸计算：Pane 调整时如何同步 PTY resize 给后端
- [ ] 录制文件的存储位置（用户目录 vs 配置目录），存储清理策略
- [ ] keymap 冲突检测：用户改的快捷键与已有 binding 撞了如何提示

---

### v1.3 — 跨平台与发布工具链（详细任务）

**目标**：从 Windows-only 走向三平台稳定发布。

#### 平台任务

| 任务 | 说明 |
|------|------|
| **macOS 构建** | `wails build -platform darwin/universal`，需 Xcode + Apple Developer ID |
| **macOS 公证** | `notarytool submit` + `staple` 流程脚本化 |
| **Linux .deb** | `nfpm` 配置，依赖 `webkit2gtk-4.0` |
| **Linux .rpm** | 同 `nfpm`，针对 Fedora / RHEL 系 |
| **Linux AppImage** | `appimagetool` 打包，单文件可执行 |
| **Windows MSI** | WiX Toolset 配置；开始菜单快捷方式 + `.gssh` 文件关联 |

#### 自动更新

| 文件 | 任务 |
|------|------|
| `backend/updater/checker.go` ★ | 新建：定时拉取 GitHub Releases API，比对版本号 |
| `backend/updater/installer.go` ★ | 新建：下载新版到临时目录，启动时替换可执行文件 |
| `frontend/src/components/UpdateNotice.vue` ★ | 新建：发现新版时的提示 banner，提供"立即更新 / 稍后" |

#### CI/CD

| 文件 | 任务 |
|------|------|
| `.github/workflows/build.yml` ★ | push 触发，矩阵构建 windows/macos/linux |
| `.github/workflows/release.yml` ★ | tag `v*` 触发，自动构建 + 创建 GitHub Release + 上传产物 |
| `tests/e2e/` ★ | Playwright 测试套：登录 / 连接 / SFTP / 设置流程 |
| `backend/**/*_test.go` ★ | Go 单元测试，覆盖核心 backend 包 |

#### 签名证书

- Windows EV Code Signing 证书（约 $300-500/年）
- Apple Developer ID（$99/年）

---

### v2.0 — 插件 + 同步 + AI（详细任务）

**目标**：架构升级。插件化 + 多端同步 + AI 助手。

#### 插件架构

| 模块 | 说明 |
|------|------|
| WebAssembly 运行时 | 选型 [`wazero`](https://github.com/tetratelabs/wazero)（纯 Go，无 CGO）|
| 插件 API 设计 | 终端 hook / 命令注入 / UI 扩展 / 文件操作 / 设置面板扩展 |
| 插件清单格式 | `plugin.toml`：name / version / permissions / hooks |
| 沙箱权限模型 | 默认无网络、无文件，明确请求权限项 |
| 内置插件市场 | 应用内浏览 / 安装，集中式 registry（GitHub repo 索引） |

#### 多设备同步

| 任务 | 说明 |
|------|------|
| 加密格式 | AES-256-GCM + 用户密码派生（Argon2id）|
| 同步后端选型 | WebDAV（Nextcloud）/ S3 兼容 / Git 三选一 |
| 冲突解决 | 字段级 last-write-wins，可选三方合并 UI |
| 增量同步 | 基于内容 hash，只传变更文件 |

#### AI 助手

| 任务 | 说明 |
|------|------|
| 模型抽象 | 统一接口，支持 OpenAI / Anthropic / Ollama / 自定义 endpoint |
| 命令补全 | 输入框旁边的灰色建议（类 Cursor）|
| 报错诊断 | 检测 `command not found` 等模式，触发解释 |
| 本地隐私 | 默认本地 Ollama，云模型必须用户主动开启 + 显式提示 |

---

*本节随每次发版同步更新；具体任务在版本启动会议上拆解为 GitHub Issues。*
