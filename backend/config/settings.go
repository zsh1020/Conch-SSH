package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// HighlightRule defines a regex pattern and the colors to apply to matches.
type HighlightRule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Color   string `json:"color"`              // hex foreground, e.g. "#ff5555"
	BgColor string `json:"bg_color,omitempty"` // hex background (optional)
	Enabled bool   `json:"enabled"`
}

// Settings holds all user-configurable preferences.
type Settings struct {
	// Terminal appearance
	Theme       string `json:"theme"`        // "dark" | "light"
	ColorScheme string `json:"color_scheme"` // e.g. "github-dark", "dracula", …
	FontFamily  string `json:"font_family"`
	FontSize    int    `json:"font_size"`
	FontWeight  int    `json:"font_weight"` // 100–900, 400 = normal, 700 = bold
	CursorStyle string `json:"cursor_style"` // "block" | "bar" | "underline"
	CursorBlink bool   `json:"cursor_blink"`
	Scrollback  int    `json:"scrollback"`
	BellStyle   string `json:"bell_style"` // "none" | "sound" | "visual"
	// Local terminal
	LocalShell string `json:"local_shell"` // path/name of local shell; empty = auto-detect
	// Sidebar
	ShowRecent bool `json:"show_recent"` // show "最近" group in session list
	// File manager
	FMColumns   []string `json:"fm_columns"`    // ordered visible columns
	DownloadDir string   `json:"download_dir"`  // default local download directory
	// Output highlight rules
	HighlightRules []HighlightRule `json:"highlight_rules,omitempty"`
	// Window geometry (saved on close, restored on next launch)
	WindowWidth     int  `json:"window_width"`
	WindowHeight    int  `json:"window_height"`
	WindowMaximized bool `json:"window_maximized"`
}

func defaults() Settings {
	return Settings{
		Theme:       "dark",
		ColorScheme: "conch",
		FontFamily:  "Cascadia Code, JetBrains Mono, Fira Code, Consolas, monospace",
		FontSize:    14,
		FontWeight:  400,
		CursorStyle: "block",
		CursorBlink: true,
		Scrollback:  5000,
		BellStyle:   "none",
		LocalShell:  "",
		ShowRecent:  true,
		FMColumns:   []string{"name", "mode", "owner", "group", "size", "mod_time"},
		HighlightRules: defaultHighlightRules(),
		WindowWidth:     1200,
		WindowHeight:    800,
		WindowMaximized: false,
	}
}

