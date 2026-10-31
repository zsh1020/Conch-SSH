<template>
  <aside class="sidebar" :class="{ collapsed }">
    <!-- 折叠态：仅显示状态点 -->
    <template v-if="collapsed">
      <div class="collapsed-sessions">
        <button
          v-for="sess in allSessions"
          :key="sess.id"
          class="collapsed-item"
          :title="`${sess.name || sess.host}\n${sess.username}@${sess.host}:${sess.port || 22}`"
          @click="$emit('connect', sess)"
        >
          <span class="dot" :style="dotStyle(sess)"></span>
        </button>
      </div>
    </template>

    <template v-else>
      <!-- ── 会话头 38px ── -->
      <div class="sess-header">
        <i class="ph ph-tree-view head-icon"></i>
        <span class="head-title">会话</span>
        <span v-if="onlineCount > 0" class="online-pill">
          <span class="online-dot"></span>{{ onlineCount }} 在线
        </span>
        <div class="spring"></div>
        <button class="hbtn" title="新建会话" @click="$emit('new-session')"><i class="ph ph-plus"></i></button>
        <button class="hbtn" title="折叠 / 展开全部" @click="toggleCollapseAll"><i class="ph ph-arrows-in-line-vertical"></i></button>
      </div>

      <!-- ── 搜索 + 分段筛选 ── -->
      <div class="sess-filters">
        <div class="search-box">
          <i class="ph ph-magnifying-glass search-icon"></i>
          <input v-model="searchQuery" class="search-input" placeholder="搜索主机 / IP / 标签…" />
          <span v-if="!searchQuery" class="kbd mono">⌘K</span>
          <button v-else class="clear-btn" @click="searchQuery = ''"><i class="ph ph-x"></i></button>
        </div>
        <div class="segmented">
          <button class="seg" :class="{ active: sessionFilter === 'all' }" @click="sessionFilter = 'all'">
            全部 <span class="seg-count">{{ totalCount }}</span>
          </button>
          <button class="seg" :class="{ active: sessionFilter === 'connected' }" @click="sessionFilter = 'connected'">
            <span class="seg-green-dot"></span>已连接 <span class="seg-count">{{ connectedCount }}</span>
          </button>
          <button class="seg" :class="{ active: sessionFilter === 'favorite' }" @click="sessionFilter = 'favorite'">
            <i class="ph-fill ph-star seg-star"></i>收藏 <span class="seg-count">{{ favoriteCount }}</span>
          </button>
        </div>
      </div>

      <!-- ── 会话树 ── -->
      <div class="session-tree scroll">
        <div v-if="filteredGroups.length === 0 && !showRecent" class="empty-tip">
          {{ searchQuery || sessionFilter !== 'all' ? '无匹配会话' : '暂无会话，点击 ＋ 新建' }}
        </div>

        <!-- ── 最近使用（置顶）── -->
        <div v-if="showRecent" class="group-block">
          <div class="group-header" @click="toggleGroup('__recent__')">
            <i class="ph-bold group-caret" :class="isExpanded('__recent__') ? 'ph-caret-down' : 'ph-caret-right'"></i>
            <i class="ph-fill ph-clock group-folder" style="color: var(--accent)"></i>
            <span class="group-name">最近</span>
            <span class="group-count mono">
              <span :style="{ color: recentConnected > 0 ? 'var(--green)' : 'var(--tm)', fontWeight: 600 }">{{ recentConnected }}</span><span class="group-count-total">/{{ recentList.length }}</span>
            </span>
          </div>
          <div v-if="isExpanded('__recent__')" class="session-list">
            <div
              v-for="sess in recentList"
              :key="'recent-' + sess.id"
              class="session-item"
              :class="{ active: selectedId === sess.id }"
              :title="`${sess.name || sess.host}\n${sess.username}@${sess.host}:${sess.port || 22}`"
              @click="selectSession(sess)"
              @dblclick="$emit('connect', sess)"
              @contextmenu.prevent.stop="openCtxMenu($event, sess)"
            >
              <span class="sel-bar"></span>
              <span class="dot" :style="dotStyle(sess)"></span>
              <span class="sess-main">
                <span class="sess-name" :style="{ color: nameColor(sess) }">{{ sess.name || sess.host }}</span>
                <i v-if="sess.starred" class="ph-fill ph-star sess-fav"></i>
              </span>
              <span v-if="pingOf(sess)" class="ping mono" :style="pingStyle(sess)">{{ pingOf(sess) }}</span>
              <button class="sess-more" title="更多" @click.stop="openCtxMenu($event, sess)"><i class="ph ph-dots-three-vertical"></i></button>
            </div>
          </div>
        </div>

        <div
          v-for="group in filteredGroups"
          :key="group.name"
          class="group-block"
          :class="{ 'drag-over': dragOverGroup === group.name }"
          draggable="true"
          @dragstart="onGroupDragStart($event, group.name)"
          @dragover.prevent="onGroupDragOver($event, group.name)"
          @dragleave="dragOverGroup = null"
          @drop="onGroupDrop(group.name)"
        >
          <!-- 分组头 32px -->
          <div
            class="group-header"
            @click="toggleGroup(group.name)"
            @contextmenu.prevent.stop="openGroupCtxMenu($event, group.name)"
          >
            <i class="ph-bold group-caret" :class="isExpanded(group.name) ? 'ph-caret-down' : 'ph-caret-right'"></i>
            <i class="ph-fill ph-folder-simple group-folder" :style="{ color: groupColor(group.name) }"></i>
            <span class="group-name">{{ group.name }}</span>
            <span class="group-count mono">
              <span :style="{ color: group.connected > 0 ? 'var(--green)' : 'var(--tm)', fontWeight: 600 }">{{ group.connected }}</span><span class="group-count-total">/{{ group.total }}</span>
            </span>
          </div>

          <!-- 子会话（仅展开时） -->
          <div v-if="isExpanded(group.name)" class="session-list">
            <div
              v-for="sess in group.sessions"
              :key="sess.id"
              class="session-item"
              :class="{
                active:          selectedId === sess.id,
                dragging:        dragFromSessId === sess.id,
                'drag-over-sess':dragOverSessId === sess.id && dragFromSessId !== sess.id,
              }"
              :title="`${sess.name || sess.host}\n${sess.username}@${sess.host}:${sess.port || 22}`"
              draggable="true"
              @dragstart.stop="onSessDragStart($event, sess, group.name)"
              @dragover.prevent="onSessDragOver($event, sess, group.name)"
              @dragleave="onSessDragLeave"
              @drop.stop="onSessDrop($event, sess, group.name)"
              @click="selectSession(sess)"
              @dblclick="$emit('connect', sess)"
              @contextmenu.prevent.stop="openCtxMenu($event, sess)"
            >
              <span class="sel-bar"></span>
              <span class="dot" :style="dotStyle(sess)"></span>
              <span class="sess-main">
                <span class="sess-name" :style="{ color: nameColor(sess) }">{{ sess.name || sess.host }}</span>
                <i v-if="sess.starred" class="ph-fill ph-star sess-fav"></i>
              </span>
              <span v-if="pingOf(sess)" class="ping mono" :style="pingStyle(sess)">{{ pingOf(sess) }}</span>
              <button class="sess-more" title="更多" @click.stop="openCtxMenu($event, sess)"><i class="ph ph-dots-three-vertical"></i></button>
            </div>
          </div>
        </div>

        <!-- 拖拽末尾放置区 -->
        <div
          class="group-drop-zone"
          :class="{ 'drag-over': dragOverGroup === '__end__' }"
          @dragover.prevent="dragOverGroup = '__end__'"
          @dragleave="dragOverGroup = null"
          @drop="onGroupDrop('__end__')"
        />

        <!-- 新建会话虚线按钮 -->
        <button class="new-session-btn" @click="$emit('new-session')">
          <i class="ph ph-plus"></i>新建会话
        </button>
      </div>
    </template>

    <!-- 会话右键菜单 -->
    <div
      v-if="ctxMenu.visible"
      class="ctx-menu sess-ctx-menu"
      :style="{ top: ctxMenu.y + 'px', left: ctxMenu.x + 'px' }"
      @click.stop
    >
      <div class="ctx-item" @click="onCtxConnect">连接</div>
      <div class="ctx-item" @click="onCtxEdit">编辑</div>
      <div class="ctx-item" @click="onCtxStar">{{ ctxMenu.sess?.starred ? '取消收藏 ★' : '收藏 ☆' }}</div>
      <div class="ctx-item" @click="onCtxDuplicate">复制会话</div>
      <div class="ctx-divider" />
      <div class="ctx-item danger" @click="onCtxDelete">删除</div>
    </div>
    <div v-if="ctxMenu.visible" class="ctx-overlay" @click="ctxMenu.visible = false" @contextmenu.prevent="ctxMenu.visible = false" />

    <!-- 分组右键菜单 -->
    <div
      v-if="groupCtxMenu.visible"
      class="ctx-menu group-ctx-menu"
      :style="{ top: groupCtxMenu.y + 'px', left: groupCtxMenu.x + 'px' }"
      @click.stop
    >
      <div class="ctx-item" @click="onGroupCtxRename">重命名</div>
      <div class="ctx-divider" />
      <div class="ctx-item danger" @click="onGroupCtxDelete">删除分组</div>
    </div>
    <div v-if="groupCtxMenu.visible" class="ctx-overlay" @click="groupCtxMenu.visible = false" @contextmenu.prevent="groupCtxMenu.visible = false" />
  </aside>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { useSessionsStore, type Session } from '../stores/sessions'
