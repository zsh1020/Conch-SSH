<template>
  <div class="sd-overlay" style="--wails-draggable: drag">
    <div class="sd-dialog" :style="boxStyle" style="--wails-draggable: no-drag">

      <div class="sd-header" style="cursor: grab" @mousedown.prevent="startDrag">
        <span class="sd-title">命令片段管理</span>
        <button class="sd-close" style="cursor: default" @click="$emit('close')">✕</button>
      </div>

      <div class="sd-body">
        <!-- 左：片段列表 -->
        <div class="sd-list">
          <div class="sd-list-header">
            <span>片段列表</span>
            <button class="sd-add-btn" @click="newSnippet">＋ 新建</button>
          </div>
          <div v-if="snippetsStore.snippets.length === 0" class="sd-empty">
            暂无片段
          </div>
          <div
            v-for="sn in snippetsStore.snippets"
            :key="sn.id"
            class="sd-list-item"
            :class="{ active: editing?.id === sn.id }"
            @click="editSnippet(sn)"
          >
            <div class="sn-name">{{ sn.name }}</div>
            <div class="sn-cmd">{{ sn.command }}</div>
          </div>
        </div>

        <!-- 右：编辑表单 -->
        <div class="sd-form">
          <template v-if="editing">
            <div class="sd-field">
              <label>名称</label>
              <input v-model="editing.name" placeholder="片段名称" />
            </div>
            <div class="sd-field">
              <label>分组（可选）</label>
              <input v-model="editing.group" placeholder="默认" />
            </div>
            <div class="sd-field">
              <label>命令</label>
              <textarea v-model="editing.command" rows="5" placeholder="输入命令…" class="sd-textarea" />
            </div>
            <div class="sd-form-actions">
              <button class="sd-btn sd-btn-primary" :disabled="saving" @click="saveEditing">
                {{ saving ? '保存中…' : '保存' }}
              </button>
              <button
                v-if="editing.id"
                class="sd-btn sd-btn-danger"
                @click="deleteEditing"
              >删除</button>
              <button class="sd-btn" @click="editing = null">取消</button>
            </div>
            <div v-if="errMsg" class="sd-error">{{ errMsg }}</div>
          </template>
          <div v-else class="sd-hint">
            从左侧选择片段或点击「＋ 新建」
          </div>
        </div>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useSnippetsStore, type Snippet } from '../stores/snippets'
import { useDraggable } from '../composables/useDraggable'

const { boxStyle, startDrag } = useDraggable()
import { useDialogStore } from '../stores/dialogs'

const emit = defineEmits<{ close: [] }>()

const snippetsStore = useSnippetsStore()
const dlg           = useDialogStore()
const editing = ref<Snippet | null>(null)
const saving  = ref(false)
const errMsg  = ref('')

onMounted(() => snippetsStore.fetchAll())

function newSnippet() {
  editing.value = { id: '', name: '', command: '', group: '' }
  errMsg.value = ''
}

function editSnippet(sn: Snippet) {
  editing.value = { ...sn }
  errMsg.value = ''
}

async function saveEditing() {
  if (!editing.value) return
  if (!editing.value.name.trim()) { errMsg.value = '名称不能为空'; return }
  if (!editing.value.command.trim()) { errMsg.value = '命令不能为空'; return }
  saving.value = true
  errMsg.value = ''
  try {
    await snippetsStore.save({ ...editing.value })
    editing.value = null
  } catch (e: any) {
    errMsg.value = String(e)
  } finally {
    saving.value = false
  }
}

async function deleteEditing() {
  if (!editing.value?.id) return
  const ok = await dlg.confirm(`确定删除「${editing.value.name}」？`, '删除片段')
  if (!ok) return
  try {
    await snippetsStore.remove(editing.value.id)
    editing.value = null
  } catch (e: any) {
    errMsg.value = String(e)
  }
}
</script>

<style scoped>
.sd-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.65);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 300;
  backdrop-filter: blur(2px);
}

.sd-dialog {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 620px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 24px 48px rgba(0,0,0,0.6);
}

.sd-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.sd-title { font-size: 13px; font-weight: 600; }

.sd-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  padding: 2px 5px;
  border-radius: 3px;
  cursor: pointer;
}
.sd-close:hover { color: var(--text-primary); background: rgba(255,255,255,0.06); }

.sd-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* ── List ── */
.sd-list {
  width: 220px;
  flex-shrink: 0;
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sd-list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
  font-size: 11px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.sd-add-btn {
  background: var(--accent);
  border: none;
  border-radius: 4px;
  color: #fff;
  font-size: 11px;
  padding: 3px 8px;
  cursor: pointer;
}
.sd-add-btn:hover { filter: brightness(1.1); }

.sd-empty {
  padding: 20px;
  text-align: center;
  font-size: 12px;
  color: var(--text-muted);
}

.sd-list-item {
  padding: 8px 10px;
  cursor: pointer;
  border-bottom: 1px solid rgba(255,255,255,0.04);
  transition: background 0.1s;
}
.sd-list-item:hover { background: rgba(255,255,255,0.04); }
.sd-list-item.active { background: rgba(88,166,255,0.1); }

.sn-name { font-size: 12px; color: var(--text-primary); font-weight: 500; }
.sn-cmd {
  font-size: 11px;
  color: var(--text-muted);
  font-family: monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 2px;
}

/* ── Form ── */
.sd-form {
  flex: 1;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
}

.sd-hint {
  color: var(--text-muted);
  font-size: 12px;
  text-align: center;
  margin-top: 60px;
}

.sd-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.sd-field label {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 500;
}

.sd-textarea {
  resize: vertical;
  font-family: monospace;
  font-size: 12px;
  min-height: 80px;
}

.sd-form-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.sd-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 5px 14px;
  font-size: 12px;
  color: var(--text-primary);
  cursor: pointer;
  transition: background 0.15s;
}
.sd-btn:hover:not(:disabled) { background: rgba(255,255,255,0.06); }
.sd-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.sd-btn-primary { background: var(--accent); border-color: var(--accent); color: #fff; }
.sd-btn-primary:hover:not(:disabled) { filter: brightness(1.1); }

.sd-btn-danger { border-color: rgba(248,81,73,0.5); color: var(--danger); }
.sd-btn-danger:hover { background: rgba(248,81,73,0.1) !important; }

.sd-error {
  background: rgba(248,81,73,0.1);
  border: 1px solid rgba(248,81,73,0.3);
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 12px;
  color: var(--danger);
}
</style>
