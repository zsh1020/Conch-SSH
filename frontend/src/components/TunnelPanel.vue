<template>
  <div class="tp-root">

    <!-- ── Header ── -->
    <div class="tp-header">
      <span class="tp-title">端口转发</span>
      <div class="tp-header-actions">
        <button class="tp-add-btn" @click="openAddDialog">＋ 添加</button>
        <button class="tp-icon-btn tp-close-btn" title="关闭" @click="emit('close')">✕</button>
      </div>
    </div>

    <!-- ── Unified list ── -->
    <div class="tp-list-wrap">
      <div v-if="mergedEntries.length === 0" class="tp-empty">
        暂无转发规则，点击「＋ 添加」创建
      </div>

      <table v-else class="tp-table">
        <thead>
          <tr>
            <th>类型</th>
            <th>本地地址</th>
            <th>远端地址</th>
            <th>连接数</th>
            <th title="下载流量（远端→本地）">↓ 入</th>
            <th title="上传流量（本地→远端）">↑ 出</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="e in mergedEntries"
            :key="e.key"
            :class="{ 'row-stopped': !e.running }"
          >
            <td>
              <span class="tp-type-badge" :class="`type-${e.type}`">
                {{ typeLabel(e.type) }}
              </span>
            </td>
            <td class="tp-addr">{{ e.local_addr }}</td>
            <td class="tp-addr">{{ e.remote_addr || '—' }}</td>
            <td class="tp-conns">{{ e.running ? e.running.active_conn : '—' }}</td>
            <td class="tp-traffic">{{ e.running ? fmtBytes(e.running.bytes_in) : '—' }}</td>
            <td class="tp-traffic">{{ e.running ? fmtBytes(e.running.bytes_out) : '—' }}</td>
            <td>
              <span :class="e.running ? 'tp-status-on' : 'tp-status-off'">
                {{ e.running ? '● 运行中' : '○ 已停止' }}
              </span>
            </td>
            <td class="tp-action-cell">
              <button
                v-if="e.running"
                class="tp-stop-btn"
                @click="stopTunnel(e.running.id)"
              >停止</button>
              <button
                v-else
                class="tp-start-btn"
                @click="startEntry(e)"
              >启动</button>
              <!-- 已保存规则：编辑 + 删除 -->
              <button
                v-if="e.isSaved"
                class="tp-edit-btn"
                title="修改此规则"
                @click="openEditDialog(e)"
              >编辑</button>
              <button
                v-if="e.isSaved"
                class="tp-del-btn"
                title="从保存列表移除（不影响会话配置以外的规则）"
                @click="removeSavedRule(e)"
              >✕</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ── Add-rule dialog ── -->
    <div v-if="showDialog" class="tp-overlay" style="--wails-draggable: drag">
      <div class="tp-dialog" :style="dialogBoxStyle" style="--wails-draggable: no-drag">
        <div class="tp-dialog-header" style="cursor: grab" @mousedown.prevent="startDialogDrag">
          <span>{{ editTarget ? '修改转发规则' : '添加转发规则' }}</span>
          <button class="tp-icon-btn" style="cursor: default" @click="showDialog = false">✕</button>
        </div>

        <form class="tp-dialog-body" @submit.prevent="submitDialog">
          <!-- 预设（仅添加模式显示） -->
          <div v-if="!editTarget" class="tp-form-row">
            <label>快速预设</label>
            <div class="tp-presets">
              <button
                v-for="p in PRESETS"
                :key="p.name"
                type="button"
                class="tp-preset-btn"
                :title="p.hint"
                @click="applyPreset(p)"
              >{{ p.name }}</button>
            </div>
          </div>

          <!-- 类型 -->
          <div class="tp-form-row">
            <label>转发类型</label>
            <div class="tp-type-tabs">
              <button
                v-for="m in typeOptions"
                :key="m.value"
                type="button"
                class="tp-type-tab"
                :class="{ active: form.type === m.value }"
                @click="form.type = m.value"
              >{{ m.label }}</button>
            </div>
          </div>

          <!-- 本地地址 -->
          <div class="tp-form-row">
            <label>本地地址 <span class="req">*</span></label>
            <input v-model="form.local_addr" placeholder="127.0.0.1:18080" required />
            <span class="tp-hint">监听的本地端口，如 127.0.0.1:18080 或 :18080</span>
          </div>

          <!-- 远端地址（dynamic 隐藏） -->
          <div v-if="form.type !== 'dynamic'" class="tp-form-row">
            <label>远端地址 <span class="req">*</span></label>
            <input
              v-model="form.remote_addr"
              :placeholder="form.type === 'local' ? 'db.internal:5432' : '0.0.0.0:8080'"
              :required="form.type !== 'dynamic'"
            />
            <span class="tp-hint">
              <template v-if="form.type === 'local'">通过 SSH 转发到此远端地址</template>
              <template v-else>SSH 服务器侧监听的地址</template>
            </span>
          </div>

          <div v-if="errorMsg" class="tp-error">{{ errorMsg }}</div>

          <!-- 自动保存提示（仅添加模式 + 有 session） -->
          <div v-if="!editTarget && props.sessionId" class="tp-auto-save-hint">
            ✓ 添加后自动保存到会话，下次连接将自动启动
          </div>

          <div class="tp-dialog-footer">
            <button type="button" class="tp-btn" @click="showDialog = false">取消</button>
            <button type="submit" class="tp-btn tp-btn-primary" :disabled="adding">
              {{ adding ? (editTarget ? '保存中…' : '添加中…') : (editTarget ? '保存修改' : '添加') }}
            </button>
          </div>
        </form>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  StartTunnel, StopTunnel, ListTunnels,
  GetSessionSavedTunnels, UpdateSavedTunnels,
} from '../../wailsjs/go/main/App'
import { useDialogStore } from '../stores/dialogs'
import { useDraggable } from '../composables/useDraggable'

