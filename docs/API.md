# Conch 后端接口文档

本文件记录 Go 后端通过 Wails 暴露给前端的全部方法、推送事件和数据结构。

> **来源说明**：所有方法定义在 `app*.go`（`App` 结构体的导出方法），Wails 在
> `wails build` / `wails generate module` 时自动生成 TS 绑定到
> `frontend/wailsjs/go/main/App.{d.ts,js}`（该目录不入 git）。
> 本文件需在**新增/修改导出方法后手动同步**。

## 调用约定

```ts
import { Connect, SFTPList } from '../../wailsjs/go/main/App'

const connID = await Connect(req, 120, 30)
const files  = await SFTPList(connID, '/home')
```

- 所有方法返回 `Promise`；Go 端返回 `error` 的方法在前端表现为 Promise reject。
- `connID` 由 `Connect` 返回，是后续终端 / SFTP / 隧道 / 监控调用的句柄。
- 长耗时操作（SFTP 传输、终端输出、监控）通过 **Wails 事件**异步推送，见文末「推送事件」。

---

## 1. 连接与终端会话 `app_session.go`

| 方法 | 签名 | 说明 |
|------|------|------|
| `Connect` | `(req: ConnectRequest, cols: number, rows: number) → Promise<string>` | 建立 SSH 连接并启动交互式 shell，返回 `connID`。未知主机指纹时触发 `hostkey:confirm` 事件等待前端确认。 |
| `Disconnect` | `(connID: string) → Promise<void>` | 关闭连接，停止全部隧道、SFTP、shell。 |
| `SendInput` | `(connID: string, data: string) → Promise<void>` | 向远端 shell 写入文本（键盘输入）。 |
| `SendInputBase64` | `(connID: string, data: string) → Promise<void>` | 写入 base64 编码的二进制输入。 |
| `ResizeTerminal` | `(connID: string, cols: number, rows: number) → Promise<void>` | 调整远端 PTY 窗口大小。 |
| `TerminalReady` | `(connID: string) → Promise<void>` | 前端注册好事件监听后调用，触发 flush 连接初期缓冲的输出。 |
| `RespondHostKey` | `(requestID: string, accepted: boolean) → Promise<void>` | 回应 `hostkey:confirm` / `hostkey:changed` 弹窗的确认结果。 |

## 2. 本地终端（底部 dock）`app_localterm.go`

全局多会话，每个 `id` 对应一个本地 ConPTY/pty 进程。

| 方法 | 签名 | 说明 |
|------|------|------|
| `StartLocalTerm` | `(id: string, shell: string, cols: number, rows: number) → Promise<void>` | 启动（或重启同 id 的）本地 shell。`shell`：`"powershell"`（优先 pwsh7）/ `"cmd"` / `"custom"`（读设置 `local_shell`）/ `""`（平台默认）。输出经 `localterm:output:{id}` 推送。 |
| `SendLocalInput` | `(id: string, data: string) → Promise<void>` | 向本地 shell 写入输入。 |
| `ResizeLocalTerm` | `(id: string, cols: number, rows: number) → Promise<void>` | 调整本地 PTY 大小。 |
| `StopLocalTerm` | `(id: string) → Promise<void>` | 关闭指定本地终端。 |
| `StopAllLocalTerms` | `() → Promise<void>` | 关闭全部本地终端（app 退出时自动调用）。 |

## 3. SFTP 文件操作 `app_sftp.go`

SFTP 子系统对每个连接懒加载。传输类方法立即返回 `transferID`，进度经事件推送。