import { useTabsStore, pickColor } from '../stores/tabs'
import { useDialogStore } from '../stores/dialogs'
import { useSettingsStore } from '../stores/settings'

const props = defineProps<{
  collapsed: boolean
  activeSessionId?: string
}>()

const emit = defineEmits<{
  connect: [sess: Session]
  edit: [sess: Session]
  'new-session': []
  'toggle-collapse': []
}>()

const sessionsStore = useSessionsStore()
const tabsStore     = useTabsStore()
const dlg           = useDialogStore()
const settingsStore = useSettingsStore()

const searchQuery    = ref('')
const sessionFilter  = ref<'all' | 'connected' | 'favorite'>('all')

// ── 展开状态（持久化） ──────────────────────────────────────────
const EXPANDED_KEY = 'gssh-expanded-groups'
function loadExpandedGroups(): Set<string> {
  try {
    const raw = localStorage.getItem(EXPANDED_KEY)
    if (raw) return new Set(JSON.parse(raw) as string[])
  } catch {}
  return new Set<string>(['__recent__', '本地', 'ERP'])
}
function saveExpandedGroups(set: Set<string>) {
  try { localStorage.setItem(EXPANDED_KEY, JSON.stringify([...set])) } catch {}
}
const expandedGroups = ref<Set<string>>(loadExpandedGroups())
function isExpanded(name: string) { return expandedGroups.value.has(name) }

