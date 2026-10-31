<template>
  <div class="sp-overlay" />

  <div class="sp-panel">
    <div class="sp-header">
      <span class="sp-title">命令片段</span>
      <div class="sp-header-actions">
        <button class="sp-manage-btn" @click="$emit('manage')">管理</button>
        <button class="sp-close-btn" @click="$emit('close')">✕</button>
      </div>
    </div>

    <div class="sp-body">
      <div v-if="snippetsStore.snippets.length === 0" class="sp-empty">
        暂无片段，点击「管理」添加
      </div>
      <template v-for="(items, group) in snippetsStore.byGroup()" :key="group">
        <div class="sp-group-label">{{ group }}</div>
        <div
          v-for="sn in items"
          :key="sn.id"
          class="sp-item"
          :title="sn.command"
          @click="handleSend(sn)"
        >
          <div class="sp-item-info">
            <span class="sp-name">{{ sn.name }}</span>
            <span class="sp-cmd">{{ sn.command }}</span>
          </div>
          <div class="sp-item-tags">
            <span v-if="hasParams(sn.command)" class="sp-param-badge" title="含参数占位符">{…}</span>
          </div>
          <button class="sp-send-btn" @click.stop="handleSend(sn)">▶</button>
        </div>
      </template>
    </div>

    <!-- 参数填写弹窗（内嵌在面板内，绝对定位覆盖） -->
    <Transition name="param-slide">
      <div v-if="paramDialog" class="param-overlay">
        <div class="param-box">
          <div class="param-header">
            <span class="param-title">填写参数</span>
            <button class="param-close" @click="cancelParam">✕</button>
          </div>
          <div class="param-preview">{{ paramDialog.command }}</div>
          <div class="param-fields">
            <div
              v-for="p in paramDialog.params"
              :key="p.name"
              class="param-field"
            >
              <label class="param-label">{{ p.name }}</label>
              <input
                v-model="paramDialog.values[p.name]"
                class="param-input"
                :placeholder="p.defaultValue || p.name"
                @keydown.enter="confirmParam"
                @keydown.esc="cancelParam"
              />
            </div>
          </div>
          <div class="param-actions">
            <button class="param-btn" @click="cancelParam">取消</button>
            <button class="param-btn primary" @click="confirmParam">发送 ▶</button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, onMounted } from 'vue'
import { SendInput } from '../../wailsjs/go/main/App'
import { useSnippetsStore, type Snippet } from '../stores/snippets'

const props = defineProps<{ connId: string }>()
const emit  = defineEmits<{ close: []; manage: [] }>()

const snippetsStore = useSnippetsStore()

interface ParamDef { name: string; defaultValue: string }
interface ParamDialog {
  command: string
  params: ParamDef[]
  values: Record<string, string>
}

const paramDialog = ref<ParamDialog | null>(null)

onMounted(() => snippetsStore.fetchAll())

// ── 占位符解析 ────────────────────────────────────────────────────────────
const PARAM_RE = /\{(\w+)(?::([^}]*))?\}/g

function hasParams(cmd: string): boolean {
  PARAM_RE.lastIndex = 0
  return PARAM_RE.test(cmd)
}

function parseParams(cmd: string): ParamDef[] {
  const seen = new Set<string>()
  const result: ParamDef[] = []
  const re = /\{(\w+)(?::([^}]*))?\}/g
  let m: RegExpExecArray | null
  while ((m = re.exec(cmd)) !== null) {
    const name = m[1]
    if (!seen.has(name)) {
      seen.add(name)
      result.push({ name, defaultValue: m[2] ?? '' })
    }
  }
  return result
}

function fillParams(cmd: string, values: Record<string, string>): string {
  return cmd.replace(/\{(\w+)(?::[^}]*)?\}/g, (_, name) => values[name] ?? '')
}

// ── 发送逻辑 ─────────────────────────────────────────────────────────────
function handleSend(sn: Snippet) {
  const params = parseParams(sn.command)
  if (params.length === 0) {
    // 无参数：直接发送
    send(sn.command)
    return
  }
  // 有参数：弹出填写框，预填默认值
  const values: Record<string, string> = {}
  for (const p of params) values[p.name] = p.defaultValue
  paramDialog.value = { command: sn.command, params, values }
  // 自动聚焦第一个输入框
  nextTick(() => {
    const el = document.querySelector<HTMLInputElement>('.param-input')
    el?.focus()
  })
}