const { boxStyle: dialogBoxStyle, startDrag: startDialogDrag } = useDraggable()

const props = defineProps<{ connId: string; sessionId?: string }>()
const emit  = defineEmits<{ (e: 'close'): void }>()
const dlg   = useDialogStore()

// ─── Types ────────────────────────────────────────────────────────────────────

interface TunnelStatus {
  id: string
  config: { type: string; local_addr: string; remote_addr: string }
  active_conn: number
  bytes_in: number
  bytes_out: number
}

interface TunnelRule {
  type: string
  local_addr: string
  remote_addr: string
}

interface MergedEntry {
  key: string
  type: string
  local_addr: string
  remote_addr: string
  isSaved: boolean
  running: TunnelStatus | null
}

// ─── State ────────────────────────────────────────────────────────────────────

const tunnels    = ref<TunnelStatus[]>([])   // currently running
const savedRules = ref<TunnelRule[]>([])     // persisted in session config
const showDialog = ref(false)
const adding     = ref(false)
const errorMsg   = ref('')

// null = add mode; non-null = edit mode（存放被编辑的原始条目）
const editTarget = ref<MergedEntry | null>(null)

const form = ref({ type: 'local', local_addr: '', remote_addr: '' })

const typeOptions = [
  { value: 'local',   label: '本地 -L' },
  { value: 'remote',  label: '远程 -R' },
  { value: 'dynamic', label: 'SOCKS5 -D' },
]

interface Preset {
  name: string; type: string; local_addr: string; remote_addr: string; hint: string
}

const PRESETS: Preset[] = [
  { name: 'MySQL',      type: 'local',   local_addr: '127.0.0.1:3306',  remote_addr: '127.0.0.1:3306',  hint: 'MySQL 3306' },
  { name: 'PostgreSQL', type: 'local',   local_addr: '127.0.0.1:5432',  remote_addr: '127.0.0.1:5432',  hint: 'PostgreSQL 5432' },
  { name: 'Redis',      type: 'local',   local_addr: '127.0.0.1:6379',  remote_addr: '127.0.0.1:6379',  hint: 'Redis 6379' },
  { name: 'MongoDB',    type: 'local',   local_addr: '127.0.0.1:27017', remote_addr: '127.0.0.1:27017', hint: 'MongoDB 27017' },
  { name: 'K8s',        type: 'local',   local_addr: '127.0.0.1:6443',  remote_addr: '127.0.0.1:6443',  hint: 'K8s API 6443' },
  { name: 'HTTP 8080',  type: 'local',   local_addr: '127.0.0.1:18080', remote_addr: '127.0.0.1:8080',  hint: '本地 18080 → 远端 HTTP 8080' },
  { name: 'SOCKS5',     type: 'dynamic', local_addr: '127.0.0.1:1080',  remote_addr: '',                hint: 'SOCKS5 动态代理 1080' },
]