// ── 拖拽状态 ────────────────────────────────────────────────────
const dragFromGroup   = ref('')
const dragOverGroup   = ref<string | null>(null)
const dragFromSessId  = ref('')
const dragFromSessGrp = ref('')
const dragOverSessId  = ref<string | null>(null)

const allSessions = computed(() => sessionsStore.sessions)

// ── 连接状态 ────────────────────────────────────────────────────
// 有已连接标签页的会话 id 集合
const connectedSessionIds = computed(() => {
  const s = new Set<string>()
  for (const t of tabsStore.tabs) {
    if (t.status === 'connected' && t.sessionId) s.add(t.sessionId)
  }
  return s
})
// 当前激活标签页对应的会话
const activeTabSessionId = computed(() => {
  const t = tabsStore.tabs.find(t => t.connId === tabsStore.activeConnId)
  return t?.status === 'connected' ? (t.sessionId ?? '') : ''
})

type SessStatus = 'active' | 'connected' | 'idle'
function statusOf(sess: Session): SessStatus {
  if (sess.id && sess.id === activeTabSessionId.value) return 'active'
  if (connectedSessionIds.value.has(sess.id)) return 'connected'
  return 'idle'
}

// ── 最近使用（置顶特殊分组，仅 filter=all 且无搜索时显示）──
const recentList = computed(() => sessionsStore.recentSessions)
const showRecent = computed(() =>
  settingsStore.settings.show_recent &&
  recentList.value.length > 0 &&
  sessionFilter.value === 'all' &&
  !searchQuery.value.trim()
)
const recentConnected = computed(() =>
  recentList.value.filter(s => connectedSessionIds.value.has(s.id)).length
)

