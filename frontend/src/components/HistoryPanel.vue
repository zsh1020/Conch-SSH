<template>
  <Transition name="hist-slide">
    <div v-if="visible" class="hist-panel" @keydown.esc="close" @mousedown.stop>
      <div class="hist-header">
        <span class="hist-title">命令历史</span>
        <div class="hist-header-actions">
          <button class="hist-clear-btn" title="清空历史" @click="clearAll">清空</button>
          <button class="hist-close-btn" @click="close">✕</button>
        </div>
      </div>

      <div class="hist-search-row">
        <input
          ref="searchRef"
          v-model="query"
          class="hist-search"
          placeholder="过滤命令…"
          spellcheck="false"
          @keydown.down.prevent="moveFocus(1)"
          @keydown.up.prevent="moveFocus(-1)"
          @keydown.enter.prevent="insertFocused"
        />
      </div>

      <div ref="listRef" class="hist-list">
        <div
          v-for="(cmd, idx) in filtered"
          :key="idx"
          class="hist-item"
          :class="{ focused: focusedIdx === idx }"
          @click="insert(cmd)"
          @mouseenter="focusedIdx = idx"
        >
          <span class="hist-cmd">{{ cmd }}</span>
          <button class="hist-del-btn" title="删除此条" @click.stop="deleteOne(cmd)">✕</button>
        </div>
        <div v-if="filtered.length === 0" class="hist-empty">无匹配命令</div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

const STORAGE_KEY = 'gssh-cmd-history'
const MAX_ENTRIES = 500

const props   = defineProps<{ connId: string }>()
const emit    = defineEmits<{ insert: [cmd: string] }>()
const visible = ref(false)
const query   = ref('')
const focusedIdx = ref(0)
const searchRef  = ref<HTMLInputElement>()
const listRef    = ref<HTMLDivElement>()

// ─── 全局历史（所有 session 共享） ───────────────────────────────────────────

function loadHistory(): string[] {
  try { return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]') }
  catch { return [] }
}

function saveHistory(h: string[]) {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(h)) }
  catch {}
}

const history = ref<string[]>(loadHistory())

/** 外部调用：记录一条命令 */
function record(cmd: string) {
  const trimmed = cmd.trim()
  if (!trimmed) return
  const h = history.value.filter(c => c !== trimmed)
  h.unshift(trimmed)
  if (h.length > MAX_ENTRIES) h.length = MAX_ENTRIES
  history.value = h
  saveHistory(h)
}

function deleteOne(cmd: string) {
  history.value = history.value.filter(c => c !== cmd)
  saveHistory(history.value)
}

function clearAll() {
  history.value = []
  saveHistory([])
}

// ─── 过滤 ─────────────────────────────────────────────────────────────────────

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return history.value
  return history.value.filter(c => c.toLowerCase().includes(q))
})

watch(filtered, () => { focusedIdx.value = 0 })

// ─── 键盘导航 ─────────────────────────────────────────────────────────────────

function moveFocus(delta: number) {
  const n = filtered.value.length
  if (!n) return
  focusedIdx.value = (focusedIdx.value + delta + n) % n
  nextTick(() => {
    const el = listRef.value?.querySelectorAll<HTMLDivElement>('.hist-item')[focusedIdx.value]
    el?.scrollIntoView({ block: 'nearest' })
  })
}

function insertFocused() {
  const cmd = filtered.value[focusedIdx.value]
  if (cmd) insert(cmd)
}

function insert(cmd: string) {
  emit('insert', cmd)
  close()
}

// ─── 打开 / 关闭 ─────────────────────────────────────────────────────────────

function open() {
  history.value = loadHistory()   // 跨 session 刷新
  visible.value = true
  query.value   = ''
  focusedIdx.value = 0
  nextTick(() => searchRef.value?.focus())
}

function close() {
  visible.value = false
}

defineExpose({ open, close, record })
</script>

<style scoped>
.hist-panel {
  position: absolute;
  bottom: 32px;          /* 悬浮在终端右下方 */
  right: 12px;
  width: 420px;
  max-height: 340px;
  display: flex;
  flex-direction: column;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 12px 40px rgba(0,0,0,0.55);
  z-index: 120;
  overflow: hidden;
}

.hist-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.hist-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
}

.hist-header-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.hist-clear-btn {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text-muted);
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: color 0.1s, border-color 0.1s;
}
.hist-clear-btn:hover { color: var(--danger); border-color: var(--danger); }

.hist-close-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 3px;
}
.hist-close-btn:hover { color: var(--text-primary); background: rgba(255,255,255,0.07); }

.hist-search-row {
  padding: 6px 8px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.hist-search {
  width: 100%;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 4px 8px;
  outline: none;
}
.hist-search:focus { border-color: var(--accent); }

.hist-list {
  overflow-y: auto;
  flex: 1;
  padding: 4px 0;
}

.hist-item {
  display: flex;
  align-items: center;
  padding: 5px 10px;
  cursor: pointer;
  transition: background 0.07s;
  gap: 6px;
}
.hist-item:hover, .hist-item.focused { background: rgba(255,255,255,0.06); }

.hist-cmd {
  flex: 1;
  font-family: 'Cascadia Code', Consolas, monospace;
  font-size: 12px;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.hist-del-btn {
  flex-shrink: 0;
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 10px;
  cursor: pointer;
  padding: 1px 4px;
  border-radius: 3px;
  opacity: 0;
  transition: opacity 0.1s, color 0.1s;
}
.hist-item:hover .hist-del-btn, .hist-item.focused .hist-del-btn { opacity: 1; }
.hist-del-btn:hover { color: var(--danger); }

.hist-empty {
  padding: 16px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
}

.hist-slide-enter-active, .hist-slide-leave-active { transition: opacity 0.15s, transform 0.15s; }
.hist-slide-enter-from, .hist-slide-leave-to { opacity: 0; transform: translateY(8px); }
</style>