// defaultHighlightRules returns a WindTerm-inspired highlight ruleset.
//
// Color palette (WindTerm token scopes + VS Code semantic tokens):
//   token.error-token   → #f44747  红
//   token.warn-token    → #cd9731  琥珀
//   token.success-token → #6a9955  绿
//   token.info-token    → #6796e6  蓝
//   token.debug-token   → #b267e6  紫
//   constant.numeric    → #b5cea8  淡绿（数字）
//   variable            → #9cdcfe  淡蓝（变量/路径）
//   entity.name.function→ #dcdcaa  黄（HTTP 方法）
//   type                → #4ec9b0  青（IP 地址）
//   comment             → #6a737d  灰（时间戳）
func defaultHighlightRules() []HighlightRule {
	return []HighlightRule{
		// ── 1. 严重错误（token.error-token 最高优先级） ──────────────────────────
		{
			ID: "hl-fatal", Name: "致命错误",
			Pattern: `\b(FATAL|CRITICAL|CRIT|EMERG|ALERT|PANIC|SIGKILL|SIGSEGV|SIGABRT)\b`,
			Color: "#ff0000", BgColor: "#3a0000", Enabled: true,
		},
		// ── 2. 错误（token.error-token）────────────────────────────────────────
		{
			ID: "hl-error", Name: "错误",
			Pattern: `\b(error|Error|ERROR|err|ERR|failed|Failed|FAILED|failure|Failure|FAILURE|exception|Exception|EXCEPTION|traceback|Traceback|panic|Abort|abort)\b`,
			Color: "#f44747", Enabled: true,
		},
		// ── 3. 警告（token.warn-token）─────────────────────────────────────────
		{
			ID: "hl-warn", Name: "警告",
			Pattern: `\b(warning|Warning|WARNING|warn|Warn|WARN|deprecated|Deprecated|DEPRECATED|caution|Caution|CAUTION)\b`,
			Color: "#cd9731", Enabled: true,
		},
		// ── 4. 成功（token.success-token）──────────────────────────────────────
		{
			ID: "hl-success", Name: "成功",
			Pattern: `\b(success|Success|SUCCESS|ok|OK|done|Done|DONE|passed|Passed|PASSED|pass|PASS|complete|Complete|COMPLETE|completed|Completed|COMPLETED|ready|Ready|READY|started|Started)\b`,
			Color: "#6a9955", Enabled: true,
		},
		// ── 5. 信息（token.info-token）─────────────────────────────────────────
		{
			ID: "hl-info", Name: "信息",
			Pattern: `\b(info|Info|INFO|notice|Notice|NOTICE|note|Note|NOTE|hint|Hint|HINT)\b`,
			Color: "#6796e6", Enabled: true,
		},
		// ── 6. 调试（token.debug-token）────────────────────────────────────────
		{
			ID: "hl-debug", Name: "调试",
			Pattern: `\b(debug|Debug|DEBUG|trace|Trace|TRACE|verbose|Verbose|VERBOSE)\b`,
			Color: "#b267e6", Enabled: true,
		},
		// ── 7. 括号日志级别 [LEVEL]（常见于 Java/Go/Rust 日志框架）──────────────
		{
			ID: "hl-bracket-error", Name: "[ERROR/FATAL]",
			Pattern: `\[(ERROR|FATAL|CRIT|CRITICAL|PANIC|FAIL|FAILED)\]`,
			Color: "#f44747", BgColor: "#2d1515", Enabled: true,
		},
		{
			ID: "hl-bracket-warn", Name: "[WARN]",
			Pattern: `\[(WARN|WARNING|CAUTION)\]`,
			Color: "#cd9731", BgColor: "#2a2010", Enabled: true,
		},
		{
			ID: "hl-bracket-success", Name: "[OK/PASS]",
			Pattern: `\[(OK|PASS|PASSED|SUCCESS|DONE)\]`,
			Color: "#6a9955", BgColor: "#0f1f0f", Enabled: true,
		},
		{
			ID: "hl-bracket-info", Name: "[INFO]",
			Pattern: `\[(INFO|NOTICE)\]`,
			Color: "#6796e6", Enabled: true,
		},
		{
			ID: "hl-bracket-debug", Name: "[DEBUG/TRACE]",
			Pattern: `\[(DEBUG|TRACE|VERBOSE)\]`,
			Color: "#b267e6", Enabled: true,
		},
		// ── 8. HTTP 状态码（4xx/5xx 红，2xx/3xx 绿）────────────────────────────
		{
			ID: "hl-http-error", Name: "HTTP 错误状态码",
			Pattern: `\b[45]\d{2}\b`,
			Color: "#f44747", Enabled: true,
		},
		{
			ID: "hl-http-ok", Name: "HTTP 成功状态码",
			Pattern: `\b[23]\d{2}\b`,
			Color: "#6a9955", Enabled: true,
		},
		// ── 9. HTTP 方法（entity.name.function）────────────────────────────────
		{
			ID: "hl-http-method", Name: "HTTP 方法",
			Pattern: `\b(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|CONNECT|TRACE)\b`,
			Color: "#dcdcaa", Enabled: true,
		},
		// ── 10. IP 地址（type → 青色）───────────────────────────────────────────
		{
			ID: "hl-ip", Name: "IP 地址",
			Pattern: `\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b`,
			Color: "#4ec9b0", Enabled: true,
		},
		// ── 11. 时间戳（comment → 灰色）────────────────────────────────────────
		{
			ID: "hl-timestamp", Name: "时间戳",
			Pattern: `\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?`,
			Color: "#6a737d", Enabled: true,
		},
		// ── 12. 文件路径（variable → 淡蓝）─────────────────────────────────────
		{
			ID: "hl-path", Name: "文件路径",
			Pattern: `(?:^|[\s("])(/[\w./~@:%-]+)`,
			Color: "#9cdcfe", Enabled: true,
		},
		// ── 13. Shell 变量（variable → 淡蓝）───────────────────────────────────
		{
			ID: "hl-var", Name: "Shell 变量",
			Pattern: `\$(?:\{[\w]+\}|[A-Za-z_][\w]*)`,
			Color: "#9cdcfe", Enabled: true,
		},
		// ── 14. 数字（constant.numeric → 淡绿）─────────────────────────────────
		{
			ID: "hl-number", Name: "数字",
			Pattern: `\b\d+(?:\.\d+)?(?:[KkMmGgTt][Bb]?)?\b`,
			Color: "#b5cea8", Enabled: true,
		},
		// ── 15. Shell 命令（entity.name.function → 黄色）─────────────────────────
		{
			ID: "hl-cmd", Name: "Shell 命令",
			Pattern: `\b(ls|ll|la|cat|tac|more|less|head|tail|grep|egrep|fgrep|zgrep|find|locate|which|whereis|type|file|stat|wc|sort|uniq|cut|tr|awk|gawk|sed|diff|patch|comm|tee|xargs|column|paste|join|od|xxd|hexdump|strings|echo|printf|read|yes|seq|shuf|bc|dc|expr|test|sleep|date|cal|uptime|uname|hostname|id|whoami|w|who|last|pwd|cd|pushd|popd|dirs|mkdir|rmdir|rm|cp|mv|ln|touch|chmod|chown|chgrp|chattr|lsattr|install|rsync|scp|sftp|ssh|ssh-keygen|ssh-copy-id|curl|wget|aria2c|ping|traceroute|dig|nslookup|host|netstat|ss|ip|ifconfig|route|nc|netcat|nmap|iptables|ufw|ps|top|htop|kill|pkill|killall|nohup|jobs|bg|fg|disown|watch|screen|tmux|lsof|fuser|strace|ltrace|lscpu|lspci|lsblk|lsusb|dmesg|df|du|free|mount|umount|fdisk|parted|mkfs|fsck|blkid|tar|zip|unzip|gzip|gunzip|bzip2|bunzip2|xz|7z|base64|md5sum|sha256sum|sha1sum|openssl|gpg|git|svn|make|cmake|gcc|clang|python|python3|pip|pip3|node|npm|yarn|go|cargo|ruby|gem|java|mvn|gradle|docker|kubectl|helm|systemctl|service|journalctl|crontab|apt|apt-get|yum|dnf|rpm|dpkg|pacman|brew|sudo|su|env|export|source|alias|unalias|set|unset|declare|history|man|info|help|vim|vi|nvim|nano|emacs|reboot|shutdown|halt|poweroff)\b`,
			Color: "#dcdcaa", Enabled: true,
		},
		// ── 16. 命令选项 -x / --xxx（青色）──────────────────────────────────────
		{
			ID: "hl-flag", Name: "命令选项",
			Pattern: `(?<![\w/])--?[a-zA-Z][a-zA-Z0-9_-]*`,
			Color: "#56b6c2", Enabled: true,
		},
		// ── 17. 管道 / 重定向操作符（紫色）─────────────────────────────────────
		{
			ID: "hl-operator", Name: "管道/操作符",
			Pattern: `\|\|?|&&|>>?|2>&1|&>`,
			Color: "#c678dd", Enabled: true,
		},
		// ── 18. 布尔/空值（true 绿，false/null 琥珀）───────────────────────────
		{
			ID: "hl-true", Name: "true/yes/on",
			Pattern: `\b(true|True|TRUE|yes|Yes|YES|on|On|ON|enabled|Enabled|ENABLED)\b`,
			Color: "#6a9955", Enabled: true,
		},
		{
			ID: "hl-false", Name: "false/null/no",
			Pattern: `\b(false|False|FALSE|no|No|NO|off|Off|OFF|null|NULL|nil|Nil|NIL|none|None|NONE|undefined|NaN|disabled|Disabled|DISABLED)\b`,
			Color: "#cd9731", Enabled: true,
		},
	}
}

