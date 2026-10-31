import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { Disconnect } from '../../wailsjs/go/main/App'

export type TabStatus = 'connecting' | 'connected' | 'closed' | 'reconnecting'

export interface ConnectRequest {
  session_id?: string
  host?: string
  port?: number
  username?: string
  auth_type?: string
  password?: string
  identity_file?: string
}

export interface Tab {
  connId: string
  sessionId?: string
  label: string
  status: TabStatus
  color: string                    // 会话标识色，由 sessionId hash 决定
  hasNewOutput: boolean            // 非活跃时有新输出
  lastConnectReq?: ConnectRequest  // for reconnect
  reconnectCount?: number
  reconnectAt?: number             // epoch ms when next auto-reconnect fires
  manualClose?: boolean            // true when user explicitly closed the tab
}

/** 根据 sessionId（或 label）做确定性哈希 → HSL 颜色
 *  黄金角（137.5°）确保任意两个 session 色相都尽量分散，不出现大量撞色。
 */
export function pickColor(sessionId?: string, label = ''): string {
  const key = sessionId ?? label
  let h = 5381
  for (let i = 0; i < key.length; i++) h = ((h << 5) + h ^ key.charCodeAt(i)) >>> 0

  // 黄金比例倍数取小数部分 → 色相均匀覆盖 360°
  const hue = Math.round((h * 0.618033988749895 % 1) * 360)
  // 饱和度 70-86%，亮度 60-70%，保证深色主题下鲜艳可见
  const sat = 70 + (h % 17)
  const lit = 60 + (h % 11)

  return `hsl(${hue}, ${sat}%, ${lit}%)`
}

export const useTabsStore = defineStore('tabs', () => {
  const tabs = ref<Tab[]>([])
  const activeConnId = ref('')

  const activeTab = computed(() => tabs.value.find(t => t.connId === activeConnId.value))

  function openTab(connId: string, label: string, sessionId?: string, req?: ConnectRequest) {
    const color = pickColor(sessionId, label)
    tabs.value.push({ connId, label, sessionId, status: 'connected', color, hasNewOutput: false, lastConnectReq: req })
    activeConnId.value = connId
  }

  /** 非活跃 tab 收到输出时标记 */
  function markNewOutput(connId: string) {
    const tab = tabs.value.find(t => t.connId === connId)
    if (tab && connId !== activeConnId.value) tab.hasNewOutput = true
  }

  async function closeTab(connId: string) {
    // 标记为用户主动关闭，防止触发自动重连
    const closing = tabs.value.find(t => t.connId === connId)
    if (closing) closing.manualClose = true
    await Disconnect(connId).catch(() => {})
    const idx = tabs.value.findIndex(t => t.connId === connId)
    if (idx === -1) return
    tabs.value.splice(idx, 1)
    if (activeConnId.value === connId) {
      activeConnId.value = tabs.value[Math.max(0, idx - 1)]?.connId ?? ''
    }
  }

  function setStatus(connId: string, status: TabStatus) {
    const tab = tabs.value.find(t => t.connId === connId)
    if (tab) tab.status = status
  }

  function setReconnectAt(connId: string, at: number) {
    const tab = tabs.value.find(t => t.connId === connId)
    if (tab) tab.reconnectAt = at
  }

  function activateNext() {
    const idx = tabs.value.findIndex(t => t.connId === activeConnId.value)
    if (idx === -1 || tabs.value.length < 2) return
    activeConnId.value = tabs.value[(idx + 1) % tabs.value.length].connId
  }

  function activatePrev() {
    const idx = tabs.value.findIndex(t => t.connId === activeConnId.value)
    if (idx === -1 || tabs.value.length < 2) return
    activeConnId.value = tabs.value[(idx - 1 + tabs.value.length) % tabs.value.length].connId
  }

  function activateByIndex(n: number) {
    const tab = tabs.value[n]
    if (tab) activeConnId.value = tab.connId
  }

  function reorderTab(fromIndex: number, toIndex: number) {
    if (fromIndex === toIndex) return
    const tab = tabs.value.splice(fromIndex, 1)[0]
    tabs.value.splice(toIndex, 0, tab)
  }

  // 切换到某 tab 时同步清除其新输出标记（flush:'sync' 避免异步 tick 间的竞态）
  watch(activeConnId, (connId) => {
    const tab = tabs.value.find(t => t.connId === connId)
    if (tab) tab.hasNewOutput = false
  }, { flush: 'sync' })

  return {
    tabs, activeConnId, activeTab,
    openTab, closeTab, setStatus, setReconnectAt, markNewOutput,
    activateNext, activatePrev, activateByIndex,
    reorderTab,
  }
})