const onlineCount    = computed(() => connectedSessionIds.value.size)
const totalCount     = computed(() => allSessions.value.length)
const connectedCount = computed(() => onlineCount.value)
const favoriteCount  = computed(() => allSessions.value.filter(s => s.starred).length)

// ── 分组环境色 ──────────────────────────────────────────────────
const ENV_COLORS: Record<string, string> = {
  '本地': '#5aa2fb', 'ERP': '#34c8d4', 'ZG_IS': '#a98bf5',
  '默认': '#7c8a99', '总部': '#ecb44a', '测试': '#46cf8b', '测试-K8S': '#ee7ab0',
}
const FALLBACK_COLORS = ['#5aa2fb', '#34c8d4', '#a98bf5', '#ecb44a', '#46cf8b', '#ee7ab0', '#7c8a99']
function groupColor(name: string): string {
  if (ENV_COLORS[name]) return ENV_COLORS[name]
  let h = 0
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0
  return FALLBACK_COLORS[h % FALLBACK_COLORS.length]
}

// ── 会话行样式 ──────────────────────────────────────────────────
function dotStyle(sess: Session) {
  const st = statusOf(sess)
  // 会话识别色块：颜色由 sessionId 决定（每会话专属色）；
  // 已连接/激活时发光、不透明，空闲时略微变淡。
  const color = pickColor(sess.id)
  return {
    background: color,
    boxShadow: st === 'idle' ? 'none' : `0 0 6px ${color}`,
    opacity: st === 'idle' ? 0.5 : 1,
  }
}
function nameColor(sess: Session) {
  const st = statusOf(sess)
  return st === 'active' ? 'var(--tp)' : st === 'connected' ? '#c3ccd6' : 'var(--ts)'
}
// 延迟：暂无测量数据，字段就绪后自动显示（<60 绿 / <150 琥珀 / ≥150 红）
function pingOf(sess: Session): string {
  const p = (sess as any).ping
  return p ? String(p) : ''
}
function pingStyle(sess: Session) {
  const n = parseInt(pingOf(sess), 10)
  const [c, bg] = n < 60
    ? ['var(--green)', 'rgba(70,207,139,0.1)']
    : n < 150
      ? ['var(--amber)', 'rgba(236,180,74,0.1)']
      : ['var(--red)', 'rgba(239,109,109,0.1)']
  return { color: c, background: bg }
}

// ── 分组 + 过滤 ─────────────────────────────────────────────────
const filteredGroups = computed(() => {
  const q = searchQuery.value.toLowerCase()
  const filter = sessionFilter.value
  const map = sessionsStore.sessionsByGroup
  const groupOrder = sessionsStore.groups

  const passFilter = (s: Session) => {
    if (filter === 'connected' && !connectedSessionIds.value.has(s.id)) return false
    if (filter === 'favorite' && !s.starred) return false
    if (q) {
      return (s.name || '').toLowerCase().includes(q) ||
             s.host.toLowerCase().includes(q) ||
             s.username.toLowerCase().includes(q) ||
             (s.tags || []).some(t => t.toLowerCase().includes(q))
    }
    return true
  }

  return Object.entries(map)
    .map(([name, sessions]) => {
      const total = sessions.length
      const connected = sessions.filter(s => connectedSessionIds.value.has(s.id)).length
      return { name, total, connected, sessions: sessions.filter(passFilter) }
    })
    .filter(g => g.sessions.length > 0)
    .sort((a, b) => {
      const ai = groupOrder.indexOf(a.name), bi = groupOrder.indexOf(b.name)
      if (ai === -1 && bi === -1) return 0
      if (ai === -1) return 1
      if (bi === -1) return -1
      return ai - bi
    })
})

