<template>
  <Teleport to="body">
    <TransitionGroup name="dlg-fade">
      <div
        v-for="dlg in store.stack"
        :key="dlg.id"
        class="dlg-overlay"
        style="--wails-draggable: drag"
        @keydown.esc.stop="onCancel(dlg)"
      >
        <div
          class="dlg-box"
          :class="['dlg-' + dlg.kind]"
          role="dialog"
          :aria-modal="true"
          style="--wails-draggable: no-drag"
          :style="getDragStyle(dlg.id)"
        >
          <!-- 左侧图标（同时作为拖拽手柄） -->
          <div class="dlg-icon-wrap" style="cursor:grab" @mousedown.prevent="startDlgDrag($event, dlg.id)">
            <span class="dlg-icon">{{ icons[dlg.kind] }}</span>
          </div>

          <!-- 右侧内容 -->
          <div class="dlg-content">
            <div class="dlg-title">{{ dlg.title }}</div>
            <div class="dlg-message">{{ dlg.message }}</div>

            <!-- prompt 输入框 -->
            <input
              v-if="dlg.kind === 'prompt'"
              :ref="el => registerPromptInput(dlg.id, el)"
              v-model="promptValues[dlg.id]"
              :type="dlg.secret ? 'password' : 'text'"
              class="dlg-input"
              :placeholder="dlg.placeholder"
              :autocomplete="dlg.secret ? 'off' : undefined"
              spellcheck="false"
              @keydown.enter.prevent="onConfirm(dlg)"
              @keydown.esc.prevent="onCancel(dlg)"
            />

            <!-- 按钮 -->
            <div class="dlg-actions">
              <button
                v-if="dlg.kind === 'confirm' || dlg.kind === 'prompt'"
                class="dlg-btn dlg-btn-ghost"
                @click="onCancel(dlg)"
              >取消</button>

              <button
                class="dlg-btn"
                :class="dlg.kind === 'error' ? 'dlg-btn-danger' : 'dlg-btn-primary'"
                @click="onConfirm(dlg)"
              >
                {{ dlg.kind === 'confirm' || dlg.kind === 'prompt' ? '确定' : '好的' }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </TransitionGroup>
  </Teleport>
</template>

<script setup lang="ts">
import { reactive, nextTick, watch, watchEffect } from 'vue'
import { useDialogStore, type DialogItem } from '../stores/dialogs'

const store = useDialogStore()

// ─── Per-dialog drag state ─────────────────────────────────────────────────

const dragPos = reactive<Record<number, { dx: number; dy: number }>>({})

function getDragStyle(id: number) {
  const p = dragPos[id]
  return p ? { transform: `translate(${p.dx}px, ${p.dy}px)` } : {}
}

function startDlgDrag(e: MouseEvent, id: number) {
  if (!dragPos[id]) dragPos[id] = { dx: 0, dy: 0 }
  const ox = e.clientX, oy = e.clientY
  const bdx = dragPos[id].dx, bdy = dragPos[id].dy
  const onMove = (ev: MouseEvent) => {
    dragPos[id].dx = bdx + (ev.clientX - ox)
    dragPos[id].dy = bdy + (ev.clientY - oy)
  }
  const onUp = () => {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
  e.preventDefault()
}

const icons: Record<string, string> = {
  info:    'ℹ️',
  error:   '⚠️',
  confirm: '❓',
  prompt:  '✏️',
}

// 每个 prompt 对话框的临时输入值
const promptValues = reactive<Record<number, string>>({})

// 在渲染前预填 defaultValue，确保 v-model 初始值正确
watchEffect(() => {
  for (const dlg of store.stack) {
    if (dlg.kind === 'prompt' && !(dlg.id in promptValues)) {
      promptValues[dlg.id] = dlg.defaultValue ?? ''
    }
  }
})

const promptInputs = new Map<number, HTMLInputElement>()
// 已完成首次「聚焦 + 选区」初始化的对话框 id。
// 注意：模板用的是内联函数 ref，Vue 每次重渲染（每次输入）都会调用 registerPromptInput，
// 若不加此守卫，focus/选区会在每次按键后重跑 → 输入一个字符就把整个文件名重新选中，
// 下一个字符覆盖全部。故初始化只做一次。
const initializedIds = new Set<number>()

function registerPromptInput(id: number, el: unknown) {
  if (el instanceof HTMLInputElement) {
    promptInputs.set(id, el)
    if (initializedIds.has(id)) return   // 已初始化，勿再抢焦点/重选
    initializedIds.add(id)
    nextTick(() => {
      el.focus()
      const dlg = store.stack.find(d => d.id === id)
      const mode = dlg?.selectMode ?? 'end'
      if (mode === 'all') {
        el.select()
      } else if (mode === 'name') {
        // 选中文件名部分（不含最后一个 . 后的扩展名）
        const v = el.value
        const dot = v.lastIndexOf('.')
        const end = dot > 0 ? dot : v.length
        el.setSelectionRange(0, end)
      } else {
        // 'end'：光标置于末尾，不选中任何文字
        const len = el.value.length
        el.setSelectionRange(len, len)
      }
    })
  }
  // 不在 el === null 分支清理：内联 ref 每次重渲染都会以 null 触发，
  // 在此删除 promptValues 会抹掉用户正在输入的内容。清理改由下方 watch 在对话框
  // 真正关闭（从 stack 移除）时进行。
}

// 对话框关闭后清理其临时状态（stack 变化时触发，输入过程中 stack 不变，不会误清）
watch(() => store.stack.length, () => {
  const alive = new Set(store.stack.map(d => d.id))
  for (const id of [...initializedIds]) {
    if (!alive.has(id)) {
      initializedIds.delete(id)
      promptInputs.delete(id)
      delete promptValues[id]
    }
  }
})

function onConfirm(dlg: DialogItem) {
  if (dlg.kind === 'confirm') {
    store.close(dlg.id, true)
  } else if (dlg.kind === 'prompt') {
    const val = promptValues[dlg.id] ?? ''
    store.close(dlg.id, val || null)
  } else {
    store.close(dlg.id, null)
  }
}

function onCancel(dlg: DialogItem) {
  if (dlg.kind === 'confirm') {
    store.close(dlg.id, false)
  } else if (dlg.kind === 'prompt') {
    store.close(dlg.id, null)
  } else {
    store.close(dlg.id, null)
  }
}

</script>

<style>
/* 不用 scoped，以便 Teleport 后样式也生效 */

.dlg-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(3px);
  display: flex;
  align-items: center;
  justify-content: center;
}

.dlg-box {
  background: var(--bg-secondary, #1c2433);
  border: 1px solid var(--border, #30363d);
  border-radius: 12px;
  padding: 20px 22px 18px;
  width: 360px;
  max-width: 90vw;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.7);
  display: flex;
  flex-direction: row;
  align-items: flex-start;
  gap: 16px;
  animation: dlg-pop 0.18s ease;
}

@keyframes dlg-pop {
  from { transform: scale(0.92); opacity: 0; }
  to   { transform: scale(1);    opacity: 1; }
}

/* 图标区域 */
.dlg-icon-wrap {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  flex-shrink: 0;
  margin-top: 2px;
}

.dlg-info    .dlg-icon-wrap { background: rgba(88,  166, 255, 0.12); }
.dlg-error   .dlg-icon-wrap { background: rgba(248, 81,  73,  0.12); }
.dlg-confirm .dlg-icon-wrap { background: rgba(209, 36,  47,  0.10); }
.dlg-prompt  .dlg-icon-wrap { background: rgba(63,  185, 80,  0.10); }

/* 右侧内容列 */
.dlg-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

/* 文字 */
.dlg-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary, #e6edf3);
  text-align: left;
  line-height: 1.3;
}

.dlg-message {
  font-size: 13px;
  color: var(--text-muted, #8b949e);
  text-align: left;
  line-height: 1.6;
  white-space: pre-line;   /* \n 渲染为换行 */
  word-break: break-all;
  max-height: 180px;
  overflow-y: auto;
}

/* error 标题变红 */
.dlg-error .dlg-title {
  color: var(--danger, #f85149);
}

/* prompt 输入框 */
.dlg-input {
  width: 100%;
  background: var(--bg-primary, #0d1117);
  border: 1px solid var(--border, #30363d);
  border-radius: 7px;
  color: var(--text-primary, #e6edf3);
  padding: 8px 12px;
  font-size: 13px;
  outline: none;
  box-sizing: border-box;
}
.dlg-input:focus { border-color: var(--accent, #58a6ff); }

/* 按钮组 */
.dlg-actions {
  display: flex;
  gap: 8px;
  margin-top: 4px;
  justify-content: flex-end;
}

.dlg-btn {
  padding: 7px 20px;
  border-radius: 7px;
  border: none;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: filter 0.15s, opacity 0.15s;
}

.dlg-btn-primary {
  background: var(--accent, #58a6ff);
  color: #fff;
}
.dlg-btn-primary:hover { filter: brightness(1.12); }

.dlg-btn-danger {
  background: var(--danger, #f85149);
  color: #fff;
}
.dlg-btn-danger:hover { filter: brightness(1.12); }

.dlg-btn-ghost {
  background: transparent;
  color: var(--text-muted, #8b949e);
  border: 1px solid var(--border, #30363d);
}
.dlg-btn-ghost:hover {
  color: var(--text-primary, #e6edf3);
  border-color: var(--text-muted, #8b949e);
}

/* 动画 */
.dlg-fade-enter-active,
.dlg-fade-leave-active { transition: opacity 0.15s; }
.dlg-fade-enter-from,
.dlg-fade-leave-to     { opacity: 0; }
</style>
