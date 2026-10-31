<template>
  <Transition name="tf-appear">
    <div v-if="store.items.length > 0" class="tf-root" :class="{ 'tf-open': expanded }">

      <!-- ── 顶部常驻栏 ───────────────────────────────────────────── -->
      <div class="tf-bar" @click="expanded = !expanded">
        <span class="tf-icon">⇅</span>
        <span class="tf-title">传输</span>

        <!-- 进行中数量 -->
        <span v-if="store.hasRunning" class="tf-badge">{{ store.runningItems.length }}</span>

        <!-- 折叠时：总进度条 + 速度 -->
        <template v-if="!expanded && store.hasRunning">
          <div class="tf-mini-track">
            <div class="tf-mini-fill" :style="{ width: store.totalPct + '%' }" />
          </div>
          <span class="tf-mini-pct">{{ store.totalPct }}%</span>
          <span v-if="totalSpeed > 0" class="tf-mini-speed">{{ fmtBytes(totalSpeed) }}/s</span>
        </template>

        <!-- 展开时：显示完成状态 -->
        <template v-if="expanded && !store.hasRunning && store.items.length > 0">
          <span class="tf-all-done">全部完成</span>
        </template>

        <div class="tf-bar-btns" @click.stop>
          <button
            v-if="store.items.some(t => t.status !== 'running')"
            class="tf-btn"
            title="清除已完成/失败项"
            @click="store.clearCompleted()"
          >清除</button>
          <button class="tf-btn tf-toggle" :title="expanded ? '折叠' : '展开'">
            {{ expanded ? '▼' : '▲' }}
          </button>
        </div>
      </div>

      <!-- ── 传输列表（横向卡片，展开后可见） ─────────────────────── -->
      <div class="tf-list">
        <div class="tf-items-row">
          <div
            v-for="t in itemsNewestFirst"
            :key="t.id"
            class="tf-item"
            :class="`tf-s-${t.status}`"
          >
            <!-- 第一行：方向图标 + 文件名(+主机) + 取消按钮 -->
            <div class="tf-item-top">
              <i class="tf-dir ph" :class="dirIconClass(t)"></i>
              <span v-if="t.direction === 'delete'" class="tf-tag-del">删除</span>
              <span
                class="tf-name"
                :class="{ 'tf-name-link': canOpen(t) }"
                :title="canOpen(t) ? '点击打开：' + t.name : t.name"
                @click="canOpen(t) && openFile(t)"
              >{{ t.name }}</span>
              <span v-if="connLabel(t)" class="tf-host" :title="connLabel(t)!">{{ connLabel(t) }}</span>
              <button v-if="t.status === 'running'" class="tf-cancel" title="取消" @click="cancelTransfer(t)">✕</button>
              <template v-else-if="t.status === 'error'">
                <button v-if="t.retryFn" class="tf-retry" title="重试" @click="retryTransfer(t)">↺</button>
                <button class="tf-cancel" title="移除" @click="store.remove(t.id)">✕</button>
              </template>
              <template v-else-if="canOpen(t)">
                <button class="tf-reveal" title="在文件夹中显示" @click="revealFile(t)">📂</button>
                <button class="tf-cancel" title="移除" @click="store.remove(t.id)">✕</button>
              </template>
            </div>

            <!-- 第二行：进度条 -->
            <div class="tf-track">
              <div
                class="tf-fill"
                :class="{
                  'tf-fill-done':   t.status === 'done',
                  'tf-fill-error':  t.status === 'error',
                  'tf-fill-cancel': t.status === 'cancelled',
                  'tf-fill-delete': t.status === 'running' && t.direction === 'delete',
                }"
                :style="{ width: pct(t) + '%' }"
              />
            </div>

            <!-- 第三行：百分比 + 速度/ETA/状态 -->
            <div class="tf-item-meta">
              <span class="tf-pct">{{ pct(t) }}%</span>
              <template v-if="t.status === 'running'">
                <span v-if="t.unit === 'items'" class="tf-eta">{{ t.done }} / {{ t.total }} 项</span>
                <template v-else>
                  <span v-if="t.speed > 0" class="tf-speed">{{ fmtBytes(Math.round(t.speed)) }}/s</span>
                  <span v-if="t.eta > 0" class="tf-eta">{{ fmtEta(t.eta) }}</span>
                </template>
              </template>
              <span v-else-if="t.status === 'done'"      class="tf-si tf-si-done">{{ t.direction === 'delete' ? '✓ 已删除' : '✓ 完成' }}</span>
              <span v-else-if="t.status === 'error'"     class="tf-si tf-si-error" :title="t.errorMsg">✗ {{ shortErr(t.errorMsg) }}</span>
              <span v-else-if="t.status === 'cancelled'" class="tf-si tf-si-cancel">⊘ 已取消</span>
            </div>
          </div>
        </div>
      </div>

    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useTransfersStore, type TransferItem } from '../stores/transfers'