function toggleGroup(name: string) {
  if (expandedGroups.value.has(name)) expandedGroups.value.delete(name)
  else expandedGroups.value.add(name)
  expandedGroups.value = new Set(expandedGroups.value)
  saveExpandedGroups(expandedGroups.value)
}
// 折叠/展开全部：有任意展开 → 全部折叠；否则 → 全部展开
function toggleCollapseAll() {
  const names = [...Object.keys(sessionsStore.sessionsByGroup)]
  if (showRecent.value) names.push('__recent__')
  const anyExpanded = names.some(n => expandedGroups.value.has(n))
  expandedGroups.value = anyExpanded ? new Set() : new Set(names)
  saveExpandedGroups(expandedGroups.value)
}

// ── 选中态 ──────────────────────────────────────────────────────
const selectedId = ref(props.activeSessionId ?? '')
function selectSession(sess: Session) { selectedId.value = sess.id }
// 初始高亮当前激活会话
watch(activeTabSessionId, (id) => { if (id && !selectedId.value) selectedId.value = id }, { immediate: true })

// ─── 分组拖拽排序 ───────────────────────────────────────────────
function onGroupDragStart(_e: DragEvent, name: string) { dragFromGroup.value = name }
function onGroupDragOver(_e: DragEvent, name: string) {
  if (dragFromSessId.value) return
  if (name !== dragFromGroup.value) dragOverGroup.value = name
}
async function onGroupDrop(targetGroup: string) {
  if (dragFromSessId.value) return
  dragOverGroup.value = null
  const from = dragFromGroup.value
  if (!from || from === targetGroup) return
  const currentOrder = filteredGroups.value.map(g => g.name)
  const fromIdx = currentOrder.indexOf(from)
  if (fromIdx === -1) return
  if (targetGroup === '__end__') {
    currentOrder.splice(fromIdx, 1); currentOrder.push(from)
  } else {
    const toIdx = currentOrder.indexOf(targetGroup)
    if (toIdx === -1) return
    currentOrder.splice(fromIdx, 1); currentOrder.splice(toIdx, 0, from)
  }
  await sessionsStore.saveGroupOrder(currentOrder)
  dragFromGroup.value = ''
}

// ─── 组内会话拖拽排序 ───────────────────────────────────────────
function onSessDragStart(_e: DragEvent, sess: Session, groupName: string) {
  dragFromSessId.value = sess.id
  dragFromSessGrp.value = groupName
}
function onSessDragOver(_e: DragEvent, sess: Session, groupName: string) {
  if (!dragFromSessId.value) return
  if (groupName !== dragFromSessGrp.value) return
  if (sess.id === dragFromSessId.value) return
  dragOverSessId.value = sess.id
}
function onSessDragLeave() { dragOverSessId.value = null }
async function onSessDrop(_e: DragEvent, targetSess: Session, groupName: string) {
  dragOverSessId.value = null
  const fromId = dragFromSessId.value, fromGrp = dragFromSessGrp.value
  dragFromSessId.value = ''; dragFromSessGrp.value = ''
  if (!fromId || fromGrp !== groupName || fromId === targetSess.id) return
  const allSess = sessionsStore.sessions
  const groupIds = allSess.filter(s => (s.group || '默认') === groupName).map(s => s.id)
  const fromIdx = groupIds.indexOf(fromId)
  if (fromIdx === -1) return
  groupIds.splice(fromIdx, 1)
  const insertAt = groupIds.indexOf(targetSess.id)
  groupIds.splice(insertAt, 0, fromId)
  const groupQueue = [...groupIds]; let qi = 0
  const newOrder = allSess.map(s => (s.group || '默认') === groupName ? groupQueue[qi++] : s.id)
  await sessionsStore.reorderSessions(newOrder)
}

