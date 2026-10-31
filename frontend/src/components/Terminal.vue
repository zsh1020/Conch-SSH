<template>
  <div class="terminal-container" :style="{ background: termBg }" @contextmenu.prevent="onContextMenu">
    <div ref="containerRef" class="xterm-host" />

    <!-- 搜索栏 -->
    <Transition name="search-slide">
      <div v-if="searchVisible" class="search-bar" @keydown.esc="closeSearch">
        <input
          ref="searchInputRef"
          v-model="searchQuery"
          class="search-input"
          placeholder="搜索…"
          @keydown.enter.exact="findNext"
          @keydown.shift.enter="findPrev"
          @input="onSearchInput"
        />
        <span class="search-count">{{ searchStatus }}</span>
        <button class="search-nav" title="上一个 (Shift+Enter)" @click="findPrev">▲</button>
        <button class="search-nav" title="下一个 (Enter)" @click="findNext">▼</button>
        <button class="search-close" title="关闭 (Esc)" @click="closeSearch">✕</button>
      </div>
    </Transition>

    <!-- ZMODEM 传输进度条 -->
    <Transition name="zm-slide">
      <div v-if="zmodemActive" class="zmodem-bar">
        <span class="zm-icon">{{ zmodemInfo.direction === 'sz' ? '↓' : '↑' }}</span>
        <span class="zm-label">{{ zmodemInfo.direction === 'sz' ? '接收' : '发送' }}</span>
        <span class="zm-filename">{{ zmodemInfo.filename || '等待中…' }}</span>
        <div class="zm-track">
          <div class="zm-fill" :style="{ width: zmodemPercent + '%' }" />
        </div>
        <span class="zm-pct">{{ zmodemPercent }}%</span>
        <button class="zm-cancel" @click="zmodemCancel">✕</button>
      </div>
    </Transition>

    <!-- 隐藏文件选择框（rz 发送文件用） -->
    <input ref="fileInputZRef" type="file" multiple hidden @change="onZmodemFilePick" />

    <!-- 多行粘贴确认弹窗 -->
    <PasteConfirmDialog ref="pasteDialogRef" />

    <!-- 命令历史面板 -->
    <HistoryPanel
      ref="historyPanelRef"
      :conn-id="props.connId"
      @insert="onHistoryInsert"
    />

    <!-- 右键菜单 -->
    <div
      v-if="ctxMenu.visible"
      class="ctx-menu"
      :style="{ top: ctxMenu.y + 'px', left: ctxMenu.x + 'px' }"
    >
      <div class="ctx-item" @click="onCtxCopy">复制</div>
      <div class="ctx-item" @click="onCtxPaste">粘贴</div>
      <div class="ctx-divider" />
      <div class="ctx-item" @click="onCtxSelectAll">全选</div>
      <div class="ctx-item" @click="onCtxClear">清屏</div>
      <div class="ctx-divider" />
      <div class="ctx-item" @click="onCtxHistory">命令历史 (Ctrl+H)</div>
    </div>
    <div v-if="ctxMenu.visible" class="ctx-overlay" @click="ctxMenu.visible = false" />

    <!-- 滚动到底部按钮（当用户向上翻历史时出现） -->
    <Transition name="scroll-btn">
      <button
        v-if="scrolledUp"
        class="scroll-to-bottom"
        title="滚动到底部"
        @click="scrollToBottom"
      >
        <span class="stb-arrow">↓</span>
        <span v-if="scrolledLines > 0" class="stb-badge">{{ scrolledLines > 999 ? '999+' : scrolledLines }}</span>
      </button>
    </Transition>

    <!-- 断线遮罩 -->
    <Transition name="fade">
      <div v-if="disconnected" class="disconn-mask">
        <div class="disconn-box">
          <div class="disconn-icon">⚡</div>
          <div class="disconn-title">连接已断开</div>
          <div class="disconn-hint">{{ reconnectHint }}</div>
          <div class="disconn-actions">
            <button
              class="disconn-btn primary"
              :disabled="reconnecting"
              @click="doReconnect"
            >
              {{ reconnecting ? '连接中…' : '重新连接' }}
            </button>
            <button class="disconn-btn" @click="$emit('close-tab')">关闭标签</button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { SearchAddon } from '@xterm/addon-search'
import { Sentry as ZSentry } from 'zmodem.js'
import type { ZSession, ZDetection } from 'zmodem.js'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { SendInput, SendInputBase64, ResizeTerminal, ZmodemSave, TerminalReady } from '../../wailsjs/go/main/App'
import { useSettingsStore } from '../stores/settings'
import { useTabsStore } from '../stores/tabs'
import { THEMES } from '../terminal-themes'
import PasteConfirmDialog from './PasteConfirmDialog.vue'
import HistoryPanel from './HistoryPanel.vue'

const props = defineProps<{ connId: string }>()
const emit  = defineEmits<{ closed: []; reconnect: []; 'close-tab': [] }>()

/**
 * 逐字节扫描，跳过所有 ANSI 控制序列，判断是否存在"真实"输出。
 *
 * 触发条件（任一满足）：
 *   - 换行符数量 ≥ 2：说明有实际内容行（命令输出 \r\n + 新提示符 \r\n）
 *   - 可见字符数 > 80：无换行的密集输出（进度条等）
 *
 * 设计原因：仅按 Enter 重绘提示符时只有 1 个 \n，即使提示符很长（如含完整主机名）
 * 也不会误触发蓝点提示。
 */
