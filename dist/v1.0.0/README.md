# gssh v1.0.0 — Windows x64

## 安装

1. 下载 `gssh-v1.0.0-windows-amd64.zip`
2. 解压到任意目录
3. 双击 `gssh.exe` 运行

> **系统要求**：Windows 10 / Windows 11（WebView2 运行时已内置，无需额外安装）

## 功能列表

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
