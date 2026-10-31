import { ref, computed } from 'vue'
import { defineStore } from 'pinia'

export interface TransferItem {
  id: string
  connId: string
  name: string
  direction: 'upload' | 'download' | 'delete'
  /** 进度单位：字节（传输）或条目数（删除）。影响底部框的展示。 */
  unit: 'bytes' | 'items'
  done: number
  total: number
  status: 'running' | 'done' | 'error' | 'cancelled'
  speed: number          // bytes/s (EMA)
  eta: number            // remaining seconds, -1 = unknown
  _lastDone: number
  _lastTime: number
  errorMsg?: string
  /** 失败后重试的闭包，由创建者（FileManager）注入 */
  retryFn?: () => Promise<void>
  /** 下载的本地落地路径（仅 download 有），完成后可点击打开 */
  localPath?: string
}

export const useTransfersStore = defineStore('transfers', () => {
  const items = ref<TransferItem[]>([])

  const runningItems = computed(() => items.value.filter(t => t.status === 'running'))
  const hasRunning   = computed(() => runningItems.value.length > 0)

  /** Overall progress percent across all running transfers (0-100). */
  const totalPct = computed(() => {
    const active = runningItems.value
    if (!active.length) return 0
    const totalBytes = active.reduce((s, t) => s + t.total, 0)
    const doneBytes  = active.reduce((s, t) => s + t.done,  0)
    return totalBytes > 0 ? Math.round(doneBytes / totalBytes * 100) : 0
  })

  // ── CRUD ──────────────────────────────────────────────────────────────────────

  function add(
    id: string, connId: string, name: string,
    direction: 'upload' | 'download' | 'delete',
    retryFn?: () => Promise<void>,
    localPath?: string,
  ): TransferItem {
    const t: TransferItem = {
      id, connId, name, direction,
      unit: direction === 'delete' ? 'items' : 'bytes',
      done: 0, total: 0, status: 'running',
      speed: 0, eta: -1,
      _lastDone: 0, _lastTime: Date.now(),
      retryFn,
      localPath,
    }
    items.value.push(t)
    return t
  }

  function getById(id: string): TransferItem | undefined {
    return items.value.find(t => t.id === id)
  }

  function updateProgress(id: string, done: number, total: number) {
    const t = getById(id)
    if (!t || t.status !== 'running') return
    const ALPHA = 0.3
    const now   = Date.now()
    const dtMs  = now - t._lastTime
    if (dtMs > 0) {
      const instant = (done - t._lastDone) / (dtMs / 1000)
      t.speed = t.speed === 0 ? instant : t.speed * (1 - ALPHA) + instant * ALPHA
    }
    t._lastDone = done
    t._lastTime = now
    t.done  = done
    t.total = total
    t.eta   = t.speed > 0 && t.total > 0 ? Math.ceil((t.total - t.done) / t.speed) : -1
  }

  function markDone(id: string) {
    const t = getById(id)
    if (!t) return
    t.done   = t.total || t.done
    t.status = 'done'
    // 上传/删除完成项 8s 后自动移除；下载完成项保留（便于点击打开），由"清除"按钮清理
    if (t.direction === 'upload' || t.direction === 'delete') _scheduleRemove(id, 8000)
  }

  function markError(id: string, msg: string) {
    const t = getById(id)
    if (!t) return
    t.status   = 'error'
    t.errorMsg = msg
    // Error items stay until manually cleared
  }

  function markCancelled(id: string) {
    const t = getById(id)
    if (!t) return
    t.status = 'cancelled'
    _scheduleRemove(id, 3000)
  }

  function remove(id: string) {
    const idx = items.value.findIndex(t => t.id === id)
    if (idx !== -1) items.value.splice(idx, 1)
  }

  function clearCompleted() {
    items.value = items.value.filter(t => t.status === 'running')
  }

  // ── Internal ──────────────────────────────────────────────────────────────────

  function _scheduleRemove(id: string, delay: number) {
    setTimeout(() => remove(id), delay)
  }

  return {
    items,
    runningItems, hasRunning, totalPct,
    add, getById, updateProgress, markDone, markError, markCancelled, remove, clearCompleted,
  }
})
