import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { GetSettings, SaveSettings } from '../../wailsjs/go/main/App'

export const FM_ALL_COLUMNS = ['name', 'mode', 'owner', 'group', 'size', 'mod_time'] as const
export type FMColumn = typeof FM_ALL_COLUMNS[number]

export const FM_COLUMN_LABELS: Record<FMColumn, string> = {
  name:     '文件名',
  mode:     '权限',
  owner:    '所有者',
  group:    '用户组',
  size:     '大小',
  mod_time: '修改时间',
}

export interface HighlightRule {
  id:        string
  name:      string
  pattern:   string
  color:     string    // hex foreground
  bg_color?: string    // hex background (omitted = none)
  enabled:   boolean
}

// WindTerm-inspired 默认高亮规则
// 参考 WindTerm 五大语义 token 域 + VS Code semantic token 配色
export const DEFAULT_HIGHLIGHT_RULES: HighlightRule[] = [
  // ── 严重级别 ────────────────────────────────────────────────────────────────
  { id: 'hl-fatal',   name: '致命错误',     color: '#ff0000', bg_color: '#3a0000', enabled: true,
    pattern: '\\b(FATAL|CRITICAL|CRIT|EMERG|ALERT|PANIC|SIGKILL|SIGSEGV|SIGABRT)\\b' },
  { id: 'hl-error',   name: '错误',         color: '#f44747', enabled: true,
    pattern: '\\b(error|Error|ERROR|err|ERR|failed|Failed|FAILED|failure|Failure|FAILURE|exception|Exception|EXCEPTION|traceback|Traceback|panic|Abort|abort)\\b' },
  { id: 'hl-warn',    name: '警告',         color: '#cd9731', enabled: true,
    pattern: '\\b(warning|Warning|WARNING|warn|Warn|WARN|deprecated|Deprecated|DEPRECATED|caution|Caution|CAUTION)\\b' },
  { id: 'hl-success', name: '成功',         color: '#6a9955', enabled: true,
    pattern: '\\b(success|Success|SUCCESS|ok|OK|done|Done|DONE|passed|Passed|PASSED|pass|PASS|complete|Complete|COMPLETE|completed|Completed|COMPLETED|ready|Ready|READY|started|Started)\\b' },
  { id: 'hl-info',    name: '信息',         color: '#6796e6', enabled: true,
    pattern: '\\b(info|Info|INFO|notice|Notice|NOTICE|note|Note|NOTE|hint|Hint|HINT)\\b' },
  { id: 'hl-debug',   name: '调试',         color: '#b267e6', enabled: true,
    pattern: '\\b(debug|Debug|DEBUG|trace|Trace|TRACE|verbose|Verbose|VERBOSE)\\b' },
  // ── 括号日志级别 [LEVEL] ────────────────────────────────────────────────────
  { id: 'hl-bracket-error',   name: '[ERROR/FATAL]', color: '#f44747', bg_color: '#2d1515', enabled: true,
    pattern: '\\[(ERROR|FATAL|CRIT|CRITICAL|PANIC|FAIL|FAILED)\\]' },
  { id: 'hl-bracket-warn',    name: '[WARN]',        color: '#cd9731', bg_color: '#2a2010', enabled: true,
    pattern: '\\[(WARN|WARNING|CAUTION)\\]' },
  { id: 'hl-bracket-success', name: '[OK/PASS]',     color: '#6a9955', bg_color: '#0f1f0f', enabled: true,
    pattern: '\\[(OK|PASS|PASSED|SUCCESS|DONE)\\]' },
  { id: 'hl-bracket-info',    name: '[INFO]',        color: '#6796e6', enabled: true,
    pattern: '\\[(INFO|NOTICE)\\]' },
  { id: 'hl-bracket-debug',   name: '[DEBUG/TRACE]', color: '#b267e6', enabled: true,
    pattern: '\\[(DEBUG|TRACE|VERBOSE)\\]' },
  // ── HTTP ────────────────────────────────────────────────────────────────────
  { id: 'hl-http-error',  name: 'HTTP 错误状态码', color: '#f44747', enabled: true,
    pattern: '\\b[45]\\d{2}\\b' },
  { id: 'hl-http-ok',     name: 'HTTP 成功状态码', color: '#6a9955', enabled: true,
    pattern: '\\b[23]\\d{2}\\b' },
  { id: 'hl-http-method', name: 'HTTP 方法',       color: '#dcdcaa', enabled: true,
    pattern: '\\b(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|CONNECT|TRACE)\\b' },
  // ── 网络 / 系统 ─────────────────────────────────────────────────────────────
  { id: 'hl-ip',        name: 'IP 地址',  color: '#4ec9b0', enabled: true,
    pattern: '\\b(?:\\d{1,3}\\.){3}\\d{1,3}(?::\\d+)?\\b' },
  { id: 'hl-timestamp', name: '时间戳',   color: '#6a737d', enabled: true,
    pattern: '\\d{4}-\\d{2}-\\d{2}[T ]\\d{2}:\\d{2}:\\d{2}(?:[.,]\\d+)?(?:Z|[+-]\\d{2}:?\\d{2})?' },
  { id: 'hl-path',      name: '文件路径', color: '#9cdcfe', enabled: true,
    pattern: '(?:^|[\\s("])(/[\\w./~@:%-]+)' },
  { id: 'hl-var',       name: 'Shell 变量', color: '#9cdcfe', enabled: true,
    pattern: '\\$(?:\\{[\\w]+\\}|[A-Za-z_][\\w]*)' },
  // ── Shell 命令 / 选项 / 操作符 ──────────────────────────────────────────────
  { id: 'hl-cmd', name: 'Shell 命令', color: '#dcdcaa', enabled: true,
    pattern: '\\b(ls|ll|la|cat|tac|more|less|head|tail|grep|egrep|fgrep|zgrep|find|locate|which|whereis|type|file|stat|wc|sort|uniq|cut|tr|awk|gawk|sed|diff|patch|comm|tee|xargs|column|paste|join|od|xxd|hexdump|strings|echo|printf|read|yes|seq|shuf|bc|dc|expr|test|sleep|date|cal|uptime|uname|hostname|id|whoami|w|who|last|pwd|cd|pushd|popd|dirs|mkdir|rmdir|rm|cp|mv|ln|touch|chmod|chown|chgrp|chattr|lsattr|install|rsync|scp|sftp|ssh|ssh-keygen|ssh-copy-id|curl|wget|aria2c|ping|traceroute|dig|nslookup|host|netstat|ss|ip|ifconfig|route|nc|netcat|nmap|iptables|ufw|ps|top|htop|kill|pkill|killall|nohup|jobs|bg|fg|disown|watch|screen|tmux|lsof|fuser|strace|ltrace|lscpu|lspci|lsblk|lsusb|dmesg|df|du|free|mount|umount|fdisk|parted|mkfs|fsck|blkid|tar|zip|unzip|gzip|gunzip|bzip2|bunzip2|xz|7z|base64|md5sum|sha256sum|sha1sum|openssl|gpg|git|svn|make|cmake|gcc|clang|python|python3|pip|pip3|node|npm|yarn|go|cargo|ruby|gem|java|mvn|gradle|docker|kubectl|helm|systemctl|service|journalctl|crontab|apt|apt-get|yum|dnf|rpm|dpkg|pacman|brew|sudo|su|env|export|source|alias|unalias|set|unset|declare|history|man|info|help|vim|vi|nvim|nano|emacs|reboot|shutdown|halt|poweroff)\\b' },
  { id: 'hl-flag', name: '命令选项', color: '#56b6c2', enabled: true,
    pattern: '(?<![\\w/])--?[a-zA-Z][a-zA-Z0-9_-]*' },
  { id: 'hl-operator', name: '管道/操作符', color: '#c678dd', enabled: true,
    pattern: '\\|\\|?|&&|>>?|2>&1|&>' },
  // ── 数据值 ──────────────────────────────────────────────────────────────────
  { id: 'hl-number', name: '数字', color: '#b5cea8', enabled: true,
    pattern: '\\b\\d+(?:\\.\\d+)?(?:[KkMmGgTt][Bb?])?\\b' },
  { id: 'hl-true',  name: 'true / yes / on',   color: '#6a9955', enabled: true,
    pattern: '\\b(true|True|TRUE|yes|Yes|YES|on|On|ON|enabled|Enabled|ENABLED)\\b' },
  { id: 'hl-false', name: 'false / null / no',  color: '#cd9731', enabled: true,
    pattern: '\\b(false|False|FALSE|no|No|NO|off|Off|OFF|null|NULL|nil|Nil|NIL|none|None|NONE|undefined|NaN|disabled|Disabled|DISABLED)\\b' },
]