function hasVisibleOutput(bytes: Uint8Array): boolean {
  let n = 0
  let lfCount = 0
  for (let i = 0; i < bytes.length; i++) {
    const b = bytes[i]
    if (b === 0x0a) { lfCount++ }
    // ASCII 可打印字符（含空格）
    if (b >= 0x20 && b <= 0x7e) { n++; continue }
    // 换行/回车也计数
    if (b === 0x0a || b === 0x0d) { n++; continue }
    // ESC 控制序列 — 跳过整个序列
    if (b === 0x1b && i + 1 < bytes.length) {
      const next = bytes[i + 1]
      if (next === 0x5b) { // CSI
        i += 2
        while (i < bytes.length && !(bytes[i] >= 0x40 && bytes[i] <= 0x7e)) i++
      } else if (next === 0x5d) { // OSC
        i += 2
        while (i < bytes.length && !(bytes[i] === 0x07 || (bytes[i] === 0x1b && i + 1 < bytes.length && bytes[i + 1] === 0x5c))) i++
      } else if (next === 0x50 || next === 0x58 || next === 0x5e || next === 0x5f) {
        i += 2
        while (i < bytes.length && !(bytes[i] === 0x1b && i + 1 < bytes.length && bytes[i + 1] === 0x5c)) i++
      } else if (next === 0x4f) {
        i += 2
      } else {
        i += 2
      }
    }
  }
  return lfCount >= 2 || n > 80
}

const settingsStore  = useSettingsStore()
const tabsStore      = useTabsStore()
const containerRef   = ref<HTMLDivElement>()
const searchInputRef = ref<HTMLInputElement>()
const searchVisible  = ref(false)
const searchQuery    = ref('')
const searchStatus   = ref('')

const ctxMenu        = ref({ visible: false, x: 0, y: 0 })
const disconnected   = ref(false)
const scrolledUp     = ref(false)
const scrolledLines  = ref(0)
const reconnecting   = ref(false)
const pasteDialogRef = ref<InstanceType<typeof PasteConfirmDialog>>()
const historyPanelRef = ref<InstanceType<typeof HistoryPanel>>()

// ─── 自动重连倒计时 ───────────────────────────────────────────────────────────
let _countdownTimer: ReturnType<typeof setInterval> | null = null
const reconnectCountdown = ref(0)   // 剩余秒数，0 = 未计划

const reconnectHint = computed(() => {
  if (reconnecting.value) return '正在重新连接…'
  if (reconnectCountdown.value > 0) {
    const tab = tabsStore.tabs.find(t => t.connId === props.connId)
    const attempt = tab?.reconnectCount ?? 0
    return `${reconnectCountdown.value}s 后自动重连（第 ${attempt}/3 次）`
  }
  return '会话已终止'
})

function startCountdown(targetMs: number) {
  if (_countdownTimer) clearInterval(_countdownTimer)
  reconnectCountdown.value = Math.max(0, Math.ceil((targetMs - Date.now()) / 1000))
  _countdownTimer = setInterval(() => {
    reconnectCountdown.value = Math.max(0, Math.ceil((targetMs - Date.now()) / 1000))
    if (reconnectCountdown.value <= 0 && _countdownTimer) {
      clearInterval(_countdownTimer)
      _countdownTimer = null
    }
  }, 300)
}

watch(
  () => tabsStore.tabs.find(t => t.connId === props.connId)?.reconnectAt,
  (at) => {
    if (at && at > Date.now()) startCountdown(at)
    else { reconnectCountdown.value = 0; if (_countdownTimer) { clearInterval(_countdownTimer); _countdownTimer = null } }
  }
)

// ─── ZMODEM state ─────────────────────────────────────────────────────────────
const zmodemActive  = ref(false)
const zmodemInfo    = ref({ direction: '' as 'sz' | 'rz', filename: '', done: 0, total: 0 })
const fileInputZRef = ref<HTMLInputElement>()
let zmodemSession: ZSession | null = null
let zsentry: InstanceType<typeof ZSentry> | null = null

const zmodemPercent = computed(() => {
  const { done, total } = zmodemInfo.value
  if (!total) return 0
  return Math.min(100, Math.round((done / total) * 100))
})

/** 安全地将 Uint8Array 转为 base64（避免大数组 spread 溢出栈） */
function uint8ToBase64(bytes: Uint8Array): string {
  let binary = ''
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i])
  return btoa(binary)
}

/** 用户点击取消 / 传输出错时中断 ZMODEM 会话 */
function zmodemCancel() {
  try { zmodemSession?.abort() } catch {}
  zmodemActive.value  = false
  zmodemSession       = null
}

/** sz：远端发送文件，我方接收 */
async function handleZmodemReceive(session: ZSession) {
  zmodemInfo.value = { direction: 'sz', filename: '', done: 0, total: 0 }

  // 接收文件大小硬上限 256 MB，避免单文件吃光内存（用户大文件请用 SFTP）
  const MAX_ZMODEM_SIZE = 256 * 1024 * 1024

  session.on('offer', async (xfer) => {
    const details = xfer.get_details()
    zmodemInfo.value.filename = details.name
    zmodemInfo.value.total    = details.size ?? 0
    zmodemInfo.value.done     = 0

    if ((details.size ?? 0) > MAX_ZMODEM_SIZE) {
      try { xfer.skip() } catch {}
      zmodemActive.value = false
      console.warn('[gssh-zmodem] 文件过大已跳过，请改用 SFTP：', details.name, details.size)
      return
    }

    const chunks: Uint8Array[] = []
    let totalLen = 0
    let aborted = false
    await xfer.accept({
      on_input: (payload: number[]) => {
        if (aborted) return
        totalLen += payload.length
        if (totalLen > MAX_ZMODEM_SIZE) {
          aborted = true
          chunks.length = 0  // 释放已累积的数据
          try { xfer.skip() } catch {}
          return
        }
        chunks.push(payload instanceof Uint8Array ? payload : new Uint8Array(payload))
        zmodemInfo.value.done = totalLen
      },
    })

    if (aborted) return

    // 拼接所有分块
    const merged = new Uint8Array(totalLen)
    let offset = 0
    for (const c of chunks) { merged.set(c, offset); offset += c.length }
    chunks.length = 0  // 释放分块引用

    try {
      await ZmodemSave(details.name, uint8ToBase64(merged))
    } catch (e) {
      console.error('ZmodemSave error', e)
    }
  })

  try {
    await session.start()
  } finally {
    zmodemActive.value = false
    zmodemSession      = null
  }
}

