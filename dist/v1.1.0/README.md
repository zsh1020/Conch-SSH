# gssh v1.1.0 — Windows x64

## 安装

1. 下载 `gssh-v1.1.0-windows-amd64.zip`
2. 解压到任意目录
3. 双击 `gssh-v1.1.0-windows-amd64.exe` 运行

> **系统要求**：Windows 10 / Windows 11（WebView2 运行时已内置，无需额外安装）

## v1.1 新增功能

| 功能 | 说明 |
|------|------|
| 终端内搜索 | `Ctrl+F` 打开搜索栏，`Enter` / `Shift+Enter` 上下导航，`Esc` 关闭 |
| 终端右键菜单 | 右键打开菜单：复制、粘贴、全选、清屏 |
| 侧边栏拖拽调宽 | 拖拽侧边栏右边缘调整宽度（160–480px），重启后保留 |
| 远端文件预览 | 文件管理器右侧预览面板，支持文本/图片/二进制文件 |
| 拖拽上传 | 从 Windows 资源管理器拖拽文件到远端栏直接上传 |
| 本地编辑器集成 | 右键远端文件 → 「用本地编辑器打开」，保存后提示上传回服务器 |

## 完整功能列表（v1.0 基础功能）

| 功能 | 说明 |
|------|------|
| SSH 终端 | PTY + xterm.js，支持全彩、Unicode，多标签并发 |
| 会话管理 | 分组 / 搜索 / 右键菜单 / 双击连接 |
| SFTP 文件管理 | 双栏浏览，上传/下载带实时进度条 |
| 端口转发 | 本地 -L / 远程 -R / SOCKS5 动态代理 -D |
| ProxyJump | 跳板机链式连接（user@host:port 格式） |
| 设置面板 | 深色/浅色主题，字体字号，光标样式 |
| SSH 密钥生成 | Ed25519 / RSA-4096，公钥一键复制 |
| 配置导入 | 解析 `~/.ssh/config` 批量导入会话 |

## 认证方式

- 密码认证
- 私钥认证（支持加密私钥）
- SSH Agent

## 配置文件位置

- 会话列表：`%APPDATA%\gssh\sessions.json`（`~/.config/gssh/sessions.json`）
- 设置：`%APPDATA%\gssh\settings.json`
- 侧边栏宽度：浏览器 localStorage（应用内存储）