export interface Settings {
  theme: string
  color_scheme: string
  font_family: string
  font_size: number
  font_weight: number
  cursor_style: string
  cursor_blink: boolean
  scrollback: number
  bell_style: string
  local_shell: string
  show_recent: boolean
  fm_columns: string[]
  download_dir: string
  highlight_rules: HighlightRule[]
  // window geometry — managed by Go, frontend ignores but must keep in type
  window_width: number
  window_height: number
  window_maximized: boolean
}

const DEFAULT: Settings = {
  theme: 'dark',
  color_scheme: 'conch',
  font_family: 'Cascadia Code, JetBrains Mono, Fira Code, Consolas, monospace',
  font_size: 14,
  font_weight: 400,
  cursor_style: 'block',
  cursor_blink: true,
  scrollback: 5000,
  bell_style: 'none',
  local_shell: '',
  show_recent: true,
  fm_columns: ['name', 'mode', 'owner', 'group', 'size', 'mod_time'],
  download_dir: '',
  highlight_rules: DEFAULT_HIGHLIGHT_RULES,
  window_width: 1200,
  window_height: 800,
  window_maximized: false,
}

// 每种配色方案对应的背景色（light=true 时自动切换为浅色 UI 主题）
interface SchemeDef {
  bg: string; bgAlt: string; light?: boolean
  // 品牌强调色（仅 Conch 用青；其余留空 → 各自主题默认蓝）
  accent?: string; accentHover?: string
  // 设计令牌（留空则由 applyColorScheme 从 bg 派生，保证换肤时重构面板跟着变色）
  panel?: string; panel2?: string; term?: string; line?: string
  tp?: string; ts?: string; tm?: string
}