// ─── Computed: merge saved rules + running tunnels into one list ──────────────

const mergedEntries = computed<MergedEntry[]>(() => {
  const result: MergedEntry[] = []
  const usedIds = new Set<string>()

  // 1. Saved rules (always visible; may or may not be running)
  for (const rule of savedRules.value) {
    const running = tunnels.value.find(t =>
      t.config.type      === rule.type &&
      t.config.local_addr  === rule.local_addr &&
      t.config.remote_addr === rule.remote_addr
    ) ?? null
    if (running) usedIds.add(running.id)
    result.push({
      key:        `saved:${rule.type}:${rule.local_addr}:${rule.remote_addr}`,
      type:       rule.type,
      local_addr: rule.local_addr,
      remote_addr: rule.remote_addr,
      isSaved:    true,
      running,
    })
  }

  // 2. Ad-hoc running tunnels not matching any saved rule
  for (const t of tunnels.value) {
    if (!usedIds.has(t.id)) {
      result.push({
        key:        `adhoc:${t.id}`,
        type:       t.config.type,
        local_addr: t.config.local_addr,
        remote_addr: t.config.remote_addr,
        isSaved:    false,
        running:    t,
      })
    }
  }

  return result
})

// ─── Data loading ─────────────────────────────────────────────────────────────

let pollTimer: ReturnType<typeof setInterval> | null = null

async function loadTunnels() {
  try {
    tunnels.value = (await ListTunnels(props.connId)) ?? []
  } catch {
    // Connection may be closed
  }
}

async function loadSavedRules() {
  if (!props.sessionId) return
  try {
    savedRules.value = (await GetSessionSavedTunnels(props.connId)) ?? []
  } catch {
    // Ignore — ad-hoc connection or error
  }
}

// ─── Actions ──────────────────────────────────────────────────────────────────

function openAddDialog() {
  editTarget.value = null
  form.value = { type: 'local', local_addr: '', remote_addr: '' }
  errorMsg.value = ''
  showDialog.value = true
}

function openEditDialog(e: MergedEntry) {
  editTarget.value = e
  form.value = { type: e.type, local_addr: e.local_addr, remote_addr: e.remote_addr }
  errorMsg.value = ''
  showDialog.value = true
}

function applyPreset(p: Preset) {
  form.value.type        = p.type
  form.value.local_addr  = p.local_addr
  form.value.remote_addr = p.remote_addr
}

async function submitDialog() {
  errorMsg.value = ''
  adding.value = true
  const newCfg: TunnelRule = {
    type:        form.value.type,
    local_addr:  form.value.local_addr,
    remote_addr: form.value.remote_addr,
  }
  try {
    if (editTarget.value) {
      // ── 编辑模式：停旧 → 启新 → 更新保存列表 ──
      const old = editTarget.value
      if (old.running) {
        try { await StopTunnel(props.connId, old.running.id) } catch { /* ignore */ }
      }
      await StartTunnel(props.connId, newCfg)
      if (old.isSaved) {
        const newRules = savedRules.value
          .filter(r => !(r.type === old.type && r.local_addr === old.local_addr && r.remote_addr === old.remote_addr))
          .concat(newCfg)
        await UpdateSavedTunnels(props.connId, newRules)
        savedRules.value = newRules
      }
    } else {
      // ── 添加模式：启动 + 有 session 时自动保存 ──
      await StartTunnel(props.connId, newCfg)
      if (props.sessionId) {
        const dup = savedRules.value.some(r =>
          r.type === newCfg.type && r.local_addr === newCfg.local_addr && r.remote_addr === newCfg.remote_addr
        )
        if (!dup) {
          const newRules = [...savedRules.value, newCfg]
          await UpdateSavedTunnels(props.connId, newRules)
          savedRules.value = newRules
        }
      }
    }
    showDialog.value = false
    await loadTunnels()
  } catch (e: any) {
    errorMsg.value = String(e)
  } finally {
    adding.value = false
  }
}

