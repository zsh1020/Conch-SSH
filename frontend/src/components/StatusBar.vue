<template>
  <!-- ─── 快捷命令栏（置于标签栏下方） ─── -->
  <div class="qc-bar">
    <span class="qc-label"><i class="ph ph-lightning"></i>快捷命令</span>
    <span class="qc-divider"></span>
    <div class="qc-chips">
      <button
        v-for="sn in snippetsStore.snippets"
        :key="sn.id"
        class="qc-chip"
        :class="{ dashed: sbHasParams(sn.command) }"
        :title="sn.command"
        @click="sbQuickSend(sn)"
      >{{ sn.name }}</button>
      <span v-if="snippetsStore.snippets.length === 0" class="qc-empty">暂无片段，点右侧 + 添加</span>
      <button class="qc-add" title="管理命令片段" @click="$emit('manage')"><i class="ph ph-plus"></i></button>
    </div>
    <span class="qc-encoding mono"><i class="ph ph-text-aa"></i>UTF-8</span>
  </div>

  <!-- 快捷命令参数弹窗 -->
  <Transition name="sb-param-fade">
    <div v-if="sbParamDialog" class="sb-param-overlay" @click.self="sbParamDialog = null">
      <div class="sb-param-box">
        <div class="sb-param-header">
          <span class="sb-param-title">填写参数</span>
          <button class="sb-param-close" @click="sbParamDialog = null"><i class="ph ph-x"></i></button>
        </div>
        <div class="sb-param-preview mono">{{ sbParamDialog.command }}</div>
        <div class="sb-param-fields">
          <div v-for="p in sbParamDialog.params" :key="p.name" class="sb-param-field">
            <label class="sb-param-label">{{ p.name }}</label>
            <input
              v-model="sbParamDialog.values[p.name]"
              class="sb-param-input"
              :placeholder="p.defaultValue || p.name"
              @keydown.enter="sbConfirmParam"
              @keydown.esc="sbParamDialog = null"
            />
          </div>
        </div>
        <div class="sb-param-actions">
          <button class="sb-param-btn" @click="sbParamDialog = null">取消</button>
          <button class="sb-param-btn primary" @click="sbConfirmParam">填入 ▶</button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { SendInput } from '../../wailsjs/go/main/App'
import { useTabsStore } from '../stores/tabs'
import { useSnippetsStore, type Snippet } from '../stores/snippets'

defineEmits<{ (e: 'manage'): void }>()

const tabsStore     = useTabsStore()
const snippetsStore = useSnippetsStore()

const SB_PARAM_RE = /\{(\w+)(?::([^}]*))?\}/g
interface SbParamDef { name: string; defaultValue: string }
interface SbParamDialog { command: string; params: SbParamDef[]; values: Record<string, string> }
const sbParamDialog = ref<SbParamDialog | null>(null)

function sbHasParams(cmd: string): boolean {
  SB_PARAM_RE.lastIndex = 0
  return SB_PARAM_RE.test(cmd)
}
function sbParseParams(cmd: string): SbParamDef[] {
  const seen = new Set<string>()
  const result: SbParamDef[] = []
  const re = /\{(\w+)(?::([^}]*))?\}/g
  let m: RegExpExecArray | null
  while ((m = re.exec(cmd)) !== null) {
    if (!seen.has(m[1])) { seen.add(m[1]); result.push({ name: m[1], defaultValue: m[2] ?? '' }) }
  }
  return result
}
function sbFillParams(cmd: string, values: Record<string, string>): string {
  return cmd.replace(/\{(\w+)(?::[^}]*)?\}/g, (_, name) => values[name] ?? '')
}
function sbQuickSend(sn: Snippet) {
  const connId = tabsStore.activeConnId
  if (!connId) return
  const params = sbParseParams(sn.command)
  if (params.length === 0) {
    SendInput(connId, sn.command).catch(() => {})
    window.dispatchEvent(new CustomEvent('gssh:focus-terminal', { detail: connId }))
    return
  }
  const values: Record<string, string> = {}
  for (const p of params) values[p.name] = p.defaultValue
  sbParamDialog.value = { command: sn.command, params, values }
  nextTick(() => document.querySelector<HTMLInputElement>('.sb-param-input')?.focus())
}
function sbConfirmParam() {
  if (!sbParamDialog.value) return
  const connId = tabsStore.activeConnId
  if (!connId) { sbParamDialog.value = null; return }
  const cmd = sbFillParams(sbParamDialog.value.command, sbParamDialog.value.values)
  sbParamDialog.value = null
  SendInput(connId, cmd).catch(() => {})
  window.dispatchEvent(new CustomEvent('gssh:focus-terminal', { detail: connId }))
}
</script>