// ─── 会话右键菜单 ───────────────────────────────────────────────
const ctxMenu = ref({ visible: false, x: 0, y: 0, sess: null as Session | null })
function openCtxMenu(e: MouseEvent, sess: Session) {
  groupCtxMenu.value.visible = false
  ctxMenu.value = { visible: true, x: e.clientX, y: e.clientY, sess }
  nextTick(() => clampMenu('.sess-ctx-menu', ctxMenu.value, e))
}
function clampMenu(sel: string, menu: { x: number; y: number }, e: MouseEvent) {
  const el = document.querySelector<HTMLElement>(sel)
  if (!el) return
  const rect = el.getBoundingClientRect()
  let x = e.clientX, y = e.clientY
  if (x + rect.width > window.innerWidth) x = window.innerWidth - rect.width - 4
  if (y + rect.height > window.innerHeight) y = window.innerHeight - rect.height - 4
  menu.x = Math.max(4, x); menu.y = Math.max(4, y)
}
function onCtxConnect() { if (ctxMenu.value.sess) emit('connect', ctxMenu.value.sess); ctxMenu.value.visible = false }
function onCtxEdit()    { if (ctxMenu.value.sess) emit('edit', ctxMenu.value.sess); ctxMenu.value.visible = false }
async function onCtxStar() { if (ctxMenu.value.sess) await sessionsStore.toggleStar(ctxMenu.value.sess.id); ctxMenu.value.visible = false }
async function onCtxDuplicate() {
  const sess = ctxMenu.value.sess; ctxMenu.value.visible = false
  if (!sess) return
  await sessionsStore.save({ ...sess, id: '', name: (sess.name || sess.host) + ' (副本)' })
}
async function onCtxDelete() {
  const sess = ctxMenu.value.sess; ctxMenu.value.visible = false
  if (!sess) return
  if (!(await dlg.confirm(`确定删除会话「${sess.name || sess.host}」？`, '删除会话'))) return
  await sessionsStore.remove(sess.id)
}

// ─── 分组右键菜单 ───────────────────────────────────────────────
const groupCtxMenu = ref({ visible: false, x: 0, y: 0, name: '' })
function openGroupCtxMenu(e: MouseEvent, groupName: string) {
  ctxMenu.value.visible = false
  groupCtxMenu.value = { visible: true, x: e.clientX, y: e.clientY, name: groupName }
  nextTick(() => clampMenu('.group-ctx-menu', groupCtxMenu.value, e))
}
async function onGroupCtxRename() {
  const name = groupCtxMenu.value.name; groupCtxMenu.value.visible = false
  const newName = await dlg.prompt('输入新分组名称', '重命名分组', '', name, 'all')
  if (!newName || newName === name) return
  await sessionsStore.renameGroup(name, newName)
}
async function onGroupCtxDelete() {
  const name = groupCtxMenu.value.name; groupCtxMenu.value.visible = false
  const count = sessionsStore.sessions.filter(s => (s.group || '默认') === name).length
  const msg = count > 0
    ? `分组「${name}」下有 ${count} 个会话，删除后将移入默认分组。\n确定继续？`
    : `确定删除空分组「${name}」？`
  if (!(await dlg.confirm(msg, '删除分组'))) return
  await sessionsStore.deleteGroup(name)
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') { ctxMenu.value.visible = false; groupCtxMenu.value.visible = false }
}
onMounted(() => { window.addEventListener('keydown', onKeydown); sessionsStore.fetchAll() })
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<style scoped>
/* 设计令牌（--panel/--accent/--line/--tp/… 等）现由全局 applyColorScheme 按所选
   配色方案统一驱动，见 stores/settings.ts —— 换肤时本面板随之变色。 */
.sidebar {
  width: 100%;
  height: 100%;
  background: var(--panel-2);
  color: var(--tp);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  font-family: 'IBM Plex Sans', 'Segoe UI', system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
}
.mono { font-family: 'IBM Plex Mono', 'Roboto Mono', monospace; }