async function stopTunnel(id: string) {
  try {
    await StopTunnel(props.connId, id)
    await loadTunnels()
  } catch (e: any) {
    dlg.error('停止失败：' + String(e))
  }
}

async function startEntry(e: MergedEntry) {
  try {
    await StartTunnel(props.connId, {
      type:        e.type,
      local_addr:  e.local_addr,
      remote_addr: e.remote_addr,
    })
    await loadTunnels()
  } catch (err: any) {
    dlg.error('启动失败：' + String(err))
  }
}

/** Remove a saved rule from the persistent list (stops it first if running). */
async function removeSavedRule(e: MergedEntry) {
  if (e.running) {
    try { await StopTunnel(props.connId, e.running.id) } catch { /* ignore */ }
  }
  const newRules = savedRules.value.filter(r =>
    !(r.type === e.type && r.local_addr === e.local_addr && r.remote_addr === e.remote_addr)
  )
  try {
    await UpdateSavedTunnels(props.connId, newRules)
    savedRules.value = newRules
    await loadTunnels()
  } catch (err: any) {
    dlg.error('删除失败：' + String(err))
  }
}


function typeLabel(t: string) {
  return t === 'local' ? 'L' : t === 'remote' ? 'R' : 'D'
}

function fmtBytes(n: number): string {
  if (n === 0) return '—'
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 * 1024 * 1024) return (n / (1024 * 1024)).toFixed(1) + ' MB'
  return (n / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
}

// ─── Lifecycle ────────────────────────────────────────────────────────────────

onMounted(async () => {
  await Promise.all([loadTunnels(), loadSavedRules()])
  pollTimer = setInterval(loadTunnels, 2000)
})

onUnmounted(() => {
  if (pollTimer !== null) clearInterval(pollTimer)
})
</script>

<style scoped>
.tp-root {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 12px;
}

/* ── Header ── */
.tp-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.tp-title { font-size: 13px; font-weight: 600; }

.tp-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tp-close-btn {
  font-size: 13px;
  padding: 3px 7px;
  color: var(--text-muted);
}


.tp-add-btn {
  background: var(--accent);
  border: none;
  border-radius: 5px;
  color: #fff;
  padding: 5px 12px;
  font-size: 12px;
  cursor: pointer;
  transition: filter 0.15s;
}
.tp-add-btn:hover { filter: brightness(1.1); }

/* ── List ── */
.tp-list-wrap {
  flex: 1;
  overflow-y: auto;
  padding: 12px 16px;
}

.tp-empty {
  padding: 40px;
  text-align: center;
  color: var(--text-muted);
}

.tp-table {
  width: 100%;
  border-collapse: collapse;
}

.tp-table th {
  text-align: left;
  padding: 6px 10px;
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.4px;
  border-bottom: 1px solid var(--border);
}

.tp-table td {
  padding: 8px 10px;
  border-bottom: 1px solid rgba(255,255,255,0.04);
  vertical-align: middle;
}

/* Dimmed row for stopped tunnels */
.row-stopped td { opacity: 0.6; }
.row-stopped .tp-action-cell { opacity: 1; }