/** rz：远端等待接收文件，我方发送（触发文件选择框） */
function handleZmodemSend(session: ZSession) {
  zmodemSession = session
  zmodemInfo.value = { direction: 'rz', filename: '', done: 0, total: 0 }
  nextTick(() => fileInputZRef.value?.click())
}

/** 用户选完文件后通过 ZMODEM 发送 */
async function onZmodemFilePick(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''   // 重置，允许再次选同一文件

  if (!zmodemSession || files.length === 0) {
    zmodemCancel()
    return
  }

  const session = zmodemSession
  const CHUNK   = 8 * 1024  // 8 KB per chunk

  for (const file of files) {
    zmodemInfo.value.filename = file.name
    zmodemInfo.value.total    = file.size
    zmodemInfo.value.done     = 0

    const buffer = await file.arrayBuffer()
    const data   = new Uint8Array(buffer)

    const xfer = await session.send_offer({
      name:  file.name,
      size:  file.size,
      mtime: new Date(file.lastModified),
    })
    if (xfer === undefined) continue  // 对端跳过此文件

    let pos = 0
    while (pos < data.length) {
      const end   = Math.min(pos + CHUNK, data.length)
      const chunk = data.slice(pos, end)
      pos = end
      zmodemInfo.value.done = pos
      if (pos >= data.length) {
        await xfer.end(chunk)
      } else {
        xfer.send(chunk)
      }
    }
    if (data.length === 0) await xfer.end([])
  }

  try {
    await session.close()
  } catch {}
  zmodemActive.value = false
  zmodemSession      = null
}

/** 检测到 ZMODEM 会话的统一入口 */
function onZmodemDetect(detection: ZDetection) {
  const session = detection.confirm()
  zmodemSession = session
  zmodemActive.value = true

  if (session.type === 'receive') {
    handleZmodemReceive(session)
  } else {
    handleZmodemSend(session)
  }
}

let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let searchAddon: SearchAddon | null = null
let resizeObserver: ResizeObserver | null = null
let trackTimer: ReturnType<typeof setTimeout> | null = null

// ─── 写入背压（防止后台 rAF 暂停时 xterm writeBuffer 无限增长） ──────────────
// xterm.js v6 的 term.write() 把数据 push 到内部 _writeBuffer，依赖 rAF 异步消费。
// 窗口隐藏 / 后台时 rAF 大幅降频甚至完全暂停，但 SSH 流持续涌入 → 内部 buffer
// 可以累积到数 GB。通过 callback 跟踪 in-flight 字节数，超阈值时丢弃 SSH 输出，
// 避免进程内存爆炸（用户回到前台时丢失的是不可见的历史输出，可接受）。
const MAX_PENDING_BYTES = 8 * 1024 * 1024  // 8 MB 上限
let pendingBytes = 0
let lossNoticeShown = false

/** 用回调跟踪 xterm 实际消费进度。在 in-flight 字节数过高时返回 false，
 *  调用方应丢弃数据避免内存爆炸。 */
function writeToTerminal(bytes: Uint8Array): boolean {
  if (!term) return false
  if (pendingBytes >= MAX_PENDING_BYTES) {
    if (!lossNoticeShown) {
      lossNoticeShown = true
      term.write('\r\n\x1b[33m[conch] 终端缓冲区过载，已丢弃部分输出以保护内存\x1b[0m\r\n')
    }
    return false
  }
  pendingBytes += bytes.length
  term.write(bytes, () => {
    pendingBytes -= bytes.length
    if (pendingBytes < MAX_PENDING_BYTES / 2) lossNoticeShown = false
  })
  return true
}


// 窗口失去焦点时停止光标闪烁，恢复焦点时按设置还原
function onWindowBlur()  { if (term) term.options.cursorBlink = false }
function onWindowFocus() { if (term) term.options.cursorBlink = settingsStore.settings.cursor_blink }

// 终端容器背景色直接由 Vue 响应式驱动，避免 CSS 变量时机不确定的问题
const termBg = computed(() => {
  const scheme = settingsStore.settings.color_scheme ?? 'github-dark'
  return (THEMES[scheme] ?? THEMES['github-dark']).background as string
})

// ─── Output highlight engine ──────────────────────────────────────────────────
// 工作原理（仿 WindTerm 渲染层方案）：
//   1. 将字节流按 ANSI 颜色码分割为段
//   2. 仅对"不在服务端着色区域"的纯文本段应用规则
//   3. 服务端 ANSI 颜色优先，不被覆盖

function hlHexToRgb(hex: string): [number, number, number] | null {
  const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex)
  return m ? [parseInt(m[1], 16), parseInt(m[2], 16), parseInt(m[3], 16)] : null
}

function hlBuildPrefix(color: string, bgColor: string): string {
  let s = ''
  const fg = hlHexToRgb(color)
  if (fg) s += `\x1b[38;2;${fg[0]};${fg[1]};${fg[2]}m`
  const bg = hlHexToRgb(bgColor)
  if (bg) s += `\x1b[48;2;${bg[0]};${bg[1]};${bg[2]}m`
  return s
}

// 单遍扫描：先在原始文本上收集所有规则的所有匹配位置，再一次性构建输出。
// 避免顺序应用时后续规则匹配到前面规则已插入的 ANSI 转义序列内部的数字/字母，
// 导致 ANSI 码损坏、xterm.js 解析失败、颜色全部丢失的问题。
interface _HLMatch { start: number; end: number; prefix: string }
interface _CompiledRule { re: RegExp; prefix: string }