var (
	mu       sync.RWMutex
	current  Settings
	initOnce sync.Once
)

func settingsPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "conch")
	MigrateLegacyDir(filepath.Join(base, "gssh"), dir) // 老版本目录 gssh → conch
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// MigrateLegacyDir renames an old data directory to the new one, once, when the
// new directory does not yet exist and the old one does. Best-effort: on failure
// the caller simply creates a fresh new directory.
//
// 注意：这只搬数据文件目录，与密码加密密钥无关 —— keystore.machineKey 的种子
// 常量仍是 "gssh"，绝不能改，否则已保存的密码会全部解不开。
func MigrateLegacyDir(oldDir, newDir string) {
	if _, err := os.Stat(newDir); err == nil {
		return // 新目录已存在，不动
	}
	if _, err := os.Stat(oldDir); err != nil {
		return // 没有老目录
	}
	_ = os.Rename(oldDir, newDir)
}

// Init loads settings from disk (or creates defaults).
func Init() error {
	var loadErr error
	initOnce.Do(func() {
		current = defaults()
		p, err := settingsPath()
		if err != nil {
			loadErr = err
			return
		}
		data, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			return // use defaults, write on first save
		}
		if err != nil {
			loadErr = err
			return
		}
		loadErr = json.Unmarshal(data, &current)
	})
	return loadErr
}

// Get returns a copy of the current settings.
func Get() Settings {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Save persists new settings to disk.
func Save(s Settings) error {
	mu.Lock()
	current = s
	mu.Unlock()

	p, err := settingsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
