<template>
  <Teleport to="body">
    <Transition name="cp-fade">
      <div v-if="visible" class="cp-overlay" @mousedown.self="close">
        <div class="cp-box">
          <div class="cp-input-row">
            <span class="cp-icon">⌘</span>
            <input
              ref="inputRef"
              v-model="query"
              class="cp-input"
              placeholder="搜索会话、片段、配色…"
              autocomplete="off"
              spellcheck="false"
              @keydown.esc.prevent="close"
              @keydown.up.prevent="moveUp"
              @keydown.down.prevent="moveDown"
              @keydown.enter.prevent="confirm"
            />
            <span class="cp-hint">Esc 关闭</span>
          </div>

          <div class="cp-list" ref="listRef">
            <div v-if="results.length === 0" class="cp-empty">无匹配结果</div>
            <div
              v-for="(item, i) in results"
              :key="item.id"
              class="cp-item"
              :class="{ 'cp-active': i === activeIdx }"
              @mouseenter="activeIdx = i"
              @click="runItem(item)"
            >
              <span class="cp-item-icon">{{ item.icon }}</span>
              <span class="cp-item-label" v-html="highlight(item.label, query)" />
              <span class="cp-item-category">{{ item.category }}</span>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useTabsStore } from '../stores/tabs'
import { useSessionsStore } from '../stores/sessions'
import { useSnippetsStore } from '../stores/snippets'
import { useSettingsStore } from '../stores/settings'
import { SendInput } from '../../wailsjs/go/main/App'

const emit = defineEmits<{
  connect: [sessionId: string]
}>()

const visible  = ref(false)
const query    = ref('')
const activeIdx = ref(0)
const inputRef = ref<HTMLInputElement>()
const listRef  = ref<HTMLElement>()

const tabsStore     = useTabsStore()
const sessionsStore = useSessionsStore()
const snippetsStore = useSnippetsStore()
const settingsStore = useSettingsStore()

// ── Command types ──────────────────────────────────────────────────────────────

interface CmdItem {
  id: string
  icon: string
  label: string
  category: string
  run: () => void
}

const COLOR_SCHEMES = [
  'github-dark', 'dracula', 'one-dark', 'monokai', 'solarized-dark',
  'nord', 'catppuccin', 'material', 'solarized-light', 'dark-charcoal',
  'carbon', 'pure-black',
]
const SCHEME_LABELS: Record<string, string> = {
  'github-dark': 'GitHub Dark', 'dracula': 'Dracula', 'one-dark': 'One Dark',
  'monokai': 'Monokai', 'solarized-dark': 'Solarized Dark', 'nord': 'Nord',
  'catppuccin': 'Catppuccin Mocha', 'material': 'Material Dark',
  'solarized-light': 'Solarized Light', 'dark-charcoal': 'Dark Charcoal',
  'carbon': 'Carbon', 'pure-black': 'Pure Black',
}

const allItems = computed<CmdItem[]>(() => {
  const items: CmdItem[] = []

  // Sessions → open/connect
  for (const s of sessionsStore.sessions) {
    items.push({
      id: 'session:' + s.id,
      icon: '⌨',
      label: s.name + (s.host ? '  ' + s.host : ''),
      category: '会话',
      run() { emit('connect', s.id); close() },
    })
  }

  // Active-tab snippets → send to terminal
  const activeConnId = tabsStore.activeConnId
  for (const sn of snippetsStore.snippets) {
    items.push({
      id: 'snippet:' + sn.id,
      icon: '📋',
      label: sn.name + (sn.command ? '  ' + sn.command : ''),
      category: '片段',
      run() {
        if (activeConnId)
          SendInput(activeConnId, sn.command + '\n').catch(() => {})
        close()
      },
    })
  }

  // Color schemes
  for (const scheme of COLOR_SCHEMES) {
    items.push({
      id: 'scheme:' + scheme,
      icon: '🎨',
      label: SCHEME_LABELS[scheme] ?? scheme,
      category: '配色',
      run() {
        // settings store 内部 watch color_scheme 会自动调 applyColorScheme
        settingsStore.save({ ...settingsStore.settings, color_scheme: scheme })
        close()
      },
    })
  }

  return items
})

