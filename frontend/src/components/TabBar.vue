<template>
  <div v-if="tabsStore.tabs.length > 0" class="tab-bar">

    <!-- 左滚动箭头 -->
    <button v-if="canScrollLeft" class="tab-scroll-btn" @click="scrollTabs(-1)">‹</button>

    <div
      ref="tabListRef"
      class="tab-list"
      @dragover.prevent
      @drop="onTabDrop"
      @scroll="updateScrollState"
      @wheel.prevent="onWheel"
    >
      <div
        v-for="(tab, idx) in tabsStore.tabs"
        :key="tab.connId"
        class="tab"
        :class="{
          active:      tab.connId === tabsStore.activeConnId,
          'drag-over': dragOverIdx === idx,
          'tab-closed':     tab.status === 'closed',
          'tab-connecting': tab.status === 'connecting',
        }"
        :title="tab.label"
        draggable="true"
        @click="tabsStore.activeConnId = tab.connId"
        @contextmenu.prevent.stop="onTabRightClick($event, tab)"
        @dragstart="onTabDragStart($event, idx)"
        @dragover.prevent="dragOverIdx = idx"
        @dragleave="dragOverIdx = -1"
      >
        <!-- 状态点 -->
        <span class="tab-dot" :style="tabDotStyle(tab)"></span>
        <span class="tab-label">{{ tab.label }}</span>
        <span
          v-if="tab.hasNewOutput && tab.connId !== tabsStore.activeConnId"
          class="tab-new-output"
          title="有新输出"
        ></span>
        <span v-if="idx < 9" class="tab-index" :class="{ 'idx-active': tab.connId === tabsStore.activeConnId }">{{ idx + 1 }}</span>
        <button class="tab-close" @click.stop="tabsStore.closeTab(tab.connId)"><i class="ph ph-x"></i></button>
        <span class="tab-underline"></span>
      </div>
      <button class="tab-new" title="新建连接 (Ctrl+T)" @click="$emit('new-tab')"><i class="ph ph-plus"></i></button>
    </div>

    <!-- 右滚动箭头 -->
    <button v-if="canScrollRight" class="tab-scroll-btn" @click="scrollTabs(1)">›</button>

    <!-- 右侧工具按钮组 -->
    <div v-if="tabsStore.activeConnId" class="tab-tools">
      <div class="snippet-wrap">
        <button class="tab-tool-btn" :class="{ active: showSnippet }" title="命令片段" @click="$emit('toggle-snippet')"><i class="ph ph-code"></i></button>
        <SnippetPanel
          v-if="showSnippet"
          :conn-id="tabsStore.activeConnId"
          @manage="$emit('manage-snippet')"
          @close="$emit('toggle-snippet')"
        />
      </div>
      <button class="tab-tool-btn" :class="{ active: broadcastActive }" title="批量广播" @click="$emit('toggle-broadcast')"><i class="ph ph-broadcast"></i></button>
      <button class="tab-tool-btn" :class="{ active: showMonitor }" title="服务器监控" @click="$emit('toggle-monitor')"><i class="ph ph-pulse"></i></button>
      <button class="tab-tool-btn" :class="{ active: showTunnel }" title="端口转发" @click="$emit('toggle-tunnel')"><i class="ph ph-link"></i></button>
      <button class="tab-tool-btn" :class="{ active: showLocalTerm }" title="本地终端 (Ctrl+`)" @click="$emit('toggle-local-term')"><i class="ph ph-terminal-window"></i></button>
    </div>
  </div>

  <!-- ── 标签右键菜单 ── -->
  <Teleport to="body">
    <div
      v-if="tabCtx.tab"
      class="tab-ctx-overlay"
      @click="tabCtx.tab = null"
      @contextmenu.prevent="tabCtx.tab = null"
    />
    <div
      v-if="tabCtx.tab"
      class="tab-ctx-menu"
      :style="{ top: tabCtx.y + 'px', left: tabCtx.x + 'px' }"
    >
      <div
        class="tab-ctx-item"
        :class="{ disabled: !tabCtx.tab.lastConnectReq }"
        @click="onCtxDuplicate"
      >复制</div>
      <div
        class="tab-ctx-item"
        :class="{ disabled: !tabCtx.tab.lastConnectReq }"
        @click="onCtxReconnect"
      >重新连接</div>
      <div class="tab-ctx-divider" />
      <div class="tab-ctx-item danger" @click="onCtxClose">关闭</div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useTabsStore, type Tab } from '../stores/tabs'
import SnippetPanel from './SnippetPanel.vue'

defineProps<{
  showSnippet:    boolean
  broadcastActive: boolean
  showMonitor:    boolean
  showTunnel:     boolean
  showLocalTerm:  boolean
}>()