/* ── Type badge ── */
.tp-type-badge {
  display: inline-block;
  width: 20px;
  text-align: center;
  border-radius: 3px;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 0;
}
.type-local   { background: rgba(88,166,255,0.15); color: #58a6ff; }
.type-remote  { background: rgba(63,185,80,0.15);  color: #3fb950; }
.type-dynamic { background: rgba(255,166,0,0.15);  color: #ffa600; }

.tp-addr { font-family: monospace; font-size: 12px; }

.tp-conns { text-align: center; color: var(--text-muted); }

.tp-traffic {
  text-align: right;
  font-family: monospace;
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  min-width: 52px;
}

/* ── Status ── */
.tp-status-on  { color: #3fb950; font-size: 11px; }
.tp-status-off { color: var(--text-muted); font-size: 11px; }

/* ── Action cell buttons ── */
.tp-action-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tp-start-btn {
  background: transparent;
  border: 1px solid rgba(63,185,80,0.4);
  color: #3fb950;
  border-radius: 4px;
  padding: 3px 10px;
  font-size: 11px;
  cursor: pointer;
  transition: background 0.15s;
}
.tp-start-btn:hover { background: rgba(63,185,80,0.1); }

.tp-stop-btn {
  background: transparent;
  border: 1px solid rgba(248,81,73,0.4);
  color: var(--danger);
  border-radius: 4px;
  padding: 3px 10px;
  font-size: 11px;
  cursor: pointer;
  transition: background 0.15s;
}
.tp-stop-btn:hover { background: rgba(248,81,73,0.1); }

.tp-edit-btn {
  background: transparent;
  border: 1px solid rgba(88,166,255,0.35);
  color: var(--accent);
  border-radius: 4px;
  padding: 3px 8px;
  font-size: 11px;
  cursor: pointer;
  transition: background 0.15s;
}
.tp-edit-btn:hover { background: rgba(88,166,255,0.1); }

.tp-del-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 11px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 3px;
  line-height: 1;
  transition: color 0.15s, background 0.15s;
}
.tp-del-btn:hover { color: var(--danger); background: rgba(248,81,73,0.1); }

.tp-auto-save-hint {
  font-size: 11px;
  color: #3fb950;
  background: rgba(63,185,80,0.08);
  border: 1px solid rgba(63,185,80,0.25);
  border-radius: 5px;
  padding: 6px 10px;
}

/* ── Add-rule dialog ── */
.tp-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0,0,0,0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 50;
  backdrop-filter: blur(2px);
}

.tp-dialog {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 420px;
  box-shadow: 0 20px 40px rgba(0,0,0,0.6);
}

.tp-dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px 10px;
  border-bottom: 1px solid var(--border);
  font-size: 13px;
  font-weight: 600;
}

.tp-icon-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
  padding: 2px 5px;
  border-radius: 3px;
}
.tp-icon-btn:hover { color: var(--text-primary); background: rgba(255,255,255,0.06); }

.tp-dialog-body {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.tp-form-row {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.tp-form-row label {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.tp-form-row input {
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 6px 10px;
  outline: none;
  transition: border-color 0.15s;
}
.tp-form-row input:focus { border-color: var(--accent); }

.req   { color: var(--danger); }
.tp-hint { font-size: 11px; color: var(--text-muted); }

.tp-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 4px 0;
}

.tp-preset-btn {
  background: rgba(255,255,255,0.04);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-muted);
  font-size: 11px;
  padding: 4px 10px;
  cursor: pointer;
  transition: all 0.15s;
}
.tp-preset-btn:hover {
  background: rgba(88,166,255,0.1);
  border-color: rgba(88,166,255,0.4);
  color: var(--accent);
}

.tp-type-tabs {
  display: flex;
  gap: 4px;
  background: var(--bg-primary);
  border-radius: 6px;
  padding: 3px;
}

.tp-type-tab {
  flex: 1;
  background: transparent;
  border: none;
  border-radius: 4px;
  padding: 5px 8px;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
}
.tp-type-tab.active {
  background: var(--bg-secondary);
  color: var(--text-primary);
  box-shadow: 0 1px 3px rgba(0,0,0,0.3);
}

.tp-error {
  background: rgba(248,81,73,0.1);
  border: 1px solid rgba(248,81,73,0.3);
  border-radius: 6px;
  padding: 8px 12px;
  color: var(--danger);
  font-size: 12px;
}

.tp-dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 4px;
}

.tp-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 5px 16px;
  font-size: 12px;
  color: var(--text-primary);
  cursor: pointer;
  transition: background 0.15s;
}
.tp-btn:hover { background: rgba(255,255,255,0.06); }

.tp-btn-primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.tp-btn-primary:hover:not(:disabled) { filter: brightness(1.1); }
.tp-btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
