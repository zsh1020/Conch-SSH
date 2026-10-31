<template>
  <div class="app-layout" @keydown="onGlobalKeydown" @contextmenu.prevent tabindex="-1">

    <!-- ─── 标题栏（无边框窗口栏：可拖拽 + 双击最大化）───────────── -->
    <div class="title-bar" style="--wails-draggable: drag" @dblclick="onTitleDblClick">
      <img :src="logoUrl" class="tb-logo" alt="" />
      <span class="tb-brand">Conch</span>
      <span v-if="connSummary" class="tb-divider"></span>
      <span v-if="connSummary" class="tb-conn">
        <i class="ph ph-plug tb-plug"></i><span class="tb-conn-text mono">{{ connSummary }}</span>
      </span>
      <div class="tb-spring"></div>
      <div class="title-actions" style="--wails-draggable: no-drag">
        <button class="tb-btn" title="新建连接 (Ctrl+T)" @click="openNewSession"><i class="ph ph-plus"></i></button>
        <button class="tb-btn" title="快捷键速查" @click="showHelp = !showHelp"><i class="ph ph-question"></i></button>
        <button class="tb-btn" title="设置" @click="showSettings = true"><i class="ph ph-gear-six"></i></button>
      </div>
      <!-- 窗口控制按钮 -->
      <div class="win-ctrls" style="--wails-draggable: no-drag">
        <button class="win-btn" title="最小化" @click="winMinimise">
          <svg width="10" height="10" viewBox="0 0 10 10"><rect x="0" y="4.5" width="10" height="1" fill="currentColor"/></svg>
        </button>
        <button class="win-btn" title="最大化 / 还原" @click="winToggleMaximise">
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1"><rect x="0.5" y="0.5" width="9" height="9"/></svg>
        </button>
        <button class="win-btn win-close" title="关闭" @click="winClose">
          <svg width="10" height="10" viewBox="0 0 10 10" stroke="currentColor" stroke-width="1.2"><line x1="0.5" y1="0.5" x2="9.5" y2="9.5"/><line x1="9.5" y1="0.5" x2="0.5" y2="9.5"/></svg>
        </button>
      </div>
    </div>

    <!-- ─── 内容区（文件列 + 主区域 + 侧边栏） ────────────────── -->
    <div class="content-area">

      <!-- 文件面板列（仅在有标签时显示，全高度） -->
      <div v-if="tabsStore.tabs.length > 0" class="file-column" :style="{ width: filePanelWidth + 'px' }">
        <template v-for="tab in tabsStore.tabs" :key="tab.connId + '-fm'">
          <div v-show="tab.connId === tabsStore.activeConnId" class="file-column-slot">
            <FileManager :conn-id="tab.connId" />
          </div>
        </template>
      </div>

      <!-- 文件面板拖拽分隔线 -->
      <div
        v-if="tabsStore.tabs.length > 0"
        class="panel-resizer"
        @mousedown.prevent="startPanelResize"
      />

      <!-- 主区域（标签栏仅在终端上方显示） -->
      <div class="main-area">

        <!-- 标签页栏 -->
        <TabBar
          :show-snippet="showSnippet"
          :broadcast-active="broadcastMode"
          :show-monitor="showMonitor"
          :show-tunnel="showTunnel"
          :show-local-term="showLocalTerm"
          @toggle-snippet="showSnippet = !showSnippet"
          @manage-snippet="showSnippetDialog = true; showSnippet = false"
          @toggle-broadcast="toggleBroadcast"
          @toggle-monitor="showMonitor = !showMonitor"
          @toggle-tunnel="showTunnel = !showTunnel"
          @toggle-local-term="toggleLocalTerm"
          @new-tab="showConnectDialog = true"
          @reconnect="reconnect"
          @duplicate="duplicateTab"
        />

        <!-- 广播条 -->
        <BroadcastBar v-model="broadcastMode" />

        <!-- 快捷命令栏（标签下方，仅有连接时显示） -->
        <StatusBar v-if="tabsStore.tabs.length > 0" @manage="showSnippetDialog = true" />

        <!-- 会话区域 / 空状态 -->
        <div class="session-area">
          <div v-if="tabsStore.tabs.length === 0" class="empty-state">
            <div class="empty-icon">⌨</div>
            <p class="empty-title">欢迎使用 Conch</p>
            <p class="empty-hint">从右侧双击会话，或点击「＋ 新建连接」</p>
            <div class="empty-shortcuts">
              <kbd>双击</kbd> 连接会话
              <kbd>Ctrl+T</kbd> 新建
              <kbd>Ctrl+W</kbd> 关闭标签
              <kbd>?</kbd> 快捷键
            </div>
          </div>

          <template v-for="tab in tabsStore.tabs" :key="tab.connId">
            <div
              class="tab-session"
              :class="{ active: tab.connId === tabsStore.activeConnId }"
            >
              <Terminal
                :conn-id="tab.connId"
                @closed="onTerminalClosed(tab.connId)"
                @reconnect="reconnect(tab.connId)"
                @close-tab="tabsStore.closeTab(tab.connId)"
              />
            </div>
          </template>
        </div>

        <!-- 底部本地终端 dock -->
        <div
          v-if="localTermMounted"
          v-show="showLocalTerm"
          class="local-term-dock"
          :style="{ height: localTermHeight + 'px' }"
        >
          <div class="lt-resizer" @mousedown.prevent="startLocalTermResize" />
          <LocalTerminalDock ref="localTermRef" @close="hideLocalTerm" />
        </div>


      </div>

      <!-- 侧边栏拖拽手柄 -->
      <div
        v-if="!sidebarCollapsed"
        class="sidebar-resizer"
        @mousedown.prevent="startSidebarResize"
      />

      <!-- 右侧列（会话列表 + 监控面板） -->
      <div
        class="right-column"
        :style="sidebarCollapsed ? { width: '48px', flexShrink: 0 } : { width: sidebarWidth + 'px', flexShrink: 0 }"
      >
        <SessionSidebar
          :collapsed="sidebarCollapsed"
          :style="{ width: '100%', minWidth: '0', flex: '1 1 0', minHeight: '0' }"
          @toggle-collapse="sidebarCollapsed = !sidebarCollapsed"
          @new-session="openNewSession"
          @connect="onConnectSession"
          @edit="openEditSession"
        />
        <div v-if="showMonitor && tabsStore.activeConnId" class="monitor-inline">
          <MonitorPanel
            :conn-id="tabsStore.activeConnId"
            @close="showMonitor = false"
          />
        </div>
      </div>

    </div>

    <!-- ─── 底部全局状态栏 ─────────────────────────────────────── -->
    <GlobalStatusBar @manage="showSettings = true" />

    <!-- ─── 快速连接对话框 ─────────────────────────────────────── -->
    <ConnectDialog
      v-if="showConnectDialog"
      @connected="onQuickConnected"
      @cancel="showConnectDialog = false"
    />

    <!-- ─── 新建/编辑会话对话框 ───────────────────────────────── -->
    <SessionDialog
      v-if="showSessionDialog"
      :session="editingSession ?? undefined"
      :existing-groups="sessionsStore.groups"
      @close="showSessionDialog = false"
      @saved="onSessionSaved"
    />

    <!-- ─── 设置对话框 ────────────────────────────────────────── -->
    <SettingsDialog
      v-if="showSettings"
      @close="showSettings = false"
      @saved="sessionsStore.fetchAll()"
    />

    <!-- ─── 端口转发模态框 ─────────────────────────────────── -->
    <div v-if="showTunnel && tabsStore.activeConnId" class="tunnel-overlay" @click.self="showTunnel = false">
      <div class="tunnel-wrap">
        <TunnelPanel
          :conn-id="tabsStore.activeConnId"
          :session-id="tabsStore.activeTab?.sessionId ?? ''"
          @close="showTunnel = false"
        />
      </div>
    </div>

    <!-- ─── Snippet 管理对话框 ───────────────────────────── -->
    <SnippetDialog
      v-if="showSnippetDialog"
      @close="showSnippetDialog = false"
    />

    <!-- ─── 全局对话框宿主 ──────────────────────────────── -->
    <DialogHost />

    <!-- ─── 快捷键速查面板 ────────────────────────────────── -->
    <HelpPanel v-model="showHelp" />

    <!-- ─── 全局命令面板 ─────────────────────────────────── -->
    <CommandPalette ref="cmdPaletteRef" @connect="onPaletteConnect" />

    <!-- ─── 全局传输队列底栏 ──────────────────────────────── -->
    <TransferFooter />

  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { Connect, Disconnect, SendInput, RespondHostKey } from '../wailsjs/go/main/App'
