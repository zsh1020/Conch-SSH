<template>
  <div class="ltd-root">
    <!-- 头部：标题 + 标签栏 -->
    <div class="ltd-tabs">
      <span class="ltd-title"><i class="ph-fill ph-terminal-window"></i>本地终端</span>
      <span class="ltd-divider"></span>

      <div
        v-for="tab in tabs"
        :key="tab.id"
        class="ltd-tab"
        :class="{ active: tab.id === activeId }"
        :title="tab.title"
        @click="activate(tab.id)"
      >
        <span class="ltd-tab-dot"></span>
        <span class="ltd-tab-label">{{ tab.title }}</span>
        <button class="ltd-tab-close" title="关闭" @click.stop="closeTab(tab.id)"><i class="ph ph-x"></i></button>
      </div>

      <!-- 新建按钮 + shell 菜单 -->
      <div class="ltd-new-wrap">
        <button class="ltd-new" title="新建本地终端" @click="shellMenuOpen = !shellMenuOpen"><i class="ph ph-plus"></i></button>
        <template v-if="shellMenuOpen">
          <div class="ltd-menu-overlay" @click="shellMenuOpen = false" />
          <div class="ltd-menu">
            <div class="ltd-menu-item" @click="addTerminal('powershell', 'PowerShell')">
              <i class="ph ph-terminal-window ltd-menu-icon"></i>PowerShell
            </div>
            <div class="ltd-menu-item" @click="addTerminal('cmd', 'cmd')">
              <i class="ph ph-terminal ltd-menu-icon"></i>cmd
            </div>
            <div
              v-if="customLabel"
              class="ltd-menu-item"
              @click="addTerminal('custom', customLabel)"
            >
              <i class="ph ph-star ltd-menu-icon"></i>{{ customLabel }}
            </div>
          </div>
        </template>
      </div>

      <div class="ltd-spacer" />

      <button class="ltd-collapse" title="收起 (Ctrl+`)" @click="$emit('close')"><i class="ph ph-caret-down"></i></button>
    </div>

    <!-- 终端内容区 -->
    <div class="ltd-body">
      <div v-if="tabs.length === 0" class="ltd-empty">
        点击 + 新建本地终端
      </div>
      <template v-for="tab in tabs" :key="tab.id">
        <div v-show="tab.id === activeId" class="ltd-slot">
          <LocalTerminalPanel
            :ref="(el: any) => registerPanel(tab.id, el)"
            :id="tab.id"
            :shell="tab.shell"
          />
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import LocalTerminalPanel from './LocalTerminalPanel.vue'
import { useSettingsStore } from '../stores/settings'

const emit = defineEmits<{ close: [] }>()

const settingsStore = useSettingsStore()

interface LTTab { id: string; title: string; shell: string }

const tabs       = ref<LTTab[]>([])
const activeId   = ref('')
const shellMenuOpen = ref(false)

// 面板实例表（id → 暴露了 refit/focus 的组件实例）
const panels = new Map<string, { refit?: () => void; focus?: () => void }>()
function registerPanel(id: string, el: any) {
  if (el) panels.set(id, el)
  else panels.delete(id)
}

let seq = 1

// 自定义 shell 标签：取设置里 local_shell 的文件名，留空则不显示该菜单项
const customLabel = computed(() => {
  const p = settingsStore.settings.local_shell?.trim()
  if (!p) return ''
  return p.split(/[\\/]/).pop() || p
})

function activate(id: string) {
  activeId.value = id
  nextTick(() => {
    panels.get(id)?.refit?.()
    panels.get(id)?.focus?.()
  })
}

function addTerminal(shell: string, title: string) {
  const id = 'lt-' + (seq++)
  tabs.value.push({ id, title, shell })
  activeId.value = id
  shellMenuOpen.value = false
  nextTick(() => {
    panels.get(id)?.refit?.()
    panels.get(id)?.focus?.()
  })
}

function closeTab(id: string) {
  const idx = tabs.value.findIndex(t => t.id === id)
  if (idx === -1) return
  tabs.value.splice(idx, 1)   // 卸载 → 面板 onUnmounted 调 StopLocalTerm
  panels.delete(id)

  if (tabs.value.length === 0) {
    activeId.value = ''
    emit('close')             // 关掉最后一个 → 收起 dock
    return
  }
  if (activeId.value === id) {
    const next = tabs.value[idx] ?? tabs.value[idx - 1]
    activate(next.id)
  }
}