import { useTabsStore } from '../stores/tabs'
import { useDialogStore } from '../stores/dialogs'
import { SFTPCancelTransfer, OpenLocalPath, RevealLocalPath } from '../../wailsjs/go/main/App'

const store    = useTransfersStore()
const tabs     = useTabsStore()
const dlg      = useDialogStore()
const expanded = ref(true)

/** 最新的传输/删除项显示在最前边 */
const itemsNewestFirst = computed(() => [...store.items].reverse())

/** 已完成的下载项且有本地落地路径 → 可点击打开 / 在文件夹中显示 */
function canOpen(t: TransferItem): boolean {
  return t.status === 'done' && t.direction === 'download' && !!t.localPath
}

async function openFile(t: TransferItem) {
  if (!t.localPath) return
  try {
    await OpenLocalPath(t.localPath)
  } catch (e: any) {
    dlg.error('打开失败：' + String(e))
  }
}

async function revealFile(t: TransferItem) {
  if (!t.localPath) return
  try {
    await RevealLocalPath(t.localPath)
  } catch (e: any) {
    dlg.error('打开所在文件夹失败：' + String(e))
  }
}

/** 返回传输项对应的会话标签（主机名），多连接时用于区分 */
function connLabel(t: TransferItem): string | null {
  if (tabs.tabs.length <= 1) return null   // 只有一个连接时不显示
  return tabs.tabs.find(tab => tab.connId === t.connId)?.label ?? null
}

// Auto-expand whenever a new transfer starts
watch(() => store.items.length, (newLen, oldLen) => {
  if (newLen > oldLen) expanded.value = true
})

// ── Helpers ───────────────────────────────────────────────────────────────────

function pct(t: TransferItem): number {
  if (t.status === 'done') return 100
  if (t.total === 0) return 0
  return Math.min(99, Math.round(t.done / t.total * 100))
}

/** 方向图标（Phosphor）：上传 / 下载 / 删除 */
function dirIconClass(t: TransferItem): string {
  if (t.direction === 'upload') return 'ph-arrow-up'
  if (t.direction === 'delete') return 'ph-trash'
  return 'ph-arrow-down'
}

/** Sum speed of all running transfers for the mini bar（删除是按条目计，不计入字节速度）。 */
const totalSpeed = computed(() =>
  store.runningItems.filter(t => t.unit !== 'items').reduce((s, t) => s + t.speed, 0)
)

function fmtBytes(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 * 1024 * 1024) return (n / (1024 * 1024)).toFixed(1) + ' MB'
  return (n / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
}

function fmtEta(secs: number): string {
  if (secs <= 0) return ''
  if (secs < 60) return `${secs}s`
  const m = Math.floor(secs / 60), s = secs % 60
  return `${m}m${s.toString().padStart(2, '0')}s`
}

function shortErr(msg?: string): string {
  if (!msg) return '失败'
  return msg.length > 30 ? msg.slice(0, 30) + '…' : msg
}

async function cancelTransfer(t: TransferItem) {
  store.markCancelled(t.id)
  try { await SFTPCancelTransfer(t.id) } catch { /* ignore */ }
}

async function retryTransfer(t: TransferItem) {
  if (!t.retryFn) return
  const fn = t.retryFn
  store.remove(t.id)   // 移除失败项，retryFn 会创建新的
  try { await fn() } catch { /* error handled inside fn */ }
}
</script>

<style scoped>
/* ── Root ── */
.tf-root {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--bg-secondary);
  border-top: 1px solid var(--border);
  overflow: hidden;
  font-size: 12px;
  color: var(--text-primary);
  /* height controlled by .tf-list max-height below */
}

/* ── Top bar ── */
.tf-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 28px;
  padding: 0 12px;
  cursor: pointer;
  user-select: none;
  flex-shrink: 0;
}
.tf-bar:hover { background: rgba(255,255,255,0.025); }

.tf-icon  { font-size: 13px; opacity: 0.7; flex-shrink: 0; }
.tf-title { font-size: 12px; font-weight: 600; flex-shrink: 0; }

.tf-badge {
  background: var(--accent);
  color: #11111b;
  border-radius: 20px;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  flex-shrink: 0;
  line-height: 1.6;
}

/* Mini progress bar (collapsed only) */
.tf-mini-track {
  flex: 1;
  height: 3px;
  background: rgba(255,255,255,0.08);
  border-radius: 2px;
  overflow: hidden;
  min-width: 40px;
  max-width: 200px;
}
.tf-mini-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 2px;
  transition: width 0.35s ease;
}
.tf-mini-pct {
  font-size: 11px;
  color: var(--text-muted);
  flex-shrink: 0;
}
.tf-mini-speed {
  font-size: 11px;
  color: var(--accent);
  font-family: monospace;
  flex-shrink: 0;
}