import { EventsOn, EventsOff, WindowMinimise, WindowToggleMaximise, Quit } from '../wailsjs/runtime/runtime'
import Terminal         from './components/Terminal.vue'
import FileManager      from './components/FileManager.vue'
import TunnelPanel      from './components/TunnelPanel.vue'
import MonitorPanel     from './components/MonitorPanel.vue'
import SnippetDialog    from './components/SnippetDialog.vue'
import ConnectDialog    from './components/ConnectDialog.vue'
import SessionDialog    from './components/SessionDialog.vue'
import SessionSidebar   from './components/SessionSidebar.vue'
import SettingsDialog   from './components/SettingsDialog.vue'
import DialogHost       from './components/DialogHost.vue'
import TabBar           from './components/TabBar.vue'
import BroadcastBar     from './components/BroadcastBar.vue'
import StatusBar        from './components/StatusBar.vue'
import LocalTerminalDock from './components/LocalTerminalDock.vue'
import GlobalStatusBar   from './components/GlobalStatusBar.vue'
import HelpPanel        from './components/HelpPanel.vue'
import CommandPalette   from './components/CommandPalette.vue'
import TransferFooter   from './components/TransferFooter.vue'
import { useSessionsStore, type Session } from './stores/sessions'
import { useTabsStore, type ConnectRequest } from './stores/tabs'
import { useSettingsStore } from './stores/settings'
import { useSnippetsStore } from './stores/snippets'
import { useDialogStore }   from './stores/dialogs'
import logoUrl from './assets/images/logo.png'

