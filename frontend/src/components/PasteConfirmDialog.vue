<template>
  <Teleport to="body">
    <div v-if="visible" class="pcd-overlay" @mousedown.self="cancel">
      <div class="pcd-dialog">

        <div class="pcd-header">
          <span class="pcd-icon">⚠</span>
          <span class="pcd-title">多行粘贴</span>
          <span class="pcd-lines">{{ lineCount }} 行</span>
          <button class="pcd-close" @click="cancel">✕</button>
        </div>

        <p class="pcd-hint">可在下方编辑内容，确认后发送到终端：</p>

        <textarea
          ref="textareaRef"
          v-model="editedText"
          class="pcd-textarea"
          spellcheck="false"
          @keydown="onKeydown"
          @input="autoResize"
        />

        <div class="pcd-footer">
          <button class="pcd-btn" @click="cancel">取消</button>
          <button class="pcd-btn primary" @click="doConfirm">
            发送 <kbd>Ctrl+Enter</kbd>
          </button>
        </div>

      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'

const visible     = ref(false)
const editedText  = ref('')
const textareaRef = ref<HTMLTextAreaElement>()

let resolveFn: ((v: string | null) => void) | null = null

/** 标题栏显示的非空行数（随编辑实时更新） */
const lineCount = computed(() =>
  editedText.value.split('\n').filter(l => l.trim() !== '').length
)

/** 让 textarea 高度随内容自动伸缩，最高 300px */
function autoResize() {
  const el = textareaRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 300) + 'px'
}

/**
 * 打开弹窗，返回用户编辑后的文本（null = 取消）。
 * 调用方 await 此 Promise 即可。
 */
function open(text: string): Promise<string | null> {
  editedText.value = text
  visible.value = true
  nextTick(() => {
    autoResize()
    const el = textareaRef.value
    if (el) {
      el.focus()
      el.setSelectionRange(text.length, text.length)
    }
  })
  return new Promise(resolve => { resolveFn = resolve })
}

function doConfirm() {
  visible.value = false
  resolveFn?.(editedText.value)
  resolveFn = null
}

function cancel() {
  visible.value = false
  resolveFn?.(null)
  resolveFn = null
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    cancel()
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    e.preventDefault()
    doConfirm()
  }
}

defineExpose({ open })
</script>

<style scoped>
.pcd-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(2px);
}

.pcd-dialog {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 20px 24px 18px;
  width: 520px;
  max-width: calc(100vw - 48px);
  box-shadow: 0 20px 48px rgba(0, 0, 0, 0.65);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* ── Header ── */
.pcd-header {
  display: flex;
  align-items: center;
  gap: 7px;
}

.pcd-icon {
  font-size: 15px;
}

.pcd-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
}

.pcd-lines {
  font-size: 11px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 1px 8px;
}

.pcd-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 13px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 4px;
  line-height: 1;
}
.pcd-close:hover { color: var(--text-primary); background: rgba(255,255,255,0.07); }

/* ── Hint ── */
.pcd-hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

/* ── Textarea ── */
.pcd-textarea {
  width: 100%;
  box-sizing: border-box;
  min-height: 80px;
  max-height: 300px;
  resize: vertical;
  background: var(--bg-panel, #161b22);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-primary);
  font-family: 'Cascadia Code', Consolas, 'Courier New', monospace;
  font-size: 12.5px;
  line-height: 1.55;
  padding: 8px 10px;
  outline: none;
  overflow-y: auto;
  tab-size: 4;
}
.pcd-textarea:focus { border-color: var(--accent); }

/* ── Footer ── */
.pcd-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 2px;
}

.pcd-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 6px 14px;
  cursor: pointer;
  transition: background 0.15s;
  display: flex;
  align-items: center;
  gap: 6px;
}
.pcd-btn:hover { background: rgba(255, 255, 255, 0.07); }

.pcd-btn.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.pcd-btn.primary:hover { filter: brightness(1.1); }

.pcd-btn kbd {
  font-family: inherit;
  font-size: 10px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 3px;
  padding: 1px 5px;
  border: none;
  color: rgba(255, 255, 255, 0.8);
}
</style>