// 预编译规则缓存 —— 只在 highlight_rules 设置变化时重建，避免每次输出都 new RegExp
let _compiledRules: _CompiledRule[] = []

// 内置：Shell 提示符整体上色为绿色（对齐设计稿）。放在最前 → 优先级最高，
// 且仅对「服务端未着色」的提示符生效（若服务器 PS1 已带色则不覆盖）。
function buildPromptRule(): _CompiledRule | null {
  try {
    return {
      re: /\[[\w.-]+@[\w.-]+[^\]\n]*\][#$]/g,
      prefix: hlBuildPrefix('#46cf8b', ''),
    }
  } catch { return null }
}

watch(
  () => settingsStore.settings.highlight_rules,
  (rules) => {
    const compiled = (rules ?? []).flatMap(r => {
      if (!r.enabled || !r.pattern) return []
      try {
        const prefix = hlBuildPrefix(r.color, r.bg_color ?? '')
        if (!prefix) return []
        return [{ re: new RegExp(r.pattern, 'g'), prefix }]
      } catch { return [] }
    })
    const prompt = buildPromptRule()
    _compiledRules = prompt ? [prompt, ...compiled] : compiled
  },
  { immediate: true, deep: true },
)

function hlApplyRules(text: string): string {
  const hits: _HLMatch[] = []

  for (const { re, prefix } of _compiledRules) {
    re.lastIndex = 0
    let m: RegExpExecArray | null
    while ((m = re.exec(text)) !== null) {
      const s = m.index, e = m.index + m[0].length
      // 只取不与已有匹配重叠的部分（先匹配的规则优先级更高）
      if (!hits.some(h => s < h.end && e > h.start)) {
        hits.push({ start: s, end: e, prefix })
      }
    }
  }

  if (hits.length === 0) return text

  // 按位置排序后一次性拼接，不对原始文本做任何修改
  hits.sort((a, b) => a.start - b.start)
  let out = '', pos = 0
  for (const h of hits) {
    out += text.slice(pos, h.start)
    out += h.prefix + text.slice(h.start, h.end) + '\x1b[0m'
    pos = h.end
  }
  return out + text.slice(pos)
}

// 判断 ANSI 颜色码是否将状态重置回默认
// 处理 \x1b[m, \x1b[0m, \x1b[39m (默认前景), \x1b[49m (默认背景),
// 以及形如 \x1b[31;0m 这种最后一个参数是 0 的复合码
function hlIsAnsiReset(code: string): boolean {
  const inner = code.slice(2, -1)
  if (!inner) return true
  const params = inner.split(';')
  const last = params[params.length - 1]
  return last === '' || /^0+$/.test(last) || last === '39' || last === '49'
}

// 一次性诊断标志（仅首次调用时打印，方便验证引擎是否运行）
let __hlDiag = false

// 匹配所有 ANSI/VT 转义序列，确保它们不被高亮规则污染：
//   CSI  : \x1b[ ... final-byte (0x40–0x7E)
//   OSC  : \x1b] ... BEL(\x07) 或 ST(\x1b\\)   ← 终端标题序列，之前漏处理
//   DCS/SOS/PM/APC : \x1b[PX^_] ... ST
//   SS3  : \x1bO final-byte
//   其余双字符 ESC + single-char
const _ESC_RE = /\x1b(?:\[[^@-~]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)|[PX^_][^\x1b]*\x1b\\|O[A-Za-z]|[^\[\]OPX^_])/g

// 复用 codec 实例，避免每次输出都分配 TextDecoder / TextEncoder
const _decoder = new TextDecoder('utf-8', { fatal: false })
const _encoder = new TextEncoder()

// 上一块结尾「不完整的转义序列 / 多字节 UTF-8」残留字节，拼到下一块再处理。
// 后端输出泵按 16ms/64KB 攒批 flush，边界是任意字节位置，会把一条转义序列或
// 一个中文字符从中间切断；若不跨块拼接，被切断的序列会被高亮引擎当成文本污染，
// 导致多行命令回显时光标定位错乱（行错位、重复）。
let _hlCarry: Uint8Array | null = null

// 判断 b[esc..end) 是否构成一个完整的转义序列。
function hlEscComplete(b: Uint8Array, esc: number, end: number): boolean {
  if (esc + 1 >= end) return false             // 只有 ESC
  const c1 = b[esc + 1]
  if (c1 === 0x5b) {                            // CSI: ESC [ … final 0x40–0x7e
    for (let i = esc + 2; i < end; i++) if (b[i] >= 0x40 && b[i] <= 0x7e) return true
    return false
  }
  if (c1 === 0x5d) {                            // OSC: ESC ] … BEL 或 ST(ESC \)
    for (let i = esc + 2; i < end; i++) {
      if (b[i] === 0x07) return true
      if (b[i] === 0x1b && i + 1 < end && b[i + 1] === 0x5c) return true
    }
    return false
  }
  if (c1 === 0x50 || c1 === 0x58 || c1 === 0x5e || c1 === 0x5f) {
    // DCS/SOS/PM/APC: ESC P|X|^|_ … ST(ESC \)
    for (let i = esc + 2; i < end; i++) {
      if (b[i] === 0x1b && i + 1 < end && b[i + 1] === 0x5c) return true
    }
    return false
  }
  if (c1 === 0x4f) return esc + 2 < end          // SS3: ESC O + 1 byte
  return true                                    // 其余 ESC + 单字节形式
}