const sessionsStore  = useSessionsStore()
const tabsStore      = useTabsStore()
const settingsStore  = useSettingsStore()
const snippetsStore  = useSnippetsStore()
const dialogStore    = useDialogStore()

// ─── 无边框窗口控制 ────────────────────────────────────────────
function winMinimise()        { WindowMinimise() }
function winToggleMaximise()  { WindowToggleMaximise() }
function winClose()           { Quit() }
// 双击标题栏最大化/还原（点在按钮上不触发：按钮 stop 冒泡不必要，dblclick 目标非 .title-bar）
function onTitleDblClick(e: MouseEvent) {
  if ((e.target as HTMLElement).closest('.title-actions, .win-ctrls')) return
  WindowToggleMaximise()
}

// 标题栏连接摘要：当前激活连接的 用户@会话名 · 主机
const connSummary = computed(() => {
  const t = tabsStore.tabs.find(tb => tb.connId === tabsStore.activeConnId)
  if (!t || t.status !== 'connected') return ''
  const sess = t.sessionId ? sessionsStore.sessions.find(s => s.id === t.sessionId) : null
  const host = sess?.host || t.lastConnectReq?.host || ''
  const user = sess?.username || t.lastConnectReq?.username || ''
  const name = t.label || host
  if (!host) return ''
  return user ? `${user}@${name} · ${host}` : `${name} · ${host}`
})

// 切换标签后等 DOM 展示再触发终端 fit，避免 display:none 时 resize 引发服务端重绘
watch(() => tabsStore.activeConnId, (connId) => {
  if (!connId) return
  nextTick(() => {
    window.dispatchEvent(new CustomEvent('gssh:fit-terminal', { detail: connId }))
  })
})

// ─── 侧边栏宽度 ────────────────────────────────────────────────
const SIDEBAR_MIN     = 200
const SIDEBAR_MAX     = 480
const SIDEBAR_DEFAULT = 298