/* ── 会话头 ── */
.sess-header {
  height: 38px;
  flex: 0 0 38px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  border-bottom: 1px solid var(--line-soft);
}
.head-icon { font-size: 15px; color: var(--accent); }
.head-title {
  font-size: 11px; font-weight: 600; letter-spacing: 1.2px;
  color: var(--ts); text-transform: uppercase;
}
.online-pill {
  display: flex; align-items: center; gap: 4px;
  font-size: 10px; font-weight: 600; color: var(--green);
  background: rgba(70, 207, 139, 0.1);
  padding: 2px 7px 2px 6px; border-radius: 9px;
}
.online-dot {
  width: 5px; height: 5px; border-radius: 50%;
  background: var(--green); box-shadow: 0 0 5px var(--green);
}
.spring { flex: 1; }
.hbtn {
  width: 26px; height: 26px; border-radius: 6px;
  display: flex; align-items: center; justify-content: center;
  background: transparent; border: none; color: var(--ts);
  font-size: 15px; cursor: pointer; transition: background 0.15s, color 0.15s;
}
.hbtn:hover { background: var(--hover); color: var(--accent); }

/* ── 搜索 + 分段 ── */
.sess-filters { padding: 9px 10px 7px; display: flex; flex-direction: column; gap: 8px; container-type: inline-size; }
.search-box {
  height: 32px; background: var(--panel); border: 1px solid var(--line);
  border-radius: 7px; display: flex; align-items: center; gap: 7px; padding: 0 10px;
  transition: border-color 0.15s;
}
.search-box:focus-within { border-color: var(--accent); }
.search-icon { font-size: 13px; color: var(--tm); flex-shrink: 0; }
.search-input {
  flex: 1; background: transparent; border: none; outline: none; padding: 0;
  color: var(--tp); font-size: 12px; font-family: inherit; width: 0;
}
.search-input::placeholder { color: var(--tm); }
.kbd {
  font-size: 10px; color: var(--tm); border: 1px solid var(--line);
  border-radius: 4px; padding: 0 5px; flex-shrink: 0;
}
.clear-btn {
  background: transparent; border: none; color: var(--tm); cursor: pointer;
  font-size: 12px; padding: 2px; display: flex; flex-shrink: 0;
}
.clear-btn:hover { color: var(--tp); }

.segmented { display: flex; gap: 4px; }
.seg {
  flex: 1; min-width: 0; height: 24px; display: flex; align-items: center; justify-content: center;
  gap: 4px; border-radius: 6px; background: transparent; border: none;
  color: var(--ts); font-size: 11px; font-weight: 500; cursor: pointer;
  font-family: inherit; transition: background 0.15s, color 0.15s;
  white-space: nowrap; overflow: hidden;   /* 拖窄时不换行、不撑破 */
}
.seg:hover { background: var(--hover); color: var(--tp); }
.seg.active { background: var(--accent-soft); color: var(--accent); font-weight: 600; }
.seg-count { opacity: 0.6; flex-shrink: 0; }
.seg.active .seg-count { opacity: 0.65; }
.seg-green-dot { width: 5px; height: 5px; border-radius: 50%; background: var(--green); flex-shrink: 0; }
.seg-star { font-size: 11px; color: var(--amber); flex-shrink: 0; }

/* 会话列表拖窄时：先去掉计数数字，再进一步收窄字号，避免文字被挤变形 */
@container (max-width: 250px) {
  .seg-count { display: none; }
}
@container (max-width: 210px) {
  .seg { font-size: 10px; gap: 3px; }
}

/* ── 会话树 ── */
.session-tree { flex: 1; overflow-y: auto; padding: 2px 8px 8px; }
.empty-tip { padding: 20px 12px; font-size: 12px; color: var(--tm); text-align: center; }

.group-block { margin-top: 1px; border-radius: 6px; }
.group-block.drag-over { box-shadow: inset 0 2px 0 var(--accent); }
.group-drop-zone { height: 6px; border-radius: 4px; }
.group-drop-zone.drag-over { box-shadow: inset 0 2px 0 var(--accent); }