// 计算本块可安全处理到的字节数：截掉结尾不完整的转义序列 / 多字节 UTF-8。
function hlSafeEnd(b: Uint8Array): number {
  const len = b.length
  if (len === 0) return 0

  // 1) 结尾不完整的转义序列：找最近的 ESC(0x1b)，若到结尾未闭合则从它起留到下一块。
  //    全量向前扫（不设固定窗口），确保超长 OSC/DCS 被批边界切断时也能正确识别；
  //    但用 MAX_INCOMPLETE_ESC 兜底：畸形/永不闭合的序列不再滞留，原样放行以免
  //    _hlCarry 无限增长。
  const MAX_INCOMPLETE_ESC = 64 * 1024
  let esc = -1
  for (let i = len - 1; i >= 0; i--) {
    if (b[i] === 0x1b) { esc = i; break }
  }
  if (esc >= 0 && !hlEscComplete(b, esc, len)) {
    return len - esc > MAX_INCOMPLETE_ESC ? len : esc
  }

  // 2) 结尾不完整的多字节 UTF-8：回退未闭合的 lead byte
  let i = len
  while (i > 0 && len - i < 4) {
    const c = b[i - 1]
    if (c < 0x80) break                          // ASCII，完整
    if ((c & 0xc0) === 0x80) { i--; continue }    // continuation，继续往前
    let need = 0
    if ((c & 0xe0) === 0xc0) need = 2
    else if ((c & 0xf0) === 0xe0) need = 3
    else if ((c & 0xf8) === 0xf0) need = 4
    else break                                    // 非法 lead，交给解码器
    if (len - (i - 1) < need) return i - 1        // 不完整，从 lead 起留到下一块
    break
  }
  return len
}

// 按所有转义序列分段，仅对纯文本段应用高亮规则；
// 转义序列段原封不动透传，CSI 颜色码同时用于追踪"服务端已着色"状态。
function applyHighlightRules(bytes: Uint8Array): Uint8Array {
  if (_compiledRules.length === 0) return bytes

  // 拼接上一块残留的不完整尾字节
  if (_hlCarry && _hlCarry.length) {
    const merged = new Uint8Array(_hlCarry.length + bytes.length)
    merged.set(_hlCarry, 0)
    merged.set(bytes, _hlCarry.length)
    bytes = merged
    _hlCarry = null
  }
  // 从尾部切掉不完整的转义序列 / UTF-8，留到下一块
  const keep = hlSafeEnd(bytes)
  if (keep < bytes.length) {
    _hlCarry = bytes.slice(keep)
    bytes = bytes.subarray(0, keep)
    if (bytes.length === 0) return new Uint8Array(0)  // 整块都是残留，等下一块
  }

  if (!__hlDiag) {
    __hlDiag = true
    console.warn('[gssh-hl] engine first-call:', {
      rulesCount: _compiledRules.length,
      preview: _decoder.decode(bytes.slice(0, 80)).replace(/\x1b/g, '\\x1b'),
    })
  }

  let text: string
  try {
    text = _decoder.decode(bytes)
  } catch {
    return bytes
  }

  // 将文本切成：纯文本段 + 转义序列段（交替）
  interface Seg { start: number; end: number; isEsc: boolean }
  const segs: Seg[] = []
  let pos = 0
  _ESC_RE.lastIndex = 0
  let m: RegExpExecArray | null
  while ((m = _ESC_RE.exec(text)) !== null) {
    if (m.index > pos) segs.push({ start: pos, end: m.index, isEsc: false })
    segs.push({ start: m.index, end: m.index + m[0].length, isEsc: true })
    pos = m.index + m[0].length
  }
  if (pos < text.length) segs.push({ start: pos, end: text.length, isEsc: false })

  let serverColor = false
  const out: string[] = []

  for (const seg of segs) {
    const chunk = text.slice(seg.start, seg.end)
    if (seg.isEsc) {
      // CSI 颜色码：追踪服务端着色状态；其他转义序列不影响状态
      if (chunk.startsWith('\x1b[') && chunk.endsWith('m')) {
        serverColor = !hlIsAnsiReset(chunk)
      }
      out.push(chunk)           // 转义序列始终原样透传
    } else {
      out.push(serverColor ? chunk : hlApplyRules(chunk))
    }
  }

  return _encoder.encode(out.join(''))
}


function buildOptions() {
  const s = settingsStore.settings
  return {
    theme:       THEMES[s.color_scheme] ?? THEMES['github-dark'],
    fontFamily:  s.font_family || 'Cascadia Code, Consolas, monospace',
    fontSize:    s.font_size   || 14,
    fontWeight:  (s.font_weight || 400) as 100|200|300|400|500|600|700|800|900|'bold'|'normal',
    lineHeight:  1.1,
    cursorBlink: s.cursor_blink,
    cursorStyle: (s.cursor_style || 'block') as 'block' | 'bar' | 'underline',
    scrollback:  s.scrollback  || 5000,
    allowProposedApi: true,
  }
}

// ─── Search ───────────────────────────────────────────────────────────────────

function openSearch() {
  searchVisible.value = true
  nextTick(() => searchInputRef.value?.focus())
}

function closeSearch() {
  searchVisible.value = false
  searchQuery.value = ''
  searchStatus.value = ''
  searchAddon?.clearDecorations()
  term?.focus()
}

function onSearchInput() {
  if (!searchQuery.value) {
    searchAddon?.clearDecorations()
    searchStatus.value = ''
    return
  }
  const found = searchAddon?.findNext(searchQuery.value, { incremental: true, regex: false })
  searchStatus.value = found ? '' : '无结果'
}

function findNext() {
  if (!searchQuery.value) return
  const found = searchAddon?.findNext(searchQuery.value)
  searchStatus.value = found ? '' : '无结果'
}

function findPrev() {
  if (!searchQuery.value) return
  const found = searchAddon?.findPrevious(searchQuery.value)
  searchStatus.value = found ? '' : '无结果'
}

