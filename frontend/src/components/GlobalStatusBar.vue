<template>
  <div class="gsb">
    <!-- 左：连接状态 -->
    <span class="gsb-item">
      <span class="gsb-dot" :class="{ on: connected }"></span>{{ connected ? '已连接' : '未连接' }}
    </span>
    <template v-if="connected">
      <span class="gsb-sep">·</span>
      <span class="gsb-item"><i class="ph ph-desktop-tower"></i>{{ label }}</span>
      <span v-if="user && host" class="gsb-sep">·</span>
      <span v-if="user && host" class="gsb-item mono">{{ user }}@{{ host }}</span>
      <span class="gsb-sep">·</span>
      <span class="gsb-item"><i class="ph-fill ph-shield-check gsb-shield"></i>SSH-2 · AES-256</span>
    </template>

    <div class="gsb-spring"></div>

    <!-- 右：编码 · 时间 · 管理 -->
    <span class="gsb-item mono"><i class="ph ph-text-aa"></i>UTF-8</span>
    <span class="gsb-sep">·</span>
    <span class="gsb-item mono">{{ time }}</span>
    <button class="gsb-manage" title="设置" @click="$emit('manage')"><i class="ph ph-gear-six"></i>管理</button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useTabsStore } from '../stores/tabs'
import { useSessionsStore } from '../stores/sessions'

defineEmits<{ (e: 'manage'): void }>()

const tabsStore     = useTabsStore()
const sessionsStore = useSessionsStore()

const activeTab = computed(() => tabsStore.tabs.find(t => t.connId === tabsStore.activeConnId))
const connected = computed(() => activeTab.value?.status === 'connected')
const sess = computed(() => {
  const t = activeTab.value
  return t?.sessionId ? sessionsStore.sessions.find(s => s.id === t.sessionId) : null
})
const host  = computed(() => sess.value?.host || activeTab.value?.lastConnectReq?.host || '')
const user  = computed(() => sess.value?.username || activeTab.value?.lastConnectReq?.username || '')
const label = computed(() => activeTab.value?.label || host.value)

function nowStr() { return new Date().toLocaleTimeString('zh-CN', { hour12: false }) }
const time = ref(nowStr())
let timer: ReturnType<typeof setInterval> | null = null
onMounted(() => { timer = setInterval(() => { time.value = nowStr() }, 1000) })
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<style scoped>
.gsb {
  display: flex;
  align-items: center;
  gap: 7px;
  height: 28px;
  flex: 0 0 28px;
  padding: 0 8px 0 12px;
  background: linear-gradient(180deg, var(--panel-2), var(--term));
  border-top: 1px solid var(--line);
  font-family: 'IBM Plex Sans', 'Segoe UI', system-ui, sans-serif;
  font-size: 11px;
  color: var(--tm);
  user-select: none;
  overflow: hidden;
}
.mono { font-family: 'IBM Plex Mono', 'Roboto Mono', monospace; }

.gsb-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  white-space: nowrap;
  min-width: 0;
}
.gsb-item i { font-size: 12px; }
.gsb-sep { color: var(--line); flex-shrink: 0; }

.gsb-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--tm); flex-shrink: 0;
}
.gsb-dot.on { background: var(--green); box-shadow: 0 0 5px var(--green); }
.gsb-shield { color: var(--green); }

.gsb-spring { flex: 1; min-width: 8px; }

.gsb-manage {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: 4px;
  height: 20px;
  padding: 0 9px;
  border-radius: 6px;
  border: none;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 11px;
  font-family: inherit;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.15s;
}
.gsb-manage i { font-size: 12px; }
.gsb-manage:hover { background: color-mix(in srgb, var(--accent) 22%, transparent); }
</style>