.group-header {
  height: 32px; display: flex; align-items: center; gap: 7px;
  padding: 0 8px; border-radius: 6px; cursor: pointer; user-select: none;
  transition: background 0.15s;
}
.group-header:hover { background: var(--hover); }
.group-caret { font-size: 10px; color: var(--tm); width: 10px; flex: 0 0 10px; }
.group-folder { font-size: 15px; flex-shrink: 0; }
.group-name {
  flex: 1; font-size: 12.5px; color: var(--tp); font-weight: 600;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.group-count { display: flex; align-items: baseline; gap: 0; font-size: 10px; }
.group-count-total { color: var(--tm); }

/* ── 会话行 ── */
.session-list { margin: 1px 0 3px 15px; padding-left: 9px; border-left: 1px solid var(--line); }
.session-item {
  height: 26px; display: flex; align-items: center; gap: 9px;
  padding: 0 7px; border-radius: 6px; cursor: pointer; position: relative;
  transition: background 0.12s;
}
.session-item:hover { background: var(--hover); }
.session-item.active { background: var(--accent-soft); }
.session-item.dragging { opacity: 0.4; }
.session-item.drag-over-sess { box-shadow: inset 0 2px 0 var(--accent); }

.sel-bar {
  position: absolute; left: -10px; top: 6px; bottom: 6px; width: 2.5px;
  border-radius: 2px; background: transparent;
}
.session-item.active .sel-bar { background: var(--accent); }

.dot { width: 8px; height: 8px; border-radius: 2px; flex: 0 0 8px; }

.sess-main { flex: 1; display: flex; align-items: center; gap: 5px; min-width: 0; }
.sess-name {
  font-size: 11.5px; letter-spacing: 0.2px;
  -webkit-font-smoothing: antialiased; text-rendering: optimizeLegibility;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.sess-fav { font-size: 10px; color: var(--amber); flex: 0 0 auto; }

.ping { font-size: 10px; padding: 1px 5px; border-radius: 4px; flex: 0 0 auto; }

.sess-more {
  background: transparent; border: none; color: var(--tm); cursor: pointer;
  font-size: 14px; padding: 0; display: flex; align-items: center;
  flex: 0 0 auto; opacity: 0; transition: opacity 0.15s, color 0.15s;
}
.session-item:hover .sess-more { opacity: 1; }
.sess-more:hover { color: var(--tp); }

/* ── 新建会话虚线按钮 ── */
.new-session-btn {
  display: flex; align-items: center; justify-content: center; gap: 6px;
  width: calc(100% - 8px); height: 32px; margin: 6px 4px 2px;
  border: 1px dashed var(--line); border-radius: 7px; background: transparent;
  color: var(--tm); font-size: 12px; cursor: pointer; font-family: inherit;
  transition: border-color 0.15s, color 0.15s;
}
.new-session-btn:hover { border-color: var(--accent); color: var(--accent); }
.new-session-btn i { font-size: 13px; }

/* ── 折叠态 ── */
.collapsed-sessions {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  padding: 10px 0; gap: 8px; overflow-y: auto;
}
.collapsed-item {
  width: 32px; height: 32px; border-radius: 6px; background: transparent;
  border: 1px solid var(--line); cursor: pointer; display: flex;
  align-items: center; justify-content: center; transition: background 0.15s;
}
.collapsed-item:hover { background: var(--hover); }

/* ── 右键菜单 ── */
.ctx-overlay { position: fixed; inset: 0; z-index: 299; }
.ctx-menu {
  position: fixed; z-index: 300; background: var(--panel); color: var(--tp);
  border: 1px solid var(--line); border-radius: 7px; padding: 4px;
  min-width: 130px; box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
  font-family: 'IBM Plex Sans', 'Segoe UI', sans-serif;
}
.ctx-item { padding: 7px 12px; font-size: 12px; color: var(--tp); border-radius: 4px; cursor: pointer; }
.ctx-item:hover { background: var(--hover); }
.ctx-item.danger { color: var(--red); }
.ctx-item.danger:hover { background: rgba(239, 109, 109, 0.12); }
.ctx-divider { height: 1px; background: var(--line); margin: 3px 4px; }

/* 滚动条 */
.scroll::-webkit-scrollbar { width: 6px; }
.scroll::-webkit-scrollbar-thumb { background: var(--line); border-radius: 3px; }
.scroll::-webkit-scrollbar-thumb:hover { background: var(--tm); }
</style>