// ─── Right-click context menu ─────────────────────────────────────────────────

async function onContextMenu(e: MouseEvent) {
  e.preventDefault()
  ctxMenu.value = { visible: true, x: e.clientX, y: e.clientY }
  await nextTick()
  const menu = document.querySelector('.ctx-menu') as HTMLElement | null
  if (menu) {
    const rect = menu.getBoundingClientRect()
    if (rect.right  > window.innerWidth)  ctxMenu.value.x = e.clientX - rect.width
    if (rect.bottom > window.innerHeight) ctxMenu.value.y = e.clientY - rect.height
  }
}

function onCtxCopy() {
  ctxMenu.value.visible = false
  const sel = term?.getSelection()
  if (sel) navigator.clipboard.writeText(sel).catch(() => {})
}

/**
 * 统一粘贴入口（Ctrl+V 和右键菜单共用）。
 * 内容含 2+ 个非空行时弹出编辑确认框，否则直接发送。
 */
async function doPaste() {
  try {
    const text = await navigator.clipboard.readText()
    if (!text) { term?.focus(); return }

    const nonEmptyLines = text.split('\n').filter(l => l.trim() !== '')
    if (nonEmptyLines.length >= 2) {
      const edited = await pasteDialogRef.value?.open(text)
      if (edited != null) {
        SendInput(props.connId, edited).catch(() => {})
      }
    } else {
      SendInput(props.connId, text).catch(() => {})
    }
  } catch {
    // clipboard access denied — ignore
  }
  term?.focus()
}

async function onCtxPaste() {
  ctxMenu.value.visible = false
  await doPaste()
}

function onCtxSelectAll() {
  ctxMenu.value.visible = false
  term?.selectAll()
}

function onCtxClear() {
  ctxMenu.value.visible = false
  term?.clear()
}

// ─── 命令历史 ─────────────────────────────────────────────────────────────────

// 当前正在输入的命令行（逐键拼接，Enter 时提交）
let _cmdBuf = ''

/** 用户在终端输入一段数据 → 拆出命令并记录 */
function trackInput(data: string) {
  for (const ch of data) {
    if (ch === '\r' || ch === '\n') {
      historyPanelRef.value?.record(_cmdBuf)
      _cmdBuf = ''
    } else if (ch === '\x7f' || ch === '\b') {
      // Backspace
      _cmdBuf = _cmdBuf.slice(0, -1)
    } else if (ch >= ' ') {
      _cmdBuf += ch
    }
  }
}

function onHistoryInsert(cmd: string) {
  // 发送命令并回车（同步 _cmdBuf 以免误记）
  _cmdBuf = ''
  SendInput(props.connId, cmd + '\r').catch(() => {})
  term?.focus()
}

function onCtxHistory() {
  ctxMenu.value.visible = false
  historyPanelRef.value?.open()
}

function scrollToBottom() {
  term?.scrollToBottom()
}

// ─── Lifecycle ────────────────────────────────────────────────────────────────

