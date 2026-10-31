# Conch — 项目约定与已知陷阱

> 本文件由 `CLAUDE.md` 导入而来，供所有 AI 编码助手使用。**`CLAUDE.md` 是原始来源**，修改本项目约定时请同步更新两个文件。

> 项目对外品牌为 **Conch**（🐚）。数据目录已迁至 `~/.config/conch/` 与 `%AppData%\conch\`（首启从旧 `gssh` 目录自动迁移，见 `config.MigrateLegacyDir`）。
> 但以下内部标识**仍沿用 `gssh`，请勿改动**：密码加密种子 `keystore.machineKey`（改了密码全部解不开）、各 `localStorage` key（如 `gssh-fm-col-widths`，改了会重置用户状态）、`gssh:` 事件名前缀。

本文件记录项目约定、已知陷阱和常用操作，帮助 AI 助手快速上手。

---

## 项目快速导航

| 文件 | 作用 |
|------|------|
| `app.go` | App 结构体、NewApp、startup/beforeClose、getConn/removeConn |
| `app_session.go` | ConnectRequest + Connect/SendInput/Resize/Disconnect |
| `app_sftp.go` | getSFTP（懒加载）+ 全部 SFTPxxx 方法 + makeProgressFn |
| `app_editor.go` | OpenWithEditor/UploadEditedFile/StopEditorWatch + 目录对话框 |
| `app_monitor.go` | StartMonitor/StopMonitor |
| `app_keygen.go` | GenerateKey/marshalED25519Private/uniquePath |
| `app_misc.go` | Snippet/Session/Settings/Import/LocalFS/Tunnel 方法 |
| `main.go` | Wails 入口，窗口配置，`OnBeforeClose` 保存几何 |
| `backend/config/settings.go` | 全局设置结构体 + 持久化（JSON） |
| `backend/store/sessions.go` | 会话 CRUD + 密码 AES-GCM 加密 + 分组排序持久化 |
| `frontend/src/App.vue` | 根组件（~280行，布局+连接逻辑） |
| `frontend/src/components/TabBar.vue` | 标签页栏 + 工具按钮组 |
| `frontend/src/components/SessionSidebar.vue` | 左侧会话树 + 分组拖拽排序 + 右键菜单 |
| `frontend/src/components/BroadcastBar.vue` | 批量广播条 |
| `frontend/src/components/StatusBar.vue` | 底部快捷命令状态栏（含参数弹窗） |
| `frontend/src/components/HelpPanel.vue` | 快捷键速查面板 |
| `frontend/src/components/FileManager.vue` | SFTP 文件管理器 |
| `frontend/src/components/FilePreviewModal.vue` | 文件预览弹窗 |
| `frontend/src/components/Terminal.vue` | xterm.js 终端 |
| `frontend/src/stores/settings.ts` | 设置 Pinia store + `applyColorScheme` |
| `frontend/src/stores/dialogs.ts` | Promise-based 全局对话框 |
| `frontend/src/style.css` | CSS 变量设计令牌（深/浅双主题） |

---

## 常见操作

### 修改 Go 绑定后重新生成 TS 类型

```bash
wails generate module
```

`frontend/wailsjs/` 不提交到 git，由 wails 自动生成。

### 开发模式（热更新）

```bash
wails dev
```

### 生产构建

```bash
wails build
# → build/bin/conch.exe
```

### TypeScript 类型检查

```bash
cd frontend && npx vue-tsc --noEmit
```

**每次修改 .vue 后必须跑一遍确认无类型错误。**

---

## 已知陷阱

### 1. Wails 可拖拽区域阻止点击事件

标题栏有 `--wails-draggable: drag`，其中的按钮必须设置 `--wails-draggable: no-drag`：

```html
<div class="title-actions" style="--wails-draggable: no-drag">
```

### 2. Scoped CSS 不跟随 Teleport

`<Teleport to="body">` 内的元素渲染到 body，父组件的 `<style scoped>` 不生效。
解决方案：在同一个 .vue 文件里增加一个无 `scoped` 的 `<style>` 块，或者把 Teleported 组件抽成独立组件（推荐，如 `FilePreviewModal.vue`）。

### 3. CSS 特异性陷阱（文件列表）

`.fm-table td`（特异性 0,0,1,1）会覆盖 `.fm-col-name`（0,0,1,0）。
修复方式：使用元素+类选择器：`td.col-icon`、`td.fm-col-name`。

### 4. Go Settings 结构体字段必须完整

前端发送的设置字段若在 Go `Settings` struct 中不存在，会被**静默丢弃**。
添加新设置项时：
- Go: `backend/config/settings.go` 的 `Settings` struct
- 前端: `frontend/src/stores/settings.ts` 的 `Settings` interface
两边必须同步。

### 5. xterm.js 滚动视口硬编码黑色背景

`node_modules/@xterm/xterm/css/xterm.css` 内有：
```css
.xterm .xterm-viewport { background-color: #000; }
```
已在 `frontend/src/style.css` 末尾用 `!important` 覆盖为 `transparent`，配合父容器动态背景色。

---

## 对话框 API（全局）

```typescript
import { useDialogStore } from '../stores/dialogs'
const dlg = useDialogStore()

await dlg.info('提示内容')           // 确定按钮
await dlg.error('错误内容', '标题')  // 带标题
const ok = await dlg.confirm('确认？') // 返回 boolean
const val = await dlg.prompt('请输入', '标题', '占位符') // 返回 string | null
```

禁止使用原生 `alert / confirm / prompt`。

---

## 配色方案 / 主题

`settingsStore.applyColorScheme(schemeName)` 设置终端配色。
`settingsStore.applyTheme(theme)` 切换深/浅 UI 主题（修改 `data-theme` attribute）。
配色方案同时更新 CSS 变量 `--bg-primary`，让整个窗口背景与终端一致。

---

## 数据文件位置

| 文件 | 路径 |
|------|------|
| 会话配置 | `~/.config/conch/sessions.json` |
| 应用设置 | `~/.config/conch/settings.json` |
| 命令片段 | `~/.config/conch/snippets.json` |
| 主机指纹 | `~/.ssh/known_hosts` |

---

## 文档索引

| 文档 | 内容 |
|------|------|
| `README.md` | 功能特性 + 快速开始 |
| `docs/API.md` | 后端接口文档（全部 Wails 方法 + 推送事件 + 数据结构）|
| `DEVELOPMENT.md` | 架构 + 核心模块 + 后续规划 |
| `ROADMAP.md` | v1.1–v2.0+ 版本路线图 |
| `CHANGELOG.md` | 版本变更记录 |
| `docs/history/v1.0-v1.5-history.md` | 第一期～v1.5 完整开发记录 |