// ── Filtering (fuzzy) ─────────────────────────────────────────────────────────

function fuzzyMatch(text: string, q: string): boolean {
  if (!q) return true
  const t = text.toLowerCase(), s = q.toLowerCase()
  let j = 0
  for (let i = 0; i < t.length && j < s.length; i++) {
    if (t[i] === s[j]) j++
  }
  return j === s.length
}

const results = computed(() => {
  const q = query.value.trim()
  return allItems.value.filter(item => fuzzyMatch(item.label, q)).slice(0, 20)
})

watch(results, () => { activeIdx.value = 0 })

// ── Navigation ────────────────────────────────────────────────────────────────

function moveUp()   { activeIdx.value = Math.max(0, activeIdx.value - 1); scrollActive() }
function moveDown() { activeIdx.value = Math.min(results.value.length - 1, activeIdx.value + 1); scrollActive() }

function scrollActive() {
  nextTick(() => {
    const el = listRef.value?.children[activeIdx.value] as HTMLElement | undefined
    el?.scrollIntoView({ block: 'nearest' })
  })
}

function confirm() {
  const item = results.value[activeIdx.value]
  if (item) runItem(item)
}

function runItem(item: CmdItem) { item.run() }

// ── Highlight query in label ──────────────────────────────────────────────────

function highlight(text: string, q: string): string {
  if (!q) return escHtml(text)
  const t = text, s = q.toLowerCase()
  let res = '', i = 0, j = 0
  while (i < t.length) {
    if (j < s.length && t[i].toLowerCase() === s[j]) {
      res += `<mark>${escHtml(t[i])}</mark>`; j++
    } else {
      res += escHtml(t[i])
    }
    i++
  }
  return res
}

function escHtml(s: string) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
}

// ── Public API ────────────────────────────────────────────────────────────────

function open() {
  visible.value  = true
  query.value    = ''
  activeIdx.value = 0
  nextTick(() => inputRef.value?.focus())
}

function close() { visible.value = false }

defineExpose({ open, close })
</script>

<style scoped>
.cp-overlay {
  position: fixed;
  inset: 0;
  z-index: 2000;
  background: rgba(0,0,0,0.45);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 80px;
  backdrop-filter: blur(2px);
}

.cp-box {
  width: 560px;
  max-width: 90vw;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: 0 20px 60px rgba(0,0,0,0.7);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.cp-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}

.cp-icon { font-size: 16px; opacity: 0.5; flex-shrink: 0; }

.cp-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  color: var(--text-primary);
  font-size: 14px;
  font-family: inherit;
}

.cp-hint {
  font-size: 11px;
  color: var(--text-muted);
  flex-shrink: 0;
  opacity: 0.6;
}

.cp-list {
  max-height: 360px;
  overflow-y: auto;
  scrollbar-width: thin;
}

.cp-empty {
  padding: 20px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
}

.cp-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  cursor: pointer;
  transition: background 0.08s;
}

.cp-item:hover, .cp-active { background: rgba(88,166,255,0.1); }

.cp-item-icon  { flex-shrink: 0; font-size: 14px; width: 20px; text-align: center; }
.cp-item-label {
  flex: 1;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cp-item-label :deep(mark) {
  background: none;
  color: var(--accent);
  font-weight: 700;
}
.cp-item-category {
  flex-shrink: 0;
  font-size: 10px;
  color: var(--text-muted);
  background: rgba(255,255,255,0.05);
  border-radius: 3px;
  padding: 1px 6px;
}

/* ── Transition ── */
.cp-fade-enter-active, .cp-fade-leave-active { transition: opacity 0.12s, transform 0.12s; }
.cp-fade-enter-from, .cp-fade-leave-to { opacity: 0; transform: translateY(-8px); }
</style>