const emit = defineEmits<{
  (e: 'toggle-snippet'):   void
  (e: 'manage-snippet'):   void
  (e: 'toggle-broadcast'): void
  (e: 'toggle-monitor'):   void
  (e: 'toggle-tunnel'):    void
  (e: 'toggle-local-term'): void
  (e: 'new-tab'): void
  (e: 'reconnect', connId: string): void
  (e: 'duplicate', connId: string): void
}>()

const tabsStore = useTabsStore()

// 标签识别色块：颜色为会话标识色（tab.color，由 sessionId 决定），
// 激活时发光；断线变灰、连接中闪烁由 CSS（.tab-closed / .tab-connecting）处理。
function tabDotStyle(tab: Tab) {
  const active = tab.connId === tabsStore.activeConnId
  return { background: tab.color, boxShadow: active ? `0 0 6px ${tab.color}` : 'none' }
}

// ─── 标签滚动 ─────────────────────────────────────────────────
const tabListRef    = ref<HTMLDivElement>()
const canScrollLeft  = ref(false)
const canScrollRight = ref(false)

function updateScrollState() {
  const el = tabListRef.value
  if (!el) return
  canScrollLeft.value  = el.scrollLeft > 0
  canScrollRight.value = el.scrollLeft + el.clientWidth < el.scrollWidth - 1
}

function scrollTabs(dir: -1 | 1) {
  tabListRef.value?.scrollBy({ left: dir * 160, behavior: 'smooth' })
}

function onWheel(e: WheelEvent) {
  tabListRef.value?.scrollBy({ left: e.deltaY || e.deltaX, behavior: 'auto' })
}

/** 将指定标签滚入可视区 */
async function scrollActiveIntoView() {
  await nextTick()
  const el = tabListRef.value
  if (!el) return
  const active = el.querySelector('.tab.active') as HTMLElement | null
  active?.scrollIntoView({ behavior: 'smooth', inline: 'nearest', block: 'nearest' })
  updateScrollState()
}

// 标签数量变化 / 切换激活 → 检查溢出、确保可见
watch(() => [tabsStore.tabs.length, tabsStore.activeConnId], scrollActiveIntoView)

// 容器宽度变化（窗口 resize）→ 重新评估
let ro: ResizeObserver | null = null
onMounted(() => {
  ro = new ResizeObserver(updateScrollState)
  if (tabListRef.value) ro.observe(tabListRef.value)
  updateScrollState()
})
onUnmounted(() => ro?.disconnect())

// ─── 标签拖拽排序 ─────────────────────────────────────────────
const dragFromIdx = ref(-1)
const dragOverIdx = ref(-1)

function onTabDragStart(e: DragEvent, idx: number) {
  dragFromIdx.value = idx
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(idx))
  }
}

function onTabDrop(e: DragEvent) {
  e.preventDefault()
  const to   = dragOverIdx.value
  const from = dragFromIdx.value
  if (from >= 0 && to >= 0 && from !== to) {
    tabsStore.reorderTab(from, to)
  }
  dragFromIdx.value = -1
  dragOverIdx.value = -1
}

// ─── 标签右键菜单 ─────────────────────────────────────────────

const tabCtx = ref<{ tab: Tab | null; x: number; y: number }>({ tab: null, x: 0, y: 0 })

function onTabRightClick(e: MouseEvent, tab: Tab) {
  tabsStore.activeConnId = tab.connId
  tabCtx.value = { tab, x: e.clientX, y: e.clientY }
  nextTick(() => {
    const el = document.querySelector<HTMLElement>('.tab-ctx-menu')
    if (!el) return
    const rect = el.getBoundingClientRect()
    let x = e.clientX, y = e.clientY
    if (x + rect.width  > window.innerWidth)  x = window.innerWidth  - rect.width  - 4
    if (y + rect.height > window.innerHeight) y = window.innerHeight - rect.height - 4
    tabCtx.value.x = Math.max(4, x)
    tabCtx.value.y = Math.max(4, y)
  })
}

function onCtxClose() {
  const connId = tabCtx.value.tab?.connId
  tabCtx.value.tab = null
  if (connId) tabsStore.closeTab(connId)
}

function onCtxReconnect() {
  const tab = tabCtx.value.tab
  tabCtx.value.tab = null
  if (tab?.lastConnectReq) emit('reconnect', tab.connId)
}

function onCtxDuplicate() {
  const tab = tabCtx.value.tab
  tabCtx.value.tab = null
  if (tab?.lastConnectReq) emit('duplicate', tab.connId)
}
</script>

<style>
.tab-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--panel-2);
  border-bottom: 1px solid var(--line);
  flex-shrink: 0;
  height: 42px;
  padding: 0 8px 0 10px;
  font-family: 'IBM Plex Sans', 'Segoe UI', system-ui, sans-serif;
}