| 方法 | 签名 | 说明 |
|------|------|------|
| `SFTPList` | `(connID, path: string) → Promise<FileInfo[]>` | 列目录。 |
| `SFTPHomePath` | `(connID: string) → Promise<string>` | 远端当前工作目录（家目录）。 |
| `SFTPMkdir` | `(connID, path: string) → Promise<void>` | 新建目录。 |
| `SFTPCreateFile` | `(connID, path: string) → Promise<void>` | 新建空文件。 |
| `SFTPDelete` | `(connID, path: string) → Promise<void>` | 删除文件/目录。 |
| `SFTPRename` | `(connID, oldPath, newPath: string) → Promise<void>` | 重命名/移动。 |
| `SFTPChmod` | `(connID, path: string, mode: number) → Promise<void>` | 修改权限，`mode` 为八进制数值（如 `0o755`）。 |
| `SFTPStat` | `(connID, path: string) → Promise<{mode, name}>` | 单个文件信息（含权限串）。 |
| `SFTPUpload` | `(connID, localPath, remotePath: string) → Promise<string>` | 上传文件，返回 `transferID`。 |
| `SFTPDownload` | `(connID, remotePath, localPath: string) → Promise<string>` | 下载文件，返回 `transferID`。 |
| `SFTPUploadDir` | `(connID, localPath, remotePath: string) → Promise<string>` | 递归上传目录。 |
| `SFTPDownloadDir` | `(connID, remotePath, localPath: string) → Promise<string>` | 递归下载目录。 |
| `SFTPCancelTransfer` | `(transferID: string) → Promise<void>` | 取消进行中的传输。 |
| `SFTPReadFile` | `(connID, remotePath: string, maxBytes: number) → Promise<string>` | 读取文件前 `maxBytes` 字节（≤0 默认 512KB），返回 base64。用于预览。 |

## 4. 会话与分组 `app_misc.go`

| 方法 | 签名 | 说明 |
|------|------|------|
| `GetSessions` | `() → Promise<Session[]>` | 全部已保存会话。 |
| `SaveSession` | `(sess: Session) → Promise<void>` | 新建或更新会话（密码 AES-GCM 加密存储）。 |
| `DeleteSession` | `(id: string) → Promise<void>` | 删除会话。 |
| `GetGroups` | `() → Promise<string[]>` | 全部分组名。 |
| `SaveGroupOrder` | `(order: string[]) → Promise<void>` | 持久化分组排序。 |
| `RenameGroup` | `(oldName, newName: string) → Promise<void>` | 重命名分组。 |
| `DeleteGroup` | `(name: string) → Promise<void>` | 删除分组。 |
| `ReorderSessions` | `(ids: string[]) → Promise<void>` | 持久化会话排序。 |
| `ImportSSHConfig` | `() → Promise<number>` | 从 `~/.ssh/config` 导入，返回导入数量。 |

## 5. 命令片段 `app_misc.go`

| 方法 | 签名 | 说明 |
|------|------|------|
| `GetSnippets` | `() → Promise<Snippet[]>` | 全部命令片段。 |
| `SaveSnippet` | `(sn: Snippet) → Promise<void>` | 新建或更新片段。 |
| `DeleteSnippet` | `(id: string) → Promise<void>` | 删除片段。 |

## 6. 设置 `app_misc.go`

| 方法 | 签名 | 说明 |
|------|------|------|
| `GetSettings` | `() → Promise<Settings>` | 读取全局设置。 |
| `SaveSettings` | `(s: Settings) → Promise<void>` | 保存全局设置（JSON 持久化）。 |

## 7. 端口转发 `app_misc.go`

| 方法 | 签名 | 说明 |
|------|------|------|
| `StartTunnel` | `(connID: string, cfg: TunnelConfig) → Promise<string>` | 启动隧道，返回 `tunnelID`。 |
| `StopTunnel` | `(connID, tunnelID: string) → Promise<void>` | 停止隧道。 |
| `ListTunnels` | `(connID: string) → Promise<TunnelStatus[]>` | 当前运行中的隧道及流量统计。 |
| `GetSessionSavedTunnels` | `(connID: string) → Promise<TunnelRule[]>` | 该连接关联会话保存的隧道规则。 |
| `UpdateSavedTunnels` | `(connID: string, rules: TunnelRule[]) → Promise<void>` | 用显式列表替换保存的隧道规则。 |
| `SaveSessionTunnels` | `(connID: string) → Promise<void>` | 把当前运行的隧道持久化到关联会话。 |

## 8. 服务器监控 `app_monitor.go`

| 方法 | 签名 | 说明 |
|------|------|------|
| `StartMonitor` | `(connID: string) → Promise<void>` | 每 1 秒采样一次，经 `monitor:stats:{connID}` 推送 CPU/内存/网络/磁盘/温度。 |
| `StopMonitor` | `(connID: string) → Promise<void>` | 停止采样。 |

## 9. 本地文件系统 `app_misc.go`