onMounted(async () => {
  if (!containerRef.value) return

  // 等待字体加载完毕，避免 xterm.js 在字体未就绪时量算字符宽度出错（间距异常）
  await document.fonts.ready
  // 强制浏览器用当前设定字体渲染一个字符，确保字体度量已进入缓存
  const s = settingsStore.settings
  const fontFamily = s.font_family || 'Cascadia Code, Consolas, monospace'
  await document.fonts.load(`${s.font_weight || 400} ${s.font_size || 14}px "${fontFamily.split(',')[0].trim()}"`)


  term = new Terminal(buildOptions())
  fitAddon    = new FitAddon()
  searchAddon = new SearchAddon()

  term.loadAddon(fitAddon)
  term.loadAddon(searchAddon)
  term.loadAddon(new WebLinksAddon())
  term.open(containerRef.value)
  fitAddon.fit()

  term.onScroll(() => {
    if (!term) return
    const buf = term.buffer.active
    const behind = buf.baseY - buf.viewportY
    scrolledLines.value  = behind
    scrolledUp.value     = behind > 0
  })

  term.onData((data) => {
    trackInput(data)
    SendInput(props.connId, data).catch(() => {})
  })

  // ── ZMODEM sentry ──────────────────────────────────────────────────────────
  // 所有来自远端的字节先经过 zsentry.consume()，sentry 自动分流：
  //   · 普通终端数据 → to_terminal → term.write()
  //   · ZMODEM 协议字节 → 内部处理，触发 on_detect
  // 注意：高亮在调用 consume 之前就已经处理过了，to_terminal 直接写入即可
  zsentry = new ZSentry({
    to_terminal: (octets: number[]) => {
      writeToTerminal(octets instanceof Uint8Array ? octets : new Uint8Array(octets))
    },
    sender: (octets: number[]) => {
      // ZMODEM 协议回包需要用 base64 通道确保二进制安全
      SendInputBase64(props.connId, btoa(String.fromCharCode(...octets))).catch(() => {})
    },
    on_retract: () => {
      zmodemActive.value = false
      zmodemSession      = null
    },
    on_detect: (detection: ZDetection) => {
      onZmodemDetect(detection)
    },
  })

  // Ctrl+F: open search bar; Ctrl+V: paste with multi-line guard
  term.attachCustomKeyEventHandler((e: KeyboardEvent) => {
    if (e.ctrlKey && (e.key === 'f' || e.key === 'F') && e.type === 'keydown') {
      openSearch()
      return false
    }
    if (e.ctrlKey && (e.key === 'h' || e.key === 'H') && e.type === 'keydown') {
      historyPanelRef.value?.open()
      return false
    }
    // Ctrl+` 切换底部本地终端（拦截，避免发给远端 shell）
    if (e.ctrlKey && e.key === '`' && e.type === 'keydown') {
      window.dispatchEvent(new CustomEvent('gssh:toggle-localterm'))
      return false
    }
    // Intercept Ctrl+V so we can show the edit dialog for multi-line content.
    // Returning false prevents xterm.js from handling the paste itself (onData).
    if (e.ctrlKey && e.key === 'v' && e.type === 'keydown') {
      doPaste()
      return false
    }
    return true
  })

  // 选中即复制：选区非空时自动写入剪贴板
  term.onSelectionChange(() => {
    const sel = term?.getSelection()
    if (sel) navigator.clipboard.writeText(sel).catch(() => {})
  })

  // 冷却标志：忽略初始 SSH 握手 / shell prompt 的输出，避免误标
  let trackNewOutput = false
  trackTimer = setTimeout(() => { trackNewOutput = true }, 150)

  EventsOn(`terminal:output:${props.connId}`, (b64: string) => {
    if (!term) return
    // 背压检查：in-flight 字节数过高时直接丢弃，避免后台 rAF 暂停时内存爆炸
    if (pendingBytes >= MAX_PENDING_BYTES) {
      if (!lossNoticeShown) {
        lossNoticeShown = true
        term.write('\r\n\x1b[33m[conch] 终端缓冲区过载，已丢弃部分输出以保护内存\x1b[0m\r\n')
      }
      return
    }
    let bytes: Uint8Array | null = null
    try {
      bytes = Uint8Array.from(atob(b64), c => c.charCodeAt(0))
    } catch {
      writeToTerminal(_encoder.encode(b64))
      return
    }
    // 非活跃 tab 且冷却期结束、且确有"可见"输出时才标记新输出
    if (
      trackNewOutput &&
      tabsStore.activeConnId !== props.connId &&
      hasVisibleOutput(bytes)
    ) {
      tabsStore.markNewOutput(props.connId)
    }
    // 在送入 zsentry 之前先做高亮处理（仅在非 ZMODEM 传输状态）
    let outBytes = bytes
    if (!zmodemActive.value) {
      try {
        outBytes = applyHighlightRules(bytes)
      } catch (e) {
        console.error('[gssh-hl] error:', e)
      }
    }
    try {
      zsentry!.consume(outBytes as unknown as number[])
    } catch {
      writeToTerminal(outBytes)
    }
  })

  EventsOn(`terminal:closed:${props.connId}`, () => {
    term?.write('\r\n\x1b[31m[Connection closed]\x1b[0m\r\n')
    // 若 tab 已标记为重连中（programmatic/auto reconnect），跳过断线遮罩
    const tab = tabsStore.tabs.find(t => t.connId === props.connId)
    if (tab?.status === 'connecting' || tab?.status === 'reconnecting') return
    disconnected.value = true
    emit('closed')
  })

  // 通知 Go 端前端已就绪，flush 在此之前缓冲的输出（如 Last login / MOTD）
  TerminalReady(props.connId).catch(() => {})

  let lastCols = 0
  let lastRows = 0
  resizeObserver = new ResizeObserver(() => {
    if (!fitAddon || !term) return
    // 容器不可见（display:none）时 offsetParent 为 null，跳过 resize
    // 避免切换标签时因尺寸 0→实际值触发 SIGWINCH 导致终端重绘
    if (!containerRef.value?.offsetParent) return
    fitAddon.fit()
    if (term.cols !== lastCols || term.rows !== lastRows) {
      lastCols = term.cols
      lastRows = term.rows
      ResizeTerminal(props.connId, term.cols, term.rows).catch(() => {})
    }
  })
  resizeObserver.observe(containerRef.value)

  window.addEventListener('blur',  onWindowBlur)
  window.addEventListener('focus', onWindowFocus)

  function onFocusTerminal(e: Event) {
    if ((e as CustomEvent).detail === props.connId) term?.focus()
  }
  window.addEventListener('gssh:focus-terminal', onFocusTerminal)
  onUnmounted(() => window.removeEventListener('gssh:focus-terminal', onFocusTerminal))

  function onFitTerminal(e: Event) {
    if ((e as CustomEvent).detail !== props.connId) return
    if (!fitAddon || !term) return
    fitAddon.fit()
    if (term.cols !== lastCols || term.rows !== lastRows) {
      lastCols = term.cols
      lastRows = term.rows
      ResizeTerminal(props.connId, term.cols, term.rows).catch(() => {})
    }
    term.focus()
  }
  window.addEventListener('gssh:fit-terminal', onFitTerminal)
  onUnmounted(() => window.removeEventListener('gssh:fit-terminal', onFitTerminal))
})

onUnmounted(() => {
  if (trackTimer !== null) clearTimeout(trackTimer)
  if (_countdownTimer !== null) clearInterval(_countdownTimer)
  EventsOff(`terminal:output:${props.connId}`)
  EventsOff(`terminal:closed:${props.connId}`)
  resizeObserver?.disconnect()
  window.removeEventListener('blur',  onWindowBlur)
  window.removeEventListener('focus', onWindowFocus)
  term?.dispose()
})

watch(
  () => settingsStore.settings,
  (s) => {
    if (!term) return
    term.options.theme       = THEMES[s.color_scheme] ?? THEMES['github-dark']
    term.options.fontFamily  = s.font_family
    term.options.fontSize    = s.font_size
    term.options.fontWeight  = (s.font_weight || 400) as 100|200|300|400|500|600|700|800|900|'bold'|'normal'
    term.options.cursorBlink = s.cursor_blink
    term.options.cursorStyle = s.cursor_style as 'block' | 'bar' | 'underline'
    term.options.scrollback  = s.scrollback
    fitAddon?.fit()
  },
  { deep: true }
)