const sidebarCollapsed = ref(false)
const sidebarWidth = ref(
  parseInt(localStorage.getItem('sidebar-width') ?? String(SIDEBAR_DEFAULT), 10)
)

function startSidebarResize(e: MouseEvent) {
  const startX = e.clientX
  const startW = sidebarWidth.value
  function onMove(ev: MouseEvent) {
    sidebarWidth.value = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, startW - (ev.clientX - startX)))
  }
  function onUp() {
    localStorage.setItem('sidebar-width', String(sidebarWidth.value))
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

// ─── 文件面板宽度 ───────────────────────────────────────────────
const FILE_PANEL_MIN     = 200
const FILE_PANEL_MAX     = 600
const FILE_PANEL_DEFAULT = 320

const filePanelWidth = ref(
  parseInt(localStorage.getItem('file-panel-width') ?? String(FILE_PANEL_DEFAULT), 10)
)

function startPanelResize(e: MouseEvent) {
  const startX = e.clientX
  const startW = filePanelWidth.value
  function onMove(ev: MouseEvent) {
    filePanelWidth.value = Math.min(FILE_PANEL_MAX, Math.max(FILE_PANEL_MIN, startW + (ev.clientX - startX)))
  }
  function onUp() {
    localStorage.setItem('file-panel-width', String(filePanelWidth.value))
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

// ─── 对话框 / 面板状态 ──────────────────────────────────────────
const showConnectDialog = ref(false)
const showSessionDialog = ref(false)
const showSettings      = ref(false)
const showTunnel        = ref(false)
// 服务器监控默认打开（localStorage 记住用户选择，有活动连接时才实际渲染）
const showMonitor       = ref(localStorage.getItem('show-monitor') !== 'false')
const showSnippet       = ref(false)
const showSnippetDialog = ref(false)
const showHelp          = ref(false)
const cmdPaletteRef     = ref<InstanceType<typeof CommandPalette>>()

// ─── 底部本地终端 ───────────────────────────────────────────────
// 默认打开（localStorage 记住用户选择）
const localTermRef     = ref<InstanceType<typeof LocalTerminalDock>>()
const _localTermOpen   = localStorage.getItem('show-localterm') !== 'false'
const localTermMounted = ref(_localTermOpen)  // 首次打开后保持挂载，隐藏用 v-show 保活
const showLocalTerm    = ref(_localTermOpen)
const localTermHeight  = ref(
  parseInt(localStorage.getItem('local-term-height') ?? '220', 10)
)

// 持久化两个面板的开关状态
watch(showMonitor,    v => localStorage.setItem('show-monitor',   String(v)))
watch(showLocalTerm,  v => localStorage.setItem('show-localterm', String(v)))

function fitActiveTerminal() {
  if (tabsStore.activeConnId)
    window.dispatchEvent(new CustomEvent('gssh:fit-terminal', { detail: tabsStore.activeConnId }))
}

function toggleLocalTerm() {
  if (!localTermMounted.value) {
    localTermMounted.value = true
    showLocalTerm.value = true
  } else {
    showLocalTerm.value = !showLocalTerm.value
  }
  nextTick(() => {
    fitActiveTerminal()
    if (showLocalTerm.value) {
      localTermRef.value?.ensureTerminal()
      localTermRef.value?.refit()
      localTermRef.value?.focus()
    }
  })
}

function hideLocalTerm() {
  showLocalTerm.value = false
  nextTick(fitActiveTerminal)
}

function startLocalTermResize(e: MouseEvent) {
  const startY = e.clientY
  const startH = localTermHeight.value
  function onMove(ev: MouseEvent) {
    const delta = startY - ev.clientY  // 向上拖 = 变高
    localTermHeight.value = Math.min(window.innerHeight * 0.6, Math.max(80, startH + delta))
  }
  function onUp() {
    localStorage.setItem('local-term-height', String(localTermHeight.value))
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

// ─── 广播 ──────────────────────────────────────────────────────
const broadcastMode = ref(false)

function toggleBroadcast() {
  broadcastMode.value = !broadcastMode.value
}

watch(() => tabsStore.tabs.length, (n) => {
  if (n === 0) broadcastMode.value = false
})

// ─── 会话管理 ──────────────────────────────────────────────────
const editingSession = ref<Session | null>(null)

function openNewSession() {
  editingSession.value = null
  showSessionDialog.value = true
}

function openEditSession(sess: Session) {
  editingSession.value = sess
  showSessionDialog.value = true
}

function onSessionSaved(_sess: Session) {
  showSessionDialog.value = false
}

// ─── 连接逻辑 ──────────────────────────────────────────────────
async function onConnectSession(sess: Session) {
  const label = sess.name || `${sess.username}@${sess.host}`
  const req: ConnectRequest = { session_id: sess.id, password: sess.password ?? '' }
  try {
    const connId = await Connect(req, 120, 30)
    tabsStore.openTab(connId, label, sess.id, req)
  } catch (e: any) {
    dialogStore.error(String(e), '连接失败')
  }
}

function onQuickConnected(connId: string, label: string) {
  showConnectDialog.value = false
  tabsStore.openTab(connId, label)
}

/** 命令面板触发的连接 */
async function onPaletteConnect(sessionId: string) {
  const sess = sessionsStore.sessions.find(s => s.id === sessionId)
  if (sess) await onConnectSession(sess)
}

// ─── 终端断线处理 + 自动重连 ──────────────────────────────────
const _autoReconnectTimers = new Map<string, ReturnType<typeof setTimeout>>()

function onTerminalClosed(connId: string) {
  const tab = tabsStore.tabs.find(t => t.connId === connId)
  if (!tab) return
  tabsStore.setStatus(connId, 'closed')

  // 用户主动关闭 or 无保存连接参数 → 不自动重连
  if (tab.manualClose || !tab.lastConnectReq) return

  const attempt = (tab.reconnectCount ?? 0) + 1
  if (attempt > 3) {
    tabsStore.setReconnectAt(connId, 0)
    return
  }

  // 指数退避：1s / 2s / 4s
  const delay = Math.pow(2, attempt - 1) * 1000
  tab.reconnectCount = attempt
  tabsStore.setReconnectAt(connId, Date.now() + delay)

  const timer = setTimeout(() => {
    _autoReconnectTimers.delete(connId)
    tabsStore.setReconnectAt(connId, 0)
    reconnect(connId)
  }, delay)
  _autoReconnectTimers.set(connId, timer)
}

// tab 被关闭时取消尚未触发的自动重连定时器
watch(() => tabsStore.tabs.map(t => t.connId), (curr, prev) => {
  for (const id of prev) {
    if (!curr.includes(id)) {
      const t = _autoReconnectTimers.get(id)
      if (t) { clearTimeout(t); _autoReconnectTimers.delete(id) }
    }
  }
})

async function reconnect(connId: string) {
  const tab = tabsStore.tabs.find(t => t.connId === connId)
  if (!tab?.lastConnectReq) return
  // 先标记为重连中，再断开（Terminal 收到 closed 事件时检查此状态以跳过断线遮罩）
  tabsStore.setStatus(connId, 'connecting')
  await Disconnect(connId).catch(() => {})
  try {
    const newConnId = await Connect(tab.lastConnectReq, 120, 30)
    const idx = tabsStore.tabs.findIndex(t => t.connId === connId)
    if (idx !== -1) {
      tabsStore.tabs[idx].connId = newConnId
      tabsStore.tabs[idx].status = 'connected'
      tabsStore.tabs[idx].reconnectCount = 0
      if (tabsStore.activeConnId === connId) tabsStore.activeConnId = newConnId
    }
  } catch {
    tabsStore.setStatus(connId, 'closed')
  }
}

async function duplicateTab(connId: string) {
  const tab = tabsStore.tabs.find(t => t.connId === connId)
  if (!tab?.lastConnectReq) return
  try {
    const newConnId = await Connect(tab.lastConnectReq, 120, 30)
    tabsStore.openTab(newConnId, tab.label, tab.sessionId, tab.lastConnectReq)
  } catch (e: any) {
    dialogStore.error('连接失败：' + String(e))
  }
}

// ─── 键盘快捷键 ────────────────────────────────────────────────
function onGlobalKeydown(e: KeyboardEvent) {
  const tag = (e.target as HTMLElement)?.tagName
  if (e.key === '?' && tag !== 'INPUT' && tag !== 'TEXTAREA') {
    showHelp.value = !showHelp.value
    return
  }
  if (e.key === 'Escape') {
    if (showHelp.value) { showHelp.value = false; return }
  }
  if (e.ctrlKey) {
    switch (e.key) {
      case '`':
        e.preventDefault()
        toggleLocalTerm()
        return
      case 'k': case 'K':
        e.preventDefault()
        cmdPaletteRef.value?.open()
        return
      case 't': case 'T':
        e.preventDefault()
        showConnectDialog.value = true
        break
      case 'w': case 'W':
        e.preventDefault()
        if (tabsStore.activeConnId) tabsStore.closeTab(tabsStore.activeConnId)
        break
      case 'Tab':
        e.preventDefault()
        e.shiftKey ? tabsStore.activatePrev() : tabsStore.activateNext()
        break
      default:
        if (e.key >= '1' && e.key <= '9') {
          e.preventDefault()
          tabsStore.activateByIndex(Number(e.key) - 1)
        }
    }
  }
}

// ─── Lifecycle ─────────────────────────────────────────────────
function onToggleLocalTermEvent() { toggleLocalTerm() }

onMounted(() => {
  sessionsStore.fetchAll()
  settingsStore.load()
  snippetsStore.fetchAll()

  window.addEventListener('gssh:toggle-localterm', onToggleLocalTermEvent)

  // 默认打开本地终端时，启动后建一个默认终端并适配尺寸
  if (showLocalTerm.value) {
    nextTick(() => localTermRef.value?.ensureTerminal())
  }

  // ── 主机指纹确认（替代原生系统弹窗） ─────────────────────────
  EventsOn('hostkey:confirm', async (p: { id: string; host: string; fingerprint: string }) => {
    const ok = await dialogStore.confirm(
      `主机：${p.host}\n指纹：${p.fingerprint}\n\n是否信任此主机并继续连接？`,
      '未知主机'
    )
    RespondHostKey(p.id, ok)
  })

  EventsOn('hostkey:changed', async (p: { id: string; host: string; old_fp: string; new_fp: string }) => {
    const ok = await dialogStore.confirm(
      `主机 ${p.host} 的指纹已变更！\n\n旧指纹：${p.old_fp}\n新指纹：${p.new_fp}\n\n` +
      `这可能意味着中间人攻击（MITM）。\n仅在确认服务器密钥已更换时点击确定。`,
      '⚠️ 主机指纹已变更'
    )
    RespondHostKey(p.id, ok)
  })
})

onUnmounted(() => {
  EventsOff('hostkey:confirm')
  EventsOff('hostkey:changed')
  window.removeEventListener('gssh:toggle-localterm', onToggleLocalTermEvent)
})
</script>

<style>
.app-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--term);
  outline: none;
  font-family: 'IBM Plex Sans', 'Segoe UI', system-ui, sans-serif;
}
.title-bar .mono { font-family: 'IBM Plex Mono', 'Roboto Mono', monospace; }

/* ─── 标题栏 46px ─── */
.title-bar {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 0 0 0 12px;         /* 右侧不留白，让窗口控制按钮贴到窗口边缘 */
  height: 46px;
  background: linear-gradient(180deg, var(--panel), var(--panel-2));
  border-bottom: 1px solid var(--line);
  flex-shrink: 0;
}

.tb-logo { width: 22px; height: 22px; border-radius: 6px; display: block; flex-shrink: 0; }
.tb-brand { font-size: 14px; font-weight: 600; color: var(--tp); letter-spacing: 0.3px; }
.tb-divider { width: 1px; height: 18px; background: var(--line); flex-shrink: 0; }
.tb-conn { display: flex; align-items: center; gap: 6px; min-width: 0; }
.tb-plug { font-size: 13px; color: var(--green); flex-shrink: 0; }
.tb-conn-text {
  font-size: 11.5px; color: var(--ts);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.tb-spring { flex: 1; min-width: 8px; }

/* ─── 标题栏工具图标按钮（新建连接 / 帮助 / 设置，统一等大无边框 Phosphor 图标）─── */
.tb-btn {
  width: 32px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  border-radius: 7px;
  color: var(--ts);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  transition: color 0.15s, background 0.15s;
}
.tb-btn:hover  { color: var(--accent); background: var(--hover); }
.tb-btn:active { background: var(--accent-soft); }

.title-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-right: 10px;          /* 与右侧窗口控制按钮之间留点间距 */
}

/* ─── 窗口控制按钮（无边框窗口）─── */
.win-ctrls {
  display: flex;
  align-items: stretch;
  height: 100%;
  flex-shrink: 0;
}
.win-btn {
  width: 44px;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.win-btn:hover { background: rgba(255,255,255,0.09); color: var(--text-primary); }
.win-btn:active { background: rgba(255,255,255,0.14); }
.win-close:hover { background: #e0524d; color: #fff; }
.win-close:active { background: #ec7a76; }

/* ─── 内容区 ─── */
.content-area {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* ─── 侧边栏拖拽手柄 ─── */
.sidebar-resizer {
  width: 4px;
  flex-shrink: 0;
  cursor: col-resize;
  background: transparent;
  transition: background 0.15s;
}
.sidebar-resizer:hover,
.sidebar-resizer:active { background: var(--accent); }

/* ─── 主区域 ─── */
.main-area {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
}

/* ─── 端口转发模态框 ─── */
.tunnel-overlay {
  position: fixed;
  inset: 0;
  z-index: 150;
  background: rgba(0,0,0,0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(2px);
}

.tunnel-wrap {
  position: relative;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 600px;
  max-width: 90vw;
  height: 70vh;
  overflow: hidden;
  box-shadow: 0 24px 48px rgba(0,0,0,0.6);
  display: flex;
  flex-direction: column;
}


/* ─── 右侧列（会话 + 监控） ─── */
.right-column {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: width 0.2s;
  border-left: 1px solid var(--line);   /* 会话面板与终端的分隔边框（对齐设计稿）*/
}

.monitor-inline {
  flex-shrink: 0;
  overflow-y: auto;
  max-height: 46%;   /* 监控占右列下部，过高时可滚动 */
}

/* ─── 会话区域 ─── */
.session-area {
  flex: 1;
  position: relative;
  overflow: hidden;
  background: var(--term);
}

/* ─── 底部本地终端 dock ─── */
.local-term-dock {
  position: relative;
  flex-shrink: 0;
  border-top: 1px solid var(--border);
  overflow: hidden;
}

.lt-resizer {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 5px;
  cursor: row-resize;
  z-index: 10;
  background: transparent;
  transition: background 0.15s;
}
.lt-resizer:hover,
.lt-resizer:active { background: var(--accent); }

/* ─── 文件面板列（content-area 级别） ─── */
.file-column {
  flex-shrink: 0;
  overflow: hidden;
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
}

.file-column-slot {
  flex: 1;
  overflow: hidden;
}

.tab-session {
  position: absolute;
  inset: 0;
  display: none;
  background: inherit;
}

.tab-session.active { display: flex; }

.panel-resizer {
  width: 4px;
  flex-shrink: 0;
  cursor: col-resize;
  background: transparent;
  transition: background 0.15s;
}
.panel-resizer:hover,
.panel-resizer:active { background: var(--accent); }

/* ─── 空状态 ─── */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 8px;
  color: var(--text-muted);
}

.empty-icon { font-size: 44px; opacity: 0.3; }

.empty-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin-top: 4px;
}

.empty-hint { font-size: 13px; }

.empty-shortcuts {
  margin-top: 12px;
  display: flex;
  gap: 12px;
  align-items: center;
  font-size: 12px;
  color: var(--text-muted);
}

kbd {
  display: inline-block;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 2px 6px;
  font-size: 11px;
  font-family: monospace;
  color: var(--text-primary);
}
</style>