| 方法 | 签名 | 说明 |
|------|------|------|
| `LocalList` | `(path: string) → Promise<LocalFileInfo[]>` | 列本地目录。 |
| `LocalHomePath` | `() → Promise<string>` | 本地家目录。 |
| `LocalListRoots` | `() → Promise<string[]>` | 根路径（Windows 为盘符列表，其余为 `["/"]`）。 |
| `LocalIsDir` | `(path: string) → Promise<boolean>` | 是否为目录。 |
| `LocalQuickDirs` | `() → Promise<QuickDir[]>` | 标准用户目录（桌面/文档/下载/…，仅返回存在的）。 |

## 10. 编辑器集成 `app_editor.go`

| 方法 | 签名 | 说明 |
|------|------|------|
| `OpenWithEditor` | `(connID, remotePath: string) → Promise<string>` | 下载到临时目录、用系统默认程序打开并监听改动，返回 `watchID`。保存时触发 `editor:changed:{watchID}`。 |
| `UploadEditedFile` | `(connID, watchID: string) → Promise<void>` | 把编辑后的临时文件回传远端。 |
| `UploadEditedFileSudo` | `(connID, watchID, sudoPassword: string) → Promise<void>` | 目标文件无写权限时的 sudo 提权保存：临时文件经 SFTP 传到 `/tmp` 后用 `sudo cp` 覆盖目标。`sudoPassword` 传空串可复用缓存的密码（首次成功后在连接存活期内有效）；错误前缀 `sudo password required` 表示需要密码，`sudo auth failed` 表示认证失败。 |
| `StopEditorWatch` | `(watchID: string) → Promise<void>` | 停止监听并清理临时副本。 |

## 11. 系统对话框 `app_editor.go` / `app_dialog*.go`

| 方法 | 签名 | 说明 |
|------|------|------|
| `OpenDirectoryDialog` | `() → Promise<string>` | 选择目录。 |
| `OpenFilesDialog` | `() → Promise<string[]>` | 多选文件。 |
| `OpenUploadTargetsDialog` | `() → Promise<string[]>` | 选择上传目标（Windows 下支持文件夹，见 `app_dialog_windows.go`）。 |
| `GetDownloadsDir` | `() → Promise<string>` | 用户下载目录。 |
| `OpenLocalPath` | `(path: string) → Promise<void>` | 用系统默认程序打开本地文件/目录（下载完成后点击打开）。 |
| `RevealLocalPath` | `(path: string) → Promise<void>` | 在系统文件管理器中定位该文件（Win/Mac 选中，Linux 打开所在目录）。 |

## 12. 密钥与杂项

| 方法 | 签名 | 文件 | 说明 |
|------|------|------|------|
| `GenerateKey` | `(keyType, comment: string) → Promise<GenerateKeyResult>` | `app_keygen.go` | 生成密钥对写入 `~/.ssh/`。`keyType`：`"ed25519"` 或 `"rsa4096"`。 |
| `ZmodemSave` | `(suggestedName, b64data: string) → Promise<string>` | `app_zmodem.go` | 弹保存框，把 base64 内容写入用户选择的路径（sz 接收）。 |
| `GetVersion` | `() → Promise<string>` | `version.go` | 应用版本号。 |

---

## 推送事件（后端 → 前端）

通过 `EventsOn(name, handler)` 订阅，`EventsOff(name)` 取消。`{...}` 为运行时拼接的 id。

| 事件名 | 载荷 | 触发时机 |
|--------|------|----------|
| `terminal:output:{connID}` | `string` | 远端 shell 有输出 |
| `terminal:closed:{connID}` | — | 远端 shell 关闭/断线 |
| `localterm:output:{id}` | `string`（base64） | 本地 shell 有输出 |
| `localterm:exit:{id}` | — | 本地 shell 退出 |
| `monitor:stats:{connID}` | `MonitorStats` | 监控采样（每 1s） |
| `sftp:progress:{transferID}` | `{done, total}` | 传输进度（节流 200ms） |
| `sftp:done:{transferID}` | — | 传输完成 |
| `sftp:error:{transferID}` | `string` | 传输出错 |
| `sftp:cancelled:{transferID}` | — | 传输被取消 |
| `editor:changed:{watchID}` | `{watch_id, remote_path, local_path}` | 外部编辑器保存了文件 |
| `hostkey:confirm` | `{id, host, fingerprint}` | 未知主机指纹，等待 `RespondHostKey` |
| `hostkey:changed` | `{id, host, old_fp, new_fp}` | 主机指纹变更，等待 `RespondHostKey` |

