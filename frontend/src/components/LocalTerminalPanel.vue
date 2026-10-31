<template>
  <div class="lt-panel" :style="{ background: termBg }">
    <div ref="containerRef" class="lt-xterm-host" />

    <!-- 退出遮罩 -->
    <Transition name="lt-fade">
      <div v-if="exited" class="lt-exit-mask">
        <div class="lt-exit-box">
          <div class="lt-exit-text">本地 shell 已退出</div>
          <button class="lt-restart-btn" @click="restart">重新启动</button>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { StartLocalTerm, SendLocalInput, ResizeLocalTerm, StopLocalTerm } from '../../wailsjs/go/main/App'
import { useSettingsStore } from '../stores/settings'
import { THEMES } from '../terminal-themes'

const props = defineProps<{ id: string; shell: string }>()

const settingsStore = useSettingsStore()
const containerRef  = ref<HTMLDivElement>()
const exited        = ref(false)

let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let resizeObserver: ResizeObserver | null = null
let lastCols = 0
let lastRows = 0

const _encoder = new TextEncoder()

const termBg = computed(() => {
  const scheme = settingsStore.settings.color_scheme ?? 'github-dark'
  return (THEMES[scheme] ?? THEMES['github-dark']).background as string
})

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

/** 启动（或重启）本地 shell */
async function start() {
  if (!term || !fitAddon) return
  exited.value = false
  fitAddon.fit()
  lastCols = term.cols
  lastRows = term.rows
  try {
    await StartLocalTerm(props.id, props.shell, term.cols, term.rows)
  } catch (e) {
    term.write(`\r\n\x1b[31m[本地终端启动失败] ${String(e)}\x1b[0m\r\n`)
    exited.value = true
  }
}

function restart() {
  term?.clear()
  start()
}

/** 当面板从隐藏切回可见时重新适配尺寸 */
function refit() {
  if (!fitAddon || !term) return
  if (!containerRef.value?.offsetParent) return
  fitAddon.fit()
  if (term.cols !== lastCols || term.rows !== lastRows) {
    lastCols = term.cols
    lastRows = term.rows
    ResizeLocalTerm(props.id, term.cols, term.rows).catch(() => {})
  }
  term.focus()
}

defineExpose({ refit, focus: () => term?.focus() })

onMounted(async () => {
  if (!containerRef.value) return
  await document.fonts.ready

  term = new Terminal(buildOptions())
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(containerRef.value)

  // Ctrl+` 切换面板（拦截，避免发给本地 shell）
  term.attachCustomKeyEventHandler((e: KeyboardEvent) => {
    if (e.ctrlKey && e.key === '`' && e.type === 'keydown') {
      window.dispatchEvent(new CustomEvent('gssh:toggle-localterm'))
      return false
    }
    return true
  })

  // 键入 → 本地 shell
  term.onData((data) => {
    SendLocalInput(props.id, data).catch(() => {})
  })

  // 选中即复制
  term.onSelectionChange(() => {
    const sel = term?.getSelection()
    if (sel) navigator.clipboard.writeText(sel).catch(() => {})
  })

  // 本地 shell 输出
  EventsOn('localterm:output:' + props.id, (b64: string) => {
    if (!term) return
    try {
      term.write(Uint8Array.from(atob(b64), c => c.charCodeAt(0)))
    } catch {
      term.write(_encoder.encode(b64))
    }
  })

  // 本地 shell 退出
  EventsOn('localterm:exit:' + props.id, () => {
    exited.value = true
  })

  resizeObserver = new ResizeObserver(() => {
    if (!fitAddon || !term) return
    if (!containerRef.value?.offsetParent) return
    fitAddon.fit()
    if (term.cols !== lastCols || term.rows !== lastRows) {
      lastCols = term.cols
      lastRows = term.rows
      ResizeLocalTerm(props.id, term.cols, term.rows).catch(() => {})
    }
  })
  resizeObserver.observe(containerRef.value)

  await start()
  nextTick(() => term?.focus())
})

onUnmounted(() => {
  EventsOff('localterm:output:' + props.id)
  EventsOff('localterm:exit:' + props.id)
  resizeObserver?.disconnect()
  StopLocalTerm(props.id).catch(() => {})
  term?.dispose()
})

// 主题/字体变化时同步
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
</script>

<style scoped>
.lt-panel {
  position: relative;
  height: 100%;
  width: 100%;
  overflow: hidden;
}
.lt-xterm-host {
  width: 100%;
  height: 100%;
  padding: 2px 6px 0;
  box-sizing: border-box;
  background: inherit;
}

/* ── 退出遮罩 ── */
.lt-exit-mask {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(13,17,23,0.7);
  backdrop-filter: blur(2px);
}
.lt-exit-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}
.lt-exit-text { font-size: 13px; color: var(--text-muted); }
.lt-restart-btn {
  background: var(--accent);
  border: none;
  border-radius: 6px;
  color: #fff;
  font-size: 12px;
  padding: 6px 16px;
  cursor: pointer;
  transition: filter 0.15s;
}
.lt-restart-btn:hover { filter: brightness(1.1); }

.lt-fade-enter-active, .lt-fade-leave-active { transition: opacity 0.15s; }
.lt-fade-enter-from, .lt-fade-leave-to { opacity: 0; }
</style>