.tf-all-done {
  font-size: 11px;
  color: #a6e3a1;
  flex-shrink: 0;
}

/* Bar action buttons */
.tf-bar-btns {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
}
.tf-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 11px;
  cursor: pointer;
  padding: 3px 7px;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
  line-height: 1;
}
.tf-btn:hover { color: var(--text-primary); background: rgba(255,255,255,0.06); }

/* ── Expanded list ── */
.tf-list {
  max-height: 0;
  overflow: hidden;
  transition: max-height 0.22s ease;
}
.tf-open .tf-list {
  max-height: 120px;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: thin;
  scrollbar-color: rgba(255,255,255,0.1) transparent;
}

/* ── Transfer cards (horizontal) ── */
.tf-items-row {
  display: flex;
  flex-direction: row;
  align-items: stretch;
  gap: 0;
  padding: 0;
  min-width: max-content;
}

.tf-item {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 5px;
  padding: 6px 12px;
  min-width: 200px;
  max-width: 260px;
  border-right: 1px solid var(--border);
  transition: background 0.1s;
  flex-shrink: 0;
}
.tf-item:hover { background: rgba(255,255,255,0.03); }
.tf-item:last-child { border-right: none; }

/* Dim completed cards */
.tf-s-done, .tf-s-cancelled { opacity: 0.6; }

/* Card top row: direction icon + filename + cancel */
.tf-item-top {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}

.tf-dir  { flex-shrink: 0; font-size: 13px; opacity: 0.85; line-height: 1; }
/* 删除项：图标红色 + 「删除」标签，避免误认为下载 */
.tf-item:has(.tf-tag-del) .tf-dir { color: var(--red, #f38ba8); opacity: 1; }
.tf-tag-del {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  color: var(--red, #f38ba8);
  background: rgba(243,139,168,0.14);
  border-radius: 3px;
  padding: 0 5px;
  line-height: 1.6;
}

.tf-name {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 12px;
  min-width: 0;
}

/* 可点击打开的下载文件名 */
.tf-name-link { cursor: pointer; }
.tf-name-link:hover { color: var(--accent); text-decoration: underline; }

.tf-host {
  flex-shrink: 0;
  font-size: 10px;
  color: var(--text-muted);
  background: rgba(255,255,255,0.06);
  border-radius: 3px;
  padding: 0 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 70px;
}

/* Card middle: progress bar */
.tf-track {
  height: 3px;
  background: rgba(255,255,255,0.08);
  border-radius: 2px;
  overflow: hidden;
}
.tf-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 2px;
  transition: width 0.3s ease;
}
.tf-fill-done   { background: #a6e3a1; }
.tf-fill-error  { background: var(--danger, #f38ba8); }
.tf-fill-cancel { background: rgba(255,255,255,0.15); }
.tf-fill-delete { background: var(--red, #f38ba8); }

/* Card bottom row: pct + speed + eta / status */
.tf-item-meta {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tf-pct {
  font-size: 11px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.tf-speed {
  font-size: 11px;
  font-family: monospace;
  color: var(--accent);
  flex-shrink: 0;
}

.tf-eta {
  font-size: 11px;
  color: var(--text-muted);
  flex-shrink: 0;
}

/* Status icons */
.tf-si        { font-size: 11px; flex-shrink: 0; }
.tf-si-done   { color: #a6e3a1; }
.tf-si-error  { color: var(--danger, #f38ba8); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 120px; }
.tf-si-cancel { color: var(--text-muted); }

/* Cancel / remove button */
.tf-cancel {
  flex-shrink: 0;
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 11px;
  cursor: pointer;
  padding: 1px 4px;
  border-radius: 3px;
  transition: color 0.15s, background 0.15s;
  line-height: 1;
  margin-left: auto;
}
.tf-cancel:hover { color: var(--danger, #f38ba8); background: rgba(243,139,168,0.1); }

.tf-retry {
  flex-shrink: 0;
  background: transparent;
  border: none;
  color: var(--accent);
  font-size: 13px;
  cursor: pointer;
  padding: 1px 4px;
  border-radius: 3px;
  transition: color 0.15s, background 0.15s;
  line-height: 1;
}
.tf-retry:hover { background: rgba(88,166,255,0.12); }

.tf-reveal {
  flex-shrink: 0;
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  cursor: pointer;
  padding: 1px 4px;
  border-radius: 3px;
  transition: color 0.15s, background 0.15s;
  line-height: 1;
}
.tf-reveal:hover { color: var(--text-primary); background: rgba(255,255,255,0.08); }

/* ── Enter/leave animation ── */
.tf-appear-enter-active, .tf-appear-leave-active {
  transition: max-height 0.2s ease, opacity 0.2s ease;
  overflow: hidden;
}
.tf-appear-enter-from, .tf-appear-leave-to {
  max-height: 0;
  opacity: 0;
}
.tf-appear-enter-to {
  max-height: 120px;
}
</style>