// 重连：通知父组件，父组件完成后新的 connId 会刷新组件
async function doReconnect() {
  reconnecting.value = true
  emit('reconnect')
  // 父组件替换 connId 后组件会销毁重建，无需在此复位
  // 超时兜底：3s 后仍未重建则还原按钮
  setTimeout(() => { reconnecting.value = false }, 3000)
}

watch(() => props.connId, () => {
  disconnected.value = false
  reconnecting.value = false
  term?.focus()
})
defineExpose({ focus: () => term?.focus() })
</script>

<style scoped>
.terminal-container {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  /* background 由模板 :style="{ background: termBg }" 响应式注入 */
}

.xterm-host {
  width: 100%;
  height: 100%;
  background: inherit;   /* 从 .terminal-container 继承动态背景 */
}

/* ─── Search bar ─── */
.search-bar {
  position: absolute;
  top: 8px;
  right: 12px;
  display: flex;
  align-items: center;
  gap: 4px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px 6px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  z-index: 100;
}

.search-input {
  width: 200px;
  background: var(--bg-panel, #161b22);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 3px 8px;
  outline: none;
}

.search-input:focus { border-color: var(--accent); }

.search-count {
  font-size: 11px;
  color: var(--text-muted);
  min-width: 40px;
}

.search-nav,
.search-close {
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-muted);
  border-radius: 4px;
  padding: 2px 6px;
  cursor: pointer;
  font-size: 11px;
  line-height: 1;
}

.search-nav:hover,
.search-close:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.08);
}

.search-slide-enter-active,
.search-slide-leave-active { transition: opacity 0.15s, transform 0.15s; }
.search-slide-enter-from,
.search-slide-leave-to { opacity: 0; transform: translateY(-6px); }

/* ─── Context menu ─── */
.ctx-overlay {
  position: fixed;
  inset: 0;
  z-index: 299;
}

.ctx-menu {
  position: fixed;
  z-index: 300;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 7px;
  padding: 4px;
  min-width: 120px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
}

.ctx-item {
  padding: 7px 12px;
  font-size: 12px;
  color: var(--text-primary);
  border-radius: 4px;
  cursor: pointer;
  user-select: none;
}

.ctx-item:hover { background: rgba(255, 255, 255, 0.06); }

.ctx-divider {
  height: 1px;
  background: var(--border);
  margin: 3px 4px;
}

/* ─── 断线遮罩 ─── */
.disconn-mask {
  position: absolute;
  inset: 0;
  z-index: 50;
  background: rgba(13, 17, 23, 0.82);
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(2px);
}

.disconn-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 28px 36px;
  box-shadow: 0 20px 48px rgba(0,0,0,0.6);
  min-width: 240px;
  text-align: center;
}

.disconn-icon {
  font-size: 32px;
  opacity: 0.7;
}

.disconn-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
}

.disconn-hint {
  font-size: 12px;
  color: var(--text-muted);
}

.disconn-actions {
  display: flex;
  gap: 8px;
  margin-top: 6px;
}

.disconn-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 6px 16px;
  cursor: pointer;
  transition: background 0.15s;
}
.disconn-btn:hover:not(:disabled) { background: rgba(255,255,255,0.07); }
.disconn-btn:disabled { opacity: 0.45; cursor: not-allowed; }
.disconn-btn.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.disconn-btn.primary:hover:not(:disabled) { filter: brightness(1.1); }

.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

/* ─── 滚动到底部按钮 ─── */
.scroll-to-bottom {
  position: absolute;
  bottom: 18px;
  right: 18px;
  z-index: 90;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 10px 5px 8px;
  border-radius: 20px;
  background: var(--accent, #58a6ff);
  border: none;
  color: #fff;
  font-size: 12px;
  cursor: pointer;
  box-shadow: 0 2px 10px rgba(0,0,0,0.45);
  opacity: 0.88;
  transition: opacity 0.15s, transform 0.15s;
  white-space: nowrap;
  user-select: none;
}
.scroll-to-bottom:hover { opacity: 1; transform: translateY(-2px); }

.stb-arrow { font-size: 14px; line-height: 1; }
.stb-badge {
  font-size: 10px;
  font-weight: 700;
  background: rgba(0,0,0,0.25);
  border-radius: 10px;
  padding: 1px 5px;
}

.scroll-btn-enter-active, .scroll-btn-leave-active { transition: opacity 0.18s, transform 0.18s; }
.scroll-btn-enter-from, .scroll-btn-leave-to       { opacity: 0; transform: translateY(8px); }

/* ─── ZMODEM 进度条 ─── */
.zmodem-bar {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  background: rgba(13, 17, 23, 0.92);
  border-top: 1px solid var(--border);
  backdrop-filter: blur(4px);
  font-size: 12px;
  color: var(--text-primary);
}

.zm-icon  { font-size: 14px; color: var(--accent); flex-shrink: 0; }
.zm-label { flex-shrink: 0; color: var(--text-muted); }

.zm-filename {
  flex-shrink: 0;
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.zm-track {
  flex: 1;
  height: 4px;
  border-radius: 2px;
  background: rgba(255,255,255,0.1);
  overflow: hidden;
}

.zm-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 2px;
  transition: width 0.2s;
}

.zm-pct {
  flex-shrink: 0;
  width: 34px;
  text-align: right;
  color: var(--text-muted);
  font-size: 11px;
}

.zm-cancel {
  flex-shrink: 0;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-muted);
  font-size: 11px;
  padding: 2px 7px;
  cursor: pointer;
  line-height: 1.4;
}
.zm-cancel:hover { color: var(--danger); border-color: var(--danger); }

.zm-slide-enter-active, .zm-slide-leave-active { transition: transform 0.2s, opacity 0.2s; }
.zm-slide-enter-from, .zm-slide-leave-to { transform: translateY(100%); opacity: 0; }
</style>