.tab-list {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
  overflow-x: auto;
  height: 100%;
  scrollbar-width: none;
  scroll-behavior: smooth;
}
.tab-list::-webkit-scrollbar { display: none; }

/* 左右滚动箭头 */
.tab-scroll-btn {
  flex-shrink: 0;
  height: 28px;
  width: 22px;
  background: transparent;
  border: none;
  color: var(--tm);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  transition: color 0.15s, background 0.15s;
  user-select: none;
}
.tab-scroll-btn:hover { color: var(--tp); background: var(--hover); }

/* 标签（药丸） */
.tab {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 12.5px;
  color: var(--ts);
  border: 1px solid transparent;
  white-space: nowrap;
  max-width: 200px;
  position: relative;
  flex-shrink: 0;
  transition: color 0.15s, background 0.15s;
  user-select: none;
}
.tab:hover { background: var(--hover); }

.tab.active {
  background: var(--term);
  border-color: var(--line);
  box-shadow: 0 0 0 1px var(--accent-soft);
  color: var(--tp);
}

.tab.drag-over { box-shadow: inset 2px 0 0 var(--accent); }

/* 激活下划线 */
.tab-underline {
  position: absolute;
  left: 10px; right: 10px; bottom: -1px;
  height: 2px; border-radius: 2px;
  background: var(--accent);
  opacity: 0;
}
.tab.active .tab-underline { opacity: 1; }

/* 会话标识色块（颜色 = tab.color） */
.tab-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; transition: filter 0.15s; }
/* 断线时变灰 */
.tab-closed .tab-dot { filter: saturate(0) brightness(0.55); box-shadow: none !important; }
/* 连接中时闪烁 */
.tab-connecting .tab-dot { animation: tab-dot-pulse 1s infinite; }
@keyframes tab-dot-pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.3; } }

.tab-label { flex: 1; overflow: hidden; text-overflow: ellipsis; }
.tab.active .tab-label { font-weight: 500; }

/* 新输出提示点 */
.tab-new-output {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--green); flex-shrink: 0;
  animation: new-output-pulse 1.5s ease-in-out infinite;
}
@keyframes new-output-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%      { opacity: 0.4; transform: scale(0.7); }
}

.tab-index {
  font-size: 9px;
  color: var(--tm);
  background: rgba(255,255,255,0.06);
  border-radius: 4px;
  padding: 1px 5px;
  flex-shrink: 0;
}
.tab-index.idx-active { color: var(--accent); background: var(--accent-soft); }

.tab-close {
  background: transparent;
  border: none;
  padding: 0;
  font-size: 11px;
  color: var(--tm);
  cursor: pointer;
  flex-shrink: 0;
  line-height: 1;
  display: flex;
  align-items: center;
  transition: color 0.15s;
}
.tab.active .tab-close { color: var(--ts); }
.tab-close:hover { color: var(--red); }

/* 新建标签 + */
.tab-new {
  width: 28px; height: 28px; flex-shrink: 0;
  border-radius: 7px; background: transparent; border: none;
  color: var(--ts); cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  font-size: 15px; transition: color 0.15s, background 0.15s;
}
.tab-new:hover { background: var(--hover); color: var(--tp); }

/* ─── 右侧工具按钮组 ─── */
.tab-tools {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}
.tab-tool-btn {
  width: 30px; height: 30px;
  background: transparent;
  border: none;
  border-radius: 7px;
  color: var(--ts);
  font-size: 15px;
  cursor: pointer;
  transition: color 0.15s, background 0.15s;
  display: flex;
  align-items: center;
  justify-content: center;
}
.tab-tool-btn:hover { color: var(--accent); background: var(--hover); }
.tab-tool-btn.active { color: var(--accent); background: var(--accent-soft); }

.snippet-wrap {
  position: relative;
  height: 30px;
  display: flex;
  align-items: center;
}

/* ─── 标签右键菜单 ─── */
.tab-ctx-overlay { position: fixed; inset: 0; z-index: 399; }
.tab-ctx-menu {
  position: fixed;
  z-index: 400;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 7px;
  padding: 4px;
  min-width: 130px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.5);
  user-select: none;
  font-family: 'IBM Plex Sans', 'Segoe UI', sans-serif;
}
.tab-ctx-item {
  padding: 7px 12px;
  font-size: 12px;
  color: var(--tp);
  border-radius: 4px;
  cursor: pointer;
}
.tab-ctx-item:hover { background: var(--hover); }
.tab-ctx-item.disabled { color: var(--tm); cursor: not-allowed; pointer-events: none; }
.tab-ctx-item.danger { color: var(--red); }
.tab-ctx-item.danger:hover { background: rgba(239,109,109,0.12); }
.tab-ctx-divider { height: 1px; background: var(--line); margin: 3px 4px; }
</style>