// ── 暴露给 App.vue ──
function refit() { if (activeId.value) panels.get(activeId.value)?.refit?.() }
function focus() { if (activeId.value) panels.get(activeId.value)?.focus?.() }
/** dock 被显示时调用：若当前没有任何终端，自动开一个默认 PowerShell */
function ensureTerminal() {
  if (tabs.value.length === 0) addTerminal('powershell', 'PowerShell')
}
defineExpose({ refit, focus, ensureTerminal })
</script>

<style scoped>
.ltd-root {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
  overflow: hidden;
  background: var(--term);
  font-family: 'IBM Plex Sans', 'Segoe UI', system-ui, sans-serif;
}

/* ── 头部 36px ── */
.ltd-tabs {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 36px;
  flex-shrink: 0;
  background: var(--panel-2);
  border-bottom: 1px solid var(--line);
  padding: 0 4px 0 12px;
}

/* 标题 */
.ltd-title {
  display: flex; align-items: center; gap: 6px;
  font-size: 11.5px; font-weight: 600; color: var(--ts);
  flex-shrink: 0;
}
.ltd-title i { font-size: 14px; color: var(--accent); }
.ltd-divider { width: 1px; height: 16px; background: var(--line); flex-shrink: 0; }

/* 标签（药丸） */
.ltd-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 26px;
  padding: 0 8px 0 10px;
  max-width: 160px;
  border-radius: 7px;
  font-size: 12px;
  color: var(--ts);
  cursor: pointer;
  transition: color 0.15s, background 0.15s;
  user-select: none;
  flex-shrink: 0;
}
.ltd-tab:hover { color: var(--tp); background: var(--hover); }
.ltd-tab.active { color: var(--tp); background: var(--term); box-shadow: 0 0 0 1px var(--accent-soft); }
.ltd-tab-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--tm); flex-shrink: 0; }
.ltd-tab.active .ltd-tab-dot { background: var(--accent); box-shadow: 0 0 5px var(--accent); }
.ltd-tab-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ltd-tab-close {
  background: transparent; border: none; color: var(--tm);
  font-size: 11px; padding: 0; cursor: pointer; line-height: 1;
  flex-shrink: 0; display: flex; align-items: center;
}
.ltd-tab-close:hover { color: var(--red); }

/* ── 新建 ── */
.ltd-new-wrap { position: relative; display: flex; align-items: center; }
.ltd-new {
  display: flex; align-items: center; justify-content: center;
  background: transparent; border: none; color: var(--ts);
  font-size: 15px; padding: 0; width: 28px; height: 28px;
  border-radius: 7px; cursor: pointer; transition: color 0.15s, background 0.15s;
}
.ltd-new:hover { color: var(--accent); background: var(--hover); }

.ltd-menu-overlay { position: fixed; inset: 0; z-index: 199; }
.ltd-menu {
  position: absolute; top: 32px; left: 0; z-index: 200;
  min-width: 150px;
  background: var(--panel); border: 1px solid var(--line);
  border-radius: 7px; padding: 4px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.5);
}
.ltd-menu-item {
  display: flex; align-items: center; gap: 8px;
  padding: 7px 10px; font-size: 12px; color: var(--tp);
  border-radius: 4px; cursor: pointer; white-space: nowrap;
}
.ltd-menu-item:hover { background: var(--hover); color: var(--accent); }
.ltd-menu-icon { font-size: 13px; width: 15px; text-align: center; color: var(--ts); }

.ltd-spacer { flex: 1; }

.ltd-collapse {
  display: flex; align-items: center; justify-content: center;
  background: transparent; border: none; color: var(--ts);
  font-size: 14px; padding: 0; width: 28px; height: 28px;
  border-radius: 7px; cursor: pointer; transition: color 0.15s, background 0.15s;
}
.ltd-collapse:hover { color: var(--tp); background: var(--hover); }

/* ── 内容区 ── */
.ltd-body { position: relative; flex: 1; overflow: hidden; background: var(--term); }
.ltd-slot { position: absolute; inset: 0; }
.ltd-empty {
  display: flex; align-items: center; justify-content: center;
  height: 100%; font-size: 13px; color: var(--tm);
}
</style>