---

## 数据结构

```go
// ConnectRequest — 连接请求（app_session.go）
// 指定 session_id 时加载已保存配置，同传 password 可覆盖存储值
type ConnectRequest struct {
    SessionID    string `json:"session_id,omitempty"`
    Host         string `json:"host,omitempty"`
    Port         int    `json:"port,omitempty"`
    Username     string `json:"username,omitempty"`
    AuthType     string `json:"auth_type,omitempty"`   // "password"|"key"|"agent"
    Password     string `json:"password,omitempty"`
    IdentityFile string `json:"identity_file,omitempty"`
}

// Session — 会话配置（backend/store/sessions.go）
type Session struct {
    ID           string       `json:"id"`            // UUID v4
    Name         string       `json:"name"`
    Group        string       `json:"group"`         // 空值归入"默认"组
    Host         string       `json:"host"`
    Port         int          `json:"port"`
    Username     string       `json:"username"`
    AuthType     string       `json:"auth_type"`     // "password"|"key"|"agent"
    IdentityFile string       `json:"identity_file,omitempty"`
    Password     string       `json:"password,omitempty"`   // AES-GCM 加密存储
    ProxyJump    string       `json:"proxy_jump,omitempty"`
    Tags         []string     `json:"tags"`
    LastUsed     time.Time    `json:"last_used,omitempty"`
    Starred      bool         `json:"starred,omitempty"`
    Tunnels      []TunnelRule `json:"tunnels,omitempty"` // 随会话保存的隧道规则
}

// Snippet — 命令片段（backend/store/snippets.go）
type Snippet struct {
    ID      string `json:"id"`
    Name    string `json:"name"`
    Command string `json:"command"`
    Group   string `json:"group"`
}

// FileInfo — 远端文件项（backend/sftp/ops.go）
type FileInfo struct {
    Name    string `json:"name"`
    Size    int64  `json:"size"`
    Mode    string `json:"mode"`     // 如 "-rwxr-xr-x"
    ModTime string `json:"mod_time"` // RFC3339
    IsDir   bool   `json:"is_dir"`
    Owner   string `json:"owner"`
    Group   string `json:"group"`
}

// LocalFileInfo — 本地文件项（app_misc.go）
type LocalFileInfo struct {
    Name    string    `json:"name"`
    Size    int64     `json:"size"`
    IsDir   bool      `json:"is_dir"`
    Path    string    `json:"path"`
    ModTime time.Time `json:"mod_time"`
}

// QuickDir — 本地快捷目录（app_misc.go）
type QuickDir struct {
    Name string `json:"name"`
    Path string `json:"path"`
}

// TunnelConfig / TunnelStatus / TunnelRule（backend/tunnel/manager.go, backend/store/sessions.go）
type TunnelConfig struct {
    Type       string `json:"type"`        // "local"|"remote"|"dynamic"
    LocalAddr  string `json:"local_addr"`  // 如 "127.0.0.1:18080"
    RemoteAddr string `json:"remote_addr"` // 如 "db.internal:5432"（dynamic 忽略）
}
type TunnelStatus struct {
    ID         string       `json:"id"`
    Config     TunnelConfig `json:"config"`
    ActiveConn int          `json:"active_conn"`
    BytesIn    int64        `json:"bytes_in"`
    BytesOut   int64        `json:"bytes_out"`
}
type TunnelRule struct {
    Type       string `json:"type"`
    LocalAddr  string `json:"local_addr"`
    RemoteAddr string `json:"remote_addr"`
}

// GenerateKeyResult（app_keygen.go）
type GenerateKeyResult struct {
    PrivateKeyPath string `json:"private_key_path"`
    PublicKey      string `json:"public_key"`
}
```

> `Settings` 结构体字段较多，见 `backend/config/settings.go` 与
> `frontend/src/stores/settings.ts`（两边必须同步，否则前端字段会被静默丢弃）。
> 含：终端外观、`local_shell`（本地终端自定义 shell）、文件列表列、下载目录、
> 高亮规则、窗口几何等。