const SCHEME_BG: Record<string, SchemeDef> = {
  // ── Conch 设计方案（默认，像素级对应设计稿）──
  'conch': {
    bg: '#0f151d', bgAlt: '#141a22', accent: '#34c8d4', accentHover: '#4dd6e0',
    panel: '#141a22', panel2: '#111721', term: '#0b0e13', line: '#212a35',
    tp: '#d7dee7', ts: '#8c99a8', tm: '#5b6775',
  },
  'github-dark':     { bg: '#0d1117', bgAlt: '#161b22' },
  'dracula':         { bg: '#282a36', bgAlt: '#343746' },
  'one-dark':        { bg: '#282c34', bgAlt: '#2c313a' },
  'monokai':         { bg: '#272822', bgAlt: '#3e3d32' },
  'solarized-dark':  { bg: '#002b36', bgAlt: '#073642' },
  'nord':            { bg: '#2e3440', bgAlt: '#3b4252' },
  'catppuccin':      { bg: '#1e1e2e', bgAlt: '#313244' },
  'material':        { bg: '#263238', bgAlt: '#2e3c43' },
  'solarized-light': { bg: '#fdf6e3', bgAlt: '#eee8d5', light: true },
  'dark-charcoal':   { bg: '#1c1c1c', bgAlt: '#2a2a2a' },
  'carbon':          { bg: '#0c0c0c', bgAlt: '#1a1a1a' },
  'pure-black':      { bg: '#000000', bgAlt: '#111111' },
}

function hexToRgb(hex: string): [number, number, number] {
  let h = hex.replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)]
}
function softOf(hex: string, a = 0.14): string {
  const [r, g, b] = hexToRgb(hex)
  return `rgba(${r},${g},${b},${a})`
}

function applyColorScheme(scheme: string) {
  const c = SCHEME_BG[scheme] ?? SCHEME_BG['conch']
  const s = document.documentElement.style
  const light = !!c.light

  // 基础背景（旧面板沿用这些变量）
  s.setProperty('--bg-primary',   c.bg)
  s.setProperty('--bg-secondary', c.bgAlt)
  s.setProperty('--bg-panel',     c.bg)
  s.setProperty('--term-bg',      c.term ?? c.bg)   // 消除终端容器黑边

  // 强调色
  const accent = c.accent ?? (light ? '#0969da' : '#58a6ff')
  s.setProperty('--accent',       accent)
  s.setProperty('--accent-hover', c.accentHover ?? (light ? '#0550ae' : '#79b8ff'))
  s.setProperty('--accent-soft',  softOf(accent))

  // 设计令牌（重构后的面板使用；换肤时随之变化）
  s.setProperty('--panel',     c.panel  ?? c.bgAlt)
  s.setProperty('--panel-2',   c.panel2 ?? c.bg)
  s.setProperty('--term',      c.term   ?? c.bg)
  const line = c.line ?? (light ? 'rgba(0,0,0,0.14)' : 'rgba(255,255,255,0.10)')
  s.setProperty('--line',      line)
  s.setProperty('--border',    line)   // 旧面板沿用 --border → 跟随方案统一
  s.setProperty('--line-soft', light ? 'rgba(0,0,0,0.06)' : 'rgba(255,255,255,0.05)')
  s.setProperty('--hover',     light ? 'rgba(0,0,0,0.045)' : 'rgba(255,255,255,0.055)')
  s.setProperty('--tp', c.tp ?? (light ? '#1f2328' : '#d7dee7'))
  s.setProperty('--ts', c.ts ?? (light ? '#57606a' : '#8c99a8'))
  s.setProperty('--tm', c.tm ?? (light ? '#8a929c' : '#5b6775'))

  document.documentElement.setAttribute('data-theme', light ? 'light' : 'dark')
}

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings>({ ...DEFAULT })
  const loaded = ref(false)

  async function load() {
    try {
      const s = await GetSettings()
      settings.value = {
        ...DEFAULT,
        ...s,
        fm_columns:       s.fm_columns?.length       ? s.fm_columns       : DEFAULT.fm_columns,
        highlight_rules:  s.highlight_rules?.length  ? s.highlight_rules  : DEFAULT.highlight_rules,
      }
    } catch {
      settings.value = { ...DEFAULT }
    }
    loaded.value = true
    applyColorScheme(settings.value.color_scheme)  // 最后执行，覆盖 data-theme
  }

  async function save(s: Settings) {
    settings.value = s
    await SaveSettings(s as any)
    applyColorScheme(s.color_scheme)  // 最后执行，覆盖 data-theme
  }

  function applyTheme(theme: string) {
    document.documentElement.setAttribute('data-theme', theme)
  }

  watch(() => settings.value.color_scheme, applyColorScheme)

  return { settings, loaded, load, save }
})