function confirmParam() {
  if (!paramDialog.value) return
  const cmd = fillParams(paramDialog.value.command, paramDialog.value.values)
  paramDialog.value = null
  send(cmd)
}

function cancelParam() {
  paramDialog.value = null
}

function send(command: string) {
  SendInput(props.connId, command + '\n').catch(() => {})
  emit('close')
}
</script>

<style scoped>
.sp-overlay {
  position: fixed;
  inset: 0;
  z-index: 190;
}

.sp-panel {
  position: absolute;
  top: 100%;
  right: 0;
  z-index: 200;
  width: 300px;
  max-height: 420px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.5);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sp-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.sp-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.sp-header-actions {
  display: flex;
  gap: 4px;
  align-items: center;
}

.sp-manage-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--accent);
  font-size: 11px;
  padding: 2px 8px;
  cursor: pointer;
  transition: background 0.15s;
}
.sp-manage-btn:hover { background: rgba(88,166,255,0.1); }

.sp-close-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 11px;
  padding: 2px 4px;
  cursor: pointer;
  border-radius: 3px;
}
.sp-close-btn:hover { color: var(--text-primary); background: rgba(255,255,255,0.06); }

.sp-body {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}

.sp-empty {
  padding: 20px;
  text-align: center;
  font-size: 12px;
  color: var(--text-muted);
}

.sp-group-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.4px;
  padding: 6px 10px 2px;
}

.sp-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  cursor: pointer;
  transition: background 0.1s;
}
.sp-item:hover { background: rgba(255,255,255,0.05); }
.sp-item:hover .sp-send-btn { opacity: 1; }

.sp-item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sp-name {
  font-size: 12px;
  color: var(--text-primary);
  flex-shrink: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sp-cmd {
  font-size: 11px;
  color: var(--text-muted);
  font-family: monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sp-item-tags { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }

/* 参数标记徽章 */
.sp-param-badge {
  font-size: 9px;
  font-family: monospace;
  background: rgba(88,166,255,0.15);
  color: var(--accent);
  border: 1px solid rgba(88,166,255,0.3);
  border-radius: 3px;
  padding: 1px 4px;
  line-height: 1.4;
}

.sp-send-btn {
  background: var(--accent);
  border: none;
  border-radius: 3px;
  color: #fff;
  font-size: 10px;
  padding: 2px 6px;
  cursor: pointer;
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.15s;
}
.sp-send-btn:hover { filter: brightness(1.15); }

/* ── 参数弹窗 ── */
.param-overlay {
  position: absolute;
  inset: 0;
  z-index: 10;
  background: rgba(13,17,23,0.75);
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(2px);
}

.param-box {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 8px;
  width: 270px;
  box-shadow: 0 12px 32px rgba(0,0,0,0.6);
  display: flex;
  flex-direction: column;
  gap: 0;
  overflow: hidden;
}

.param-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px 8px;
  border-bottom: 1px solid var(--border);
}

.param-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
}

.param-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 11px;
  padding: 1px 4px;
  cursor: pointer;
  border-radius: 3px;
}
.param-close:hover { color: var(--text-primary); }

.param-preview {
  padding: 8px 12px;
  font-size: 11px;
  font-family: monospace;
  color: var(--text-muted);
  background: rgba(0,0,0,0.2);
  border-bottom: 1px solid var(--border);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.param-fields {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
}

.param-field {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.param-label {
  font-size: 10px;
  color: var(--text-muted);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.param-input {
  font-size: 12px;
  padding: 5px 8px;
  border-radius: 5px;
  border: 1px solid var(--border);
  background: var(--bg-primary);
  color: var(--text-primary);
  outline: none;
  width: 100%;
}
.param-input:focus { border-color: var(--accent); }

.param-actions {
  display: flex;
  gap: 6px;
  justify-content: flex-end;
  padding: 8px 12px 10px;
  border-top: 1px solid var(--border);
}

.param-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 5px 14px;
  font-size: 12px;
  color: var(--text-primary);
  cursor: pointer;
  transition: background 0.15s;
}
.param-btn:hover { background: rgba(255,255,255,0.06); }
.param-btn.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.param-btn.primary:hover { filter: brightness(1.1); }

/* 动画 */
.param-slide-enter-active, .param-slide-leave-active { transition: opacity 0.15s, transform 0.15s; }
.param-slide-enter-from, .param-slide-leave-to { opacity: 0; transform: scale(0.96); }
</style>