<style>
.qc-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 40px;
  flex: 0 0 40px;
  padding: 0 14px;
  background: rgba(255,255,255,0.012);
  border-bottom: 1px solid var(--line);
  overflow: hidden;
  font-family: 'IBM Plex Sans', 'Segoe UI', system-ui, sans-serif;
}
.qc-bar .mono { font-family: 'IBM Plex Mono', 'Roboto Mono', monospace; }

.qc-label {
  display: flex; align-items: center; gap: 5px;
  font-size: 11px; color: var(--tm); font-weight: 600; letter-spacing: 0.5px;
  flex-shrink: 0;
}
.qc-label i { font-size: 13px; color: var(--amber); }
.qc-divider { width: 1px; height: 16px; background: var(--line); flex-shrink: 0; }

.qc-chips {
  display: flex; align-items: center; gap: 6px;
  flex: 1; min-width: 0; height: 100%;
  overflow-x: auto; overflow-y: hidden;
  scrollbar-width: none;
}
.qc-chips::-webkit-scrollbar { display: none; }

.qc-chip {
  display: inline-flex; align-items: center; gap: 5px;
  height: 26px; padding: 0 11px; flex-shrink: 0;
  border-radius: 6px; background: var(--panel); border: 1px solid var(--line);
  color: var(--ts); font-size: 12px; cursor: pointer; white-space: nowrap;
  font-family: 'IBM Plex Sans', 'Segoe UI', sans-serif;
  transition: color 0.15s, border-color 0.15s;
}
.qc-chip:hover { color: var(--accent); border-color: var(--accent); }
.qc-chip.dashed { border-style: dashed; }

.qc-add {
  width: 26px; height: 26px; flex-shrink: 0;
  border-radius: 6px; border: 1px dashed var(--line); background: transparent;
  color: var(--tm); cursor: pointer; display: flex; align-items: center; justify-content: center;
  font-size: 13px; transition: color 0.15s, border-color 0.15s;
}
.qc-add:hover { color: var(--accent); border-color: var(--accent); }

.qc-empty { font-size: 11px; color: var(--tm); white-space: nowrap; }

.qc-encoding {
  display: flex; align-items: center; gap: 5px;
  font-size: 11px; color: var(--tm); flex-shrink: 0;
}
.qc-encoding i { font-size: 13px; }

/* ─── 参数弹窗 ─── */
.sb-param-overlay {
  position: fixed; inset: 0; z-index: 350;
  background: rgba(11,14,19,0.6);
  display: flex; align-items: center; justify-content: center;
  backdrop-filter: blur(2px);
  font-family: 'IBM Plex Sans', 'Segoe UI', sans-serif;
}
.sb-param-box {
  background: var(--panel); border: 1px solid var(--line); border-radius: 10px;
  width: 340px; box-shadow: 0 24px 48px rgba(0,0,0,0.5); overflow: hidden;
}
.sb-param-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 11px 14px 9px; border-bottom: 1px solid var(--line);
}
.sb-param-title { font-size: 12px; font-weight: 600; color: var(--tp); }
.sb-param-close { background: transparent; border: none; color: var(--tm); cursor: pointer; padding: 2px; display: flex; }
.sb-param-close:hover { color: var(--tp); }
.sb-param-preview {
  padding: 8px 14px; font-size: 11.5px; color: var(--ts);
  background: var(--term); border-bottom: 1px solid var(--line);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.sb-param-fields { display: flex; flex-direction: column; gap: 9px; padding: 12px 14px; }
.sb-param-field { display: flex; flex-direction: column; gap: 4px; }
.sb-param-label { font-size: 10px; color: var(--ts); font-weight: 600; text-transform: uppercase; letter-spacing: 0.3px; }
.sb-param-input {
  font-size: 12px; padding: 6px 9px; border-radius: 6px; border: 1px solid var(--line);
  background: var(--panel-2); color: var(--tp); outline: none;
}
.sb-param-input:focus { border-color: var(--accent); }
.sb-param-actions {
  display: flex; gap: 6px; justify-content: flex-end;
  padding: 9px 14px 12px; border-top: 1px solid var(--line);
}
.sb-param-btn {
  background: transparent; border: 1px solid var(--line); border-radius: 6px;
  padding: 6px 14px; font-size: 12px; color: var(--tp); cursor: pointer;
  transition: background 0.15s;
}
.sb-param-btn:hover { background: var(--hover); }
.sb-param-btn.primary { background: var(--accent); border-color: var(--accent); color: #06121a; font-weight: 600; }
.sb-param-btn.primary:hover { filter: brightness(1.08); }

.sb-param-fade-enter-active, .sb-param-fade-leave-active { transition: opacity 0.15s, transform 0.15s; }
.sb-param-fade-enter-from, .sb-param-fade-leave-to { opacity: 0; transform: translateY(8px); }
</style>
